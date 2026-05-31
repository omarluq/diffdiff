package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"github.com/samber/oops"

	"github.com/omarluq/diffdiff/internal/di"
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
// repository and whether ignored files are shown. Diff scans run off the UI
// goroutine, so repo and showIgnored are guarded by a mutex.
type session struct {
	window  fyne.Window
	content *ui.Content
	recents *recents.Store

	mu          sync.Mutex
	repo        *git.Repository
	showIgnored bool
}

// runGUI opens the diff viewer window for the repository at repoPath and blocks
// until it is closed or ctx is canceled.
func runGUI(ctx context.Context, repoPath string) error {
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
		window:      nil,
		content:     content,
		recents:     di.MustInvoke[*recents.Store](container),
		mu:          sync.Mutex{},
		repo:        repo,
		showIgnored: false,
	}
	content.OnShowIgnored(sess.setShowIgnored)
	content.OnOpenProject(sess.openProject)

	files, err := repo.WorkingDiff(false)
	if err != nil {
		return oops.In("gui").Code("working_diff").Wrapf(err, "compute working-tree diff")
	}
	sess.remember(repo.Root())
	content.SetRecentProjects(sess.recents.List())
	content.SetFiles(files)

	window := application.NewWindow(windowTitle(repo.Root()))
	sess.window = window
	window.SetContent(root)
	window.Resize(fyne.NewSize(windowWidth, windowHeight))

	stopOnCancel(ctx, application)
	window.ShowAndRun()

	return nil
}

// setShowIgnored updates the show-ignored flag and rescans the active repository.
func (s *session) setShowIgnored(show bool) {
	s.mu.Lock()
	s.showIgnored = show
	s.mu.Unlock()

	go s.reload()
}

// openProject switches the active repository to the one at path, off the UI
// goroutine.
func (s *session) openProject(path string) {
	go s.doOpen(path)
}

// doOpen opens the repository at path, makes it active, and repaints. The open
// and diff scan run off the UI goroutine; only the repaint, title, and recents
// updates run on the Fyne main loop.
func (s *session) doOpen(path string) {
	repo, err := git.Open(path)
	if err != nil {
		s.reportError(oops.In("gui").Code("open_repo").With("path", path).Wrapf(err, "open repository"))

		return
	}

	s.mu.Lock()
	s.repo = repo
	show := s.showIgnored
	s.mu.Unlock()

	s.remember(repo.Root())

	files, err := repo.WorkingDiff(show)
	if err != nil {
		s.reportError(err)

		return
	}

	recent := s.recents.List()
	fyne.Do(func() {
		s.window.SetTitle(windowTitle(repo.Root()))
		s.content.SetRecentProjects(recent)
		s.content.SetFiles(files)
	})
}

// reload rescans the active repository and repaints on the Fyne main loop.
func (s *session) reload() {
	s.mu.Lock()
	repo := s.repo
	show := s.showIgnored
	s.mu.Unlock()

	files, err := repo.WorkingDiff(show)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())

		return
	}

	fyne.Do(func() { s.content.SetFiles(files) })
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
