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
	highlighter := di.MustInvoke[*highlight.Highlighter](container)

	files, err := repo.WorkingDiff()
	if err != nil {
		return oops.In("gui").Code("working_diff").Wrapf(err, "compute working-tree diff")
	}

	application := app.NewWithID(appID)
	root, content := ui.NewContent(registry, highlighter)

	// Keep the window chrome in step with the diff surface when the theme picker
	// changes, and apply the initial theme.
	content.OnThemeChange(func(active *theme.Theme) {
		application.Settings().SetTheme(theme.NewFyneTheme(active))
	})
	application.Settings().SetTheme(theme.NewFyneTheme(content.ActiveTheme()))
	content.SetFiles(files)

	window := application.NewWindow(windowTitle(repo.Root()))
	window.SetContent(root)
	window.Resize(fyne.NewSize(windowWidth, windowHeight))

	stopOnCancel(ctx, application)
	window.ShowAndRun()

	return nil
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
