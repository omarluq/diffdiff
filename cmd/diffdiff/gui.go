package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	fynecontainer "fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynetheme "fyne.io/fyne/v2/theme"
	xwidget "fyne.io/x/fyne/widget"
	"github.com/samber/oops"
	"golang.org/x/sync/errgroup"

	"github.com/omarluq/diffdiff/assets"
	"github.com/omarluq/diffdiff/internal/di"
	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/git"
	"github.com/omarluq/diffdiff/internal/highlight"
	"github.com/omarluq/diffdiff/internal/recents"
	"github.com/omarluq/diffdiff/internal/theme"
	"github.com/omarluq/diffdiff/internal/ui"
)

const (
	appID        = "com.omarluq.diffdiff"
	windowWidth  = 1200
	windowHeight = 800
)

// session holds the mutable runtime state shared by the UI callbacks: the open
// repository and its lazily-built working set. Diff scans and the background
// build sweep run off the UI goroutine, so repo, ws, and cancel are guarded by a
// mutex. cancel stops the current sweep when a new repository is opened.
type session struct {
	window  fyne.Window
	content *ui.Content
	recents *recents.Store

	mu     sync.Mutex
	repo   *git.Repository
	ws     *git.WorkingSet
	cancel context.CancelFunc
}

// runGUI opens the diff viewer window and blocks until it is closed or ctx is
// canceled. It opens the current directory when it is a git repository (you ran
// diffdiff from inside a checkout), otherwise the most recent project that still
// opens (e.g. launched from the desktop menu, where the working directory is
// home), otherwise it shows an empty window with the project picker so you can
// choose one — so a menu launch never fails for lack of a repo.
func runGUI(ctx context.Context) error {
	const repoPath = "."
	container, err := di.NewContainer(cfgFile, repoPath)
	if err != nil {
		return err
	}
	defer func() {
		if report := container.ShutdownWithContext(ctx); !report.Succeed {
			_, _ = fmt.Fprintln(os.Stderr, report.Error())
		}
	}()

	store := di.MustInvoke[*recents.Store](container)
	repo := startupRepo(container, store)

	application := app.NewWithID(appID)
	application.SetIcon(fyne.NewStaticResource("diffdiff.png", assets.Mascot))
	root, content := ui.NewContent(
		di.MustInvoke[*theme.Registry](container),
		di.MustInvoke[*theme.FontRegistry](container),
		di.MustInvoke[*highlight.Highlighter](container),
	)

	sess := &session{
		window:  nil,
		content: content,
		recents: store,
		mu:      sync.Mutex{},
		repo:    repo,
		ws:      nil,
		cancel:  nil,
	}
	content.OnOpenProject(func(path string) { go sess.doOpen(ctx, path) })

	title := "diffdiff"
	if repo != nil {
		title = windowTitle(repo.Root())
	}
	window := application.NewWindow(title)
	sess.window = window
	window.SetContent(root)
	window.Resize(fyne.NewSize(windowWidth, windowHeight))

	// Seed the project picker with recents so it is usable immediately, before the
	// first scan finishes and when no project is open at all.
	content.SetRecentProjects(store.List())

	if repo != nil {
		// Show the window immediately; the working-tree scan (go-git status) can take
		// a moment on a huge worktree, so load runs it off the UI goroutine behind the
		// scan card and fills the file list in when it completes.
		go sess.load(ctx, repo)
	} else {
		// No project resolved: present a defined empty state until the user opens one.
		content.SetFiles(nil)
	}

	stopOnCancel(ctx, application)
	window.ShowAndRun()

	return nil
}

// startupRepo resolves the repository to open on launch: the current directory
// when it is a repository (an explicit run from inside a checkout), else the most
// recent project that still opens, else nil — in which case the window opens
// empty and the user picks a project. git.Open failures are expected here (a
// non-repo working directory, a recent that was moved or deleted), so they are
// skipped rather than surfaced.
func startupRepo(container *di.Container, store *recents.Store) *git.Repository {
	if repo, err := di.Invoke[*git.Repository](container); err == nil {
		return repo
	}
	for _, path := range store.List() {
		if repo, err := git.Open(path); err == nil {
			return repo
		}
	}

	return nil
}

// load scans the repository's working tree off the UI goroutine, then publishes
// the file list, window title, and git info (the caller shows a scanning
// indicator first; SetFiles clears it) and starts the background diff sweep. The
// scan dominates startup on huge worktrees, so keeping it off the UI goroutine
// lets the window appear and stay responsive.
func (s *session) load(ctx context.Context, repo *git.Repository) {
	fyne.Do(s.content.Clear) // empty the panels on the spot (matters on repo switch)
	indicator := startScanIndicator(s.window)

	working, files, err := repo.ChangedFiles()
	indicator.done()
	if err != nil {
		s.reportError(oops.In("gui").Code("changed_files").Wrapf(err, "scan working tree"))

		return
	}

	s.mu.Lock()
	s.repo = repo
	s.ws = working
	s.mu.Unlock()

	s.remember(repo.Root())
	details := repo.Details()
	recent := s.recents.List()
	fyne.Do(func() {
		s.window.SetTitle(windowTitle(repo.Root()))
		s.content.SetRecentProjects(recent)
		s.content.SetGitInfo(details.Branch, details.Head)
		s.content.SetFiles(files)
	})

	s.startSweep(ctx, working, files)
}

// scanShowDelay defers the scanning dialog so a fast scan never flashes it.
const scanShowDelay = 200 * time.Millisecond

// scanIndicator shows a modal nyan-cat popup while a working-tree scan runs: a
// dimmed full-window backdrop with a centered card. It appears only if the scan
// outlasts scanShowDelay, so quick scans show nothing. The card is hand-built
// rather than a Fyne dialog because a dialog paints its card with
// ColorNameOverlayBackground (our Surface shade) — the transparent GIF revealed
// that as a box clashing with the window (the "color tearing"). This card fills
// with the window's own Background, so the cat sits on one seamless tone, while
// the dimmed backdrop gives it the pop of a real dialog.
type scanIndicator struct {
	win  fyne.Window
	stop chan struct{}
}

// scanAnim is the moving content of the scanning dialog. The nyan-cat GIF is the
// primary indicator; ScanBar is a dependency-free fallback used only if the GIF
// fails to decode. Both animate from their own goroutine (not fyne.Animation), so
// the no_animations build tag never freezes them.
type scanAnim interface {
	fyne.CanvasObject
	Start()
	Stop()
}

// nyanWidth/nyanHeight are the embedded GIF's native pixel dimensions; pinning the
// indicator to them renders the pixel art crisp at 1:1. scanCardRadius rounds the
// card corners.
const (
	nyanWidth      = 200
	nyanHeight     = 161
	scanCardRadius = 12
)

// newScanAnim builds the nyan-cat indicator, falling back to ScanBar if the
// embedded GIF cannot be decoded (it is validated and committed, so the fallback
// is purely defensive). Call on the UI goroutine.
func newScanAnim() scanAnim {
	gif, err := xwidget.NewAnimatedGifFromResource(fyne.NewStaticResource("nyan-cat.gif", assets.Nyan))
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())

		return ui.NewScanBar()
	}
	gif.SetMinSize(fyne.NewSize(nyanWidth, nyanHeight))

	return gif
}

// startScanIndicator launches the (delayed) scanning dialog and returns a handle;
// call done when the scan finishes to cancel the pending dialog or dismiss it.
func startScanIndicator(win fyne.Window) *scanIndicator {
	indicator := &scanIndicator{win: win, stop: make(chan struct{})}
	go indicator.run()

	return indicator
}

// done stops the indicator from any goroutine; run handles the UI work.
func (si *scanIndicator) done() {
	close(si.stop)
}

func (si *scanIndicator) run() {
	select {
	case <-si.stop:
		return // scan finished before the delay: never show the dialog
	case <-time.After(scanShowDelay):
	}

	var (
		anim    scanAnim
		overlay fyne.CanvasObject
		ready   = make(chan struct{})
	)
	fyne.Do(func() {
		anim = newScanAnim()
		overlay = scanCard(anim)
		si.win.Canvas().Overlays().Add(overlay)
		// Overlays().Add does not size the object, so it would otherwise sit at its
		// min size in a corner. Fill the canvas so NewCenter centers the card; the
		// canvas resize loop keeps it centered on later window resizes.
		overlay.Resize(si.win.Canvas().Size())
		anim.Start()
		close(ready)
	})
	<-ready

	<-si.stop
	fyne.Do(func() {
		anim.Stop()
		si.win.Canvas().Overlays().Remove(overlay)
	})
}

// scanCard builds the modal scan popup: a dimmed full-canvas backdrop (the theme
// shadow color, as Fyne's own popups use to dim the window) behind a centered
// card. The card is a rounded rectangle filled with the window's own Background
// color — so the transparent GIF sits on one seamless tone with no Surface box —
// bordered with the theme separator and padded around the animation. The result
// fills the canvas (resize it to lay the backdrop edge to edge and center the
// card); the backdrop dims everything else so the card pops like a dialog.
func scanCard(anim scanAnim) fyne.CanvasObject {
	settings := fyne.CurrentApp().Settings()
	thm, variant := settings.Theme(), settings.ThemeVariant()

	backdrop := canvas.NewRectangle(thm.Color(fynetheme.ColorNameShadow, variant))

	card := canvas.NewRectangle(thm.Color(fynetheme.ColorNameBackground, variant))
	card.CornerRadius = scanCardRadius
	card.StrokeColor = thm.Color(fynetheme.ColorNameSeparator, variant)
	card.StrokeWidth = 1

	body := fynecontainer.NewStack(card, fynecontainer.NewPadded(anim))

	return fynecontainer.NewStack(backdrop, fynecontainer.NewCenter(body))
}

// startSweep cancels any prior background build and launches a new one off the
// UI goroutine. It is bound to parent, so it stops when parent is canceled (app
// shutdown) or when the next sweep starts (a new repository is opened).
func (s *session) startSweep(parent context.Context, working *git.WorkingSet, files []*diff.File) {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.mu.Unlock()

	go s.buildAll(ctx, working, files)
}

// buildAll materializes every file's diff, the selected file (index 0) first so
// it renders promptly, then the remainder with bounded concurrency, publishing
// each result to the UI as it lands.
func (s *session) buildAll(ctx context.Context, working *git.WorkingSet, files []*diff.File) {
	if len(files) == 0 {
		return
	}

	s.buildOne(ctx, working, files[0])

	var group errgroup.Group
	group.SetLimit(runtime.NumCPU())
	for _, file := range files[1:] {
		group.Go(func() error {
			s.buildOne(ctx, working, file)

			return nil
		})
	}
	if err := group.Wait(); err != nil {
		s.reportError(err)
	}
}

// buildOne loads one file's diff and, unless the sweep was canceled, publishes it
// to the UI. A load error is reported but does not abort the rest of the sweep;
// the file's row simply keeps its placeholder.
func (s *session) buildOne(ctx context.Context, working *git.WorkingSet, file *diff.File) {
	if ctx.Err() != nil {
		return
	}
	if err := working.LoadFile(file); err != nil {
		s.reportError(err)

		return
	}
	if ctx.Err() == nil {
		fyne.Do(func() { s.content.FileReady(file) })
	}
}

// doOpen switches the active repository to the one at path, then defers to load,
// which clears the panels, shows the scan progress dialog, and repaints. It runs
// off the UI goroutine. ctx is the application context (captured from runGUI), so
// the build sweep this triggers is canceled on app shutdown like the initial one.
func (s *session) doOpen(ctx context.Context, path string) {
	repo, err := git.Open(path)
	if err != nil {
		s.reportError(oops.In("gui").Code("open_repo").With("path", path).Wrapf(err, "open repository"))

		return
	}

	s.load(ctx, repo)
}

// remember records a project path in the recent list, logging persistence
// failures without interrupting the user.
func (s *session) remember(root string) {
	if err := s.recents.Add(root); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
	}
}

// reportError shows err in a dialog on the Fyne main loop.
func (s *session) reportError(err error) {
	fyne.Do(func() { dialog.ShowError(err, s.window) })
}

// stopOnCancel quits the application when ctx is canceled (e.g. SIGINT), so the
// CLI's signal handling also tears down the window.
func stopOnCancel(ctx context.Context, application fyne.App) {
	go func() {
		<-ctx.Done()
		// Quit touches UI state, so it must run on Fyne's main goroutine.
		fyne.Do(application.Quit)
	}()
}

// windowTitle builds the window title from the repository root path.
func windowTitle(root string) string {
	return "diffdiff — " + root
}
