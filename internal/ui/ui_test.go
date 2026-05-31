package ui_test

import (
	"os"
	"sync"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/highlight"
	"github.com/omarluq/diffdiff/internal/theme"
	"github.com/omarluq/diffdiff/internal/ui"
)

// fyneMu serializes Fyne widget construction and measurement across tests.
// Fyne's global font-metrics cache (internal/cache.GetFontMetrics) and font
// face cache mutate shared state on every MeasureText call without locking, so
// they are not safe to exercise from goroutines concurrently. The tests still
// declare t.Parallel() (satisfying the project's lint policy and letting
// non-Fyne setup overlap) but hold this mutex around the widget-touching body
// so the race detector stays clean under `go test -race`.
var fyneMu sync.Mutex

// TestMain initializes the Fyne test app exactly once, serially, before any
// parallel test runs. test.NewApp mutates the process-global storage registry,
// which races if first touched concurrently.
func TestMain(m *testing.M) {
	test.NewApp()
	os.Exit(m.Run())
}

// sampleFiles returns a small set of changed files with distinct paths for
// filter and selection tests.
func sampleFiles() []*diff.File {
	return []*diff.File{
		modifiedFile("internal/server/handler.go", 3, 1),
		modifiedFile("cmd/app/main.go", 10, 0),
		modifiedFile("internal/server/router.go", 1, 5),
		modifiedFile("README.md", 0, 2),
	}
}

// modifiedFile builds a modified diff.File with a single trivial hunk so it has
// renderable content.
func modifiedFile(path string, added, deleted int) *diff.File {
	return &diff.File{
		Path:     path,
		OldPath:  "",
		Status:   diff.StatusModified,
		Language: "",
		Binary:   false,
		Hunks: []diff.Hunk{{
			OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1, Section: "",
			Lines: []diff.Line{
				{Kind: diff.LineContext, OldNum: 1, NewNum: 1, Content: "context", Segments: nil},
			},
		}},
		Added:   added,
		Deleted: deleted,
	}
}

func TestFileListSetFilterNarrowsAndReorders(t *testing.T) {
	t.Parallel()

	fyneMu.Lock()
	defer fyneMu.Unlock()

	list := ui.NewFileList(nil)
	list.SetFiles(sampleFiles())

	require.Len(t, list.VisiblePaths(), 4, "unfiltered list shows every file")
	assert.Equal(t,
		[]string{
			"internal/server/handler.go",
			"cmd/app/main.go",
			"internal/server/router.go",
			"README.md",
		},
		list.VisiblePaths(),
		"unfiltered order matches input order",
	)

	list.SetFilter("server")
	visible := list.VisiblePaths()
	require.Len(t, visible, 2, "filter keeps only paths matching the query")
	for _, path := range visible {
		assert.Contains(t, path, "server")
	}

	list.SetFilter("")
	assert.Len(t, list.VisiblePaths(), 4, "clearing the filter restores all files")
}

func TestFileListFilterRanksBetterMatchesFirst(t *testing.T) {
	t.Parallel()

	fyneMu.Lock()
	defer fyneMu.Unlock()

	list := ui.NewFileList(nil)
	list.SetFiles([]*diff.File{
		modifiedFile("a/long/path/to/main.go", 1, 0),
		modifiedFile("main.go", 1, 0),
	})

	list.SetFilter("main")
	visible := list.VisiblePaths()
	require.Len(t, visible, 2)
	assert.Equal(t, "main.go", visible[0],
		"the tighter match should rank ahead of the deeply nested path")
}

func TestFileListSelectInvokesCallback(t *testing.T) {
	t.Parallel()

	fyneMu.Lock()
	defer fyneMu.Unlock()

	var selected *diff.File
	list := ui.NewFileList(func(file *diff.File) { selected = file })
	files := sampleFiles()
	list.SetFiles(files)

	list.SetFilter("README")
	require.Len(t, list.VisiblePaths(), 1)

	// Drive selection through the underlying list renderer path.
	test.WidgetRenderer(list)
	list.Select(0)
	require.NotNil(t, selected, "selecting a row should invoke onSelect")
	assert.Equal(t, "README.md", selected.Path)
}

func TestNewContentReturnsRootAndHandle(t *testing.T) {
	t.Parallel()

	fyneMu.Lock()
	defer fyneMu.Unlock()

	reg := theme.NewRegistry()
	hl := highlight.New(0)

	root, content := ui.NewContent(reg, theme.NewFontRegistry(), hl)
	require.NotNil(t, root, "root canvas object must be constructed")
	require.NotNil(t, content, "content handle must be constructed")
	assert.Equal(t, reg.Default().Name(), content.ActiveTheme().Name(),
		"initial theme is the registry default")
}

func TestContentSetThemeKnownAndUnknown(t *testing.T) {
	t.Parallel()

	fyneMu.Lock()
	defer fyneMu.Unlock()

	reg := theme.NewRegistry()
	hl := highlight.New(0)

	_, content := ui.NewContent(reg, theme.NewFontRegistry(), hl)
	if _, ok := reg.Get("Dracula"); !ok {
		t.Skip("Dracula theme not present in registry")
	}

	content.SetTheme("Dracula")
	assert.Equal(t, "Dracula", content.ActiveTheme().Name(),
		"a known theme name updates the active theme")

	content.SetTheme("nope-not-a-theme")
	assert.Equal(t, "Dracula", content.ActiveTheme().Name(),
		"an unknown theme name is a no-op")
}

func TestContentSetFilesFeedsFileList(t *testing.T) {
	t.Parallel()

	fyneMu.Lock()
	defer fyneMu.Unlock()

	reg := theme.NewRegistry()
	hl := highlight.New(0)

	_, content := ui.NewContent(reg, theme.NewFontRegistry(), hl)
	require.NotNil(t, content)
	// SetFiles must not panic and must accept a nil-free slice.
	content.SetFiles(sampleFiles())
}

func TestSplitFlattenPairsAndBlanks(t *testing.T) {
	t.Parallel()

	fyneMu.Lock()
	defer fyneMu.Unlock()

	file := &diff.File{
		Path:     "pkg/file.go",
		OldPath:  "",
		Status:   diff.StatusModified,
		Language: "",
		Binary:   false,
		Hunks: []diff.Hunk{
			{
				OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 2, Section: "func A()",
				Lines: []diff.Line{
					{Kind: diff.LineContext, OldNum: 1, NewNum: 1, Content: "a", Segments: nil},
					{Kind: diff.LineDeleted, OldNum: 2, NewNum: 0, Content: "old", Segments: nil},
					{Kind: diff.LineAdded, OldNum: 0, NewNum: 2, Content: "new", Segments: nil},
				},
			},
			{
				OldStart: 10, OldLines: 1, NewStart: 10, NewLines: 2, Section: "",
				Lines: []diff.Line{
					{Kind: diff.LineContext, OldNum: 10, NewNum: 10, Content: "z", Segments: nil},
					{Kind: diff.LineAdded, OldNum: 0, NewNum: 11, Content: "w", Segments: nil},
				},
			},
		},
		Added:   2,
		Deleted: 1,
	}

	shapes := ui.SplitRowShapes(file)

	// Each hunk opens with a separator. A modified line pairs old-left with
	// new-right ("LR"); a pure addition leaves the old column blank ("-R").
	assert.Equal(t,
		[]string{"sep", "LR", "LR", "sep", "LR", "-R"},
		shapes,
		"context pairs both columns, a modify pairs del/add, an add blanks the left",
	)
}

func TestContentSplitToggleSwitchesLayout(t *testing.T) {
	t.Parallel()

	fyneMu.Lock()
	defer fyneMu.Unlock()

	reg := theme.NewRegistry()
	hl := highlight.New(0)

	_, content := ui.NewContent(reg, theme.NewFontRegistry(), hl)
	content.SetFiles(sampleFiles())

	assert.True(t, content.SplitToggle(), "first toggle enables the split layout")
	assert.False(t, content.SplitToggle(), "second toggle restores the unified layout")
}

func TestFlattenProducesSeparatorPerHunkPlusLines(t *testing.T) {
	t.Parallel()

	fyneMu.Lock()
	defer fyneMu.Unlock()

	file := &diff.File{
		Path:     "pkg/file.go",
		OldPath:  "",
		Status:   diff.StatusModified,
		Language: "",
		Binary:   false,
		Hunks: []diff.Hunk{
			{
				OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 2, Section: "func A()",
				Lines: []diff.Line{
					{Kind: diff.LineContext, OldNum: 1, NewNum: 1, Content: "a", Segments: nil},
					{Kind: diff.LineDeleted, OldNum: 2, NewNum: 0, Content: "old", Segments: nil},
					{Kind: diff.LineAdded, OldNum: 0, NewNum: 2, Content: "new", Segments: nil},
				},
			},
			{
				OldStart: 10, OldLines: 1, NewStart: 10, NewLines: 1, Section: "",
				Lines: []diff.Line{
					{Kind: diff.LineContext, OldNum: 10, NewNum: 10, Content: "z", Segments: nil},
				},
			},
		},
		Added:   1,
		Deleted: 1,
	}

	kinds := ui.FlattenRowKinds(file)

	// 2 hunks => 2 separators, plus 3 + 1 line rows = 6 rows total.
	require.Len(t, kinds, file.TotalLines()+len(file.Hunks))
	require.Len(t, kinds, 6)
	assert.Equal(t,
		[]bool{true, false, false, false, true, false},
		kinds,
		"each hunk begins with a separator row followed by its line rows",
	)
}
