package main

import (
	"context"
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/samber/oops"

	"github.com/omarluq/diffdiff/internal/di"
	"github.com/omarluq/diffdiff/internal/git"
	"github.com/omarluq/diffdiff/internal/highlight"
	"github.com/omarluq/diffdiff/internal/theme"
	"github.com/omarluq/diffdiff/internal/ui"
)

const (
	appID         = "com.omarluq.diffdiff"
	windowWidth   = 1200
	windowHeight  = 800
	splitFraction = 0.28
)

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

	registry := di.MustInvoke[*theme.Registry](container)
	fonts := di.MustInvoke[*theme.FontRegistry](container)
	highlighter := di.MustInvoke[*highlight.Highlighter](container)

	files, err := repo.WorkingDiff(false)
	if err != nil {
		return oops.In("gui").Code("working_diff").Wrapf(err, "compute working-tree diff")
	}

	application := app.NewWithID(appID)
	// Content applies the active theme and font to the app itself (via
	// fyne.CurrentApp), so the window only needs the assembled root.
	root, content := ui.NewContent(registry, fonts, highlighter)

	// Re-scan off the UI goroutine when "show ignored" toggles; the worktree walk
	// can be slow, so only the SetFiles repaint runs on the Fyne main loop.
	content.OnShowIgnored(func(showIgnored bool) {
		go reloadDiff(content, repo, showIgnored)
	})
	content.SetFiles(files)

	window := application.NewWindow(windowTitle(repo.Root()))
	window.SetContent(root)
	window.Resize(fyne.NewSize(windowWidth, windowHeight))

	stopOnCancel(ctx, application)
	window.ShowAndRun()

	return nil
}

// reloadDiff recomputes the working-tree diff (optionally including ignored
// files) and applies it on the Fyne main loop.
func reloadDiff(content *ui.Content, repo *git.Repository, showIgnored bool) {
	files, err := repo.WorkingDiff(showIgnored)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())

		return
	}

	fyne.Do(func() { content.SetFiles(files) })
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
