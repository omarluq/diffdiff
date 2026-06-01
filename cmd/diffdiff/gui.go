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
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/samber/oops"
	"golang.org/x/sync/errgroup"

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

// runGUI opens the diff viewer window for the current directory's repository and
// blocks until it is closed or ctx is canceled.
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

	repo, err := di.Invoke[*git.Repository](container)
	if err != nil {
		return oops.In("gui").Code("open_repo").With("path", repoPath).Wrapf(err, "open repository")
	}

	application := app.NewWithID(appID)
	root, content := ui.NewContent(
		di.MustInvoke[*theme.Registry](container),
		di.MustInvoke[*theme.FontRegistry](container),
		di.MustInvoke[*highlight.Highlighter](container),
	)

	sess := &session{
		window:  nil,
		content: content,
		recents: di.MustInvoke[*recents.Store](container),
		mu:      sync.Mutex{},
		repo:    repo,
		ws:      nil,
		cancel:  nil,
	}
	content.OnOpenProject(func(path string) { go sess.doOpen(path) })

	window := application.NewWindow(windowTitle(repo.Root()))
	sess.window = window
	window.SetContent(root)
	window.Resize(fyne.NewSize(windowWidth, windowHeight))

	// Show the window immediately; the working-tree scan (go-git status) can take
	// a moment on a huge worktree, so load runs it off the UI goroutine behind a
	// progress dialog and fills the file list in when it completes.
	go sess.load(ctx, repo)

	stopOnCancel(ctx, application)
	window.ShowAndRun()

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

const (
	// scanShowDelay defers the scanning dialog so a fast scan never flashes it.
	scanShowDelay = 200 * time.Millisecond
	// scanSpinnerInterval is how often the spinner advances. Animations are
	// disabled in this build (no_animations), so the spinner is stepped manually
	// via Refresh rather than a fyne.Animation.
	scanSpinnerInterval = 120 * time.Millisecond
)

// scanSpinnerFrames are the manual spinner glyphs cycled while scanning.
var scanSpinnerFrames = []string{"|", "/", "-", "\\"}

// scanIndicator shows a modal "Scanning repository" dialog with a manually
// stepped spinner while a working-tree scan runs. The dialog appears only if the
// scan outlasts scanShowDelay, so quick scans show nothing.
type scanIndicator struct {
	win  fyne.Window
	stop chan struct{}
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
		label *widget.Label
		shown *dialog.CustomDialog
		ready = make(chan struct{})
	)
	fyne.Do(func() {
		label = widget.NewLabelWithStyle(scanLabel(0), fyne.TextAlignCenter, fyne.TextStyle{Monospace: true})
		shown = dialog.NewCustomWithoutButtons("", label, si.win)
		shown.Show()
		close(ready)
	})
	<-ready

	ticker := time.NewTicker(scanSpinnerInterval)
	defer ticker.Stop()
	frame := 0
	for {
		select {
		case <-si.stop:
			fyne.Do(shown.Hide)

			return
		case <-ticker.C:
			frame++
			text := scanLabel(frame)
			fyne.Do(func() { label.SetText(text) })
		}
	}
}

// scanLabel renders the spinner frame as the dialog's centered message.
func scanLabel(frame int) string {
	return "Scanning repository  " + scanSpinnerFrames[frame%len(scanSpinnerFrames)]
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
// off the UI goroutine.
func (s *session) doOpen(path string) {
	repo, err := git.Open(path)
	if err != nil {
		s.reportError(oops.In("gui").Code("open_repo").With("path", path).Wrapf(err, "open repository"))

		return
	}

	s.load(context.Background(), repo)
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
