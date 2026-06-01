package ui

import (
	"image/color"
	"sort"

	"fyne.io/fyne/v2"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/highlight"
)

// testRunTokens builds white highlight tokens from plain run strings.
func testRunTokens(runs []string) []highlight.Token {
	white := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	tokens := make([]highlight.Token, 0, len(runs))
	for _, text := range runs {
		tokens = append(tokens, highlight.Token{Text: text, Color: white, Bold: false, Italic: false})
	}

	return tokens
}

// DiffRowSplitVisibleAfterLayout builds a split row, runs Refresh then Layout —
// the recycle-then-resize sequence — and returns the visible pooled text count.
// It guards the regression where Layout rebuilt the split columns without
// resetting the pool, stacking a duplicate set of runs over the previous frame's
// (the overlap seen on long split lines). The expected count is len(left)+len(right).
func DiffRowSplitVisibleAfterLayout(leftRuns, rightRuns []string, width float32) int {
	metrics := rowMetrics{advance: 8, height: 16, padding: 8, gutterW: 48, signW: 8, contentX: 112}
	dr := newDiffRow(metrics, palette{}, diffTextSize)
	renderer, ok := dr.CreateRenderer().(*diffRowRenderer)
	if !ok {
		return -1
	}

	dr.data = row{
		kind: rowSplit,
		left: splitCell{
			present: true,
			line:    diff.Line{Kind: diff.LineDeleted, OldNum: 1, NewNum: 0, Content: "x", Segments: nil},
			tokens:  testRunTokens(leftRuns),
			hlIndex: 0,
		},
		right: splitCell{
			present: true,
			line:    diff.Line{Kind: diff.LineAdded, OldNum: 0, NewNum: 1, Content: "y", Segments: nil},
			tokens:  testRunTokens(rightRuns),
			hlIndex: 0,
		},
	}
	dr.hasData = true

	renderer.width = width
	renderer.Refresh()
	renderer.Layout(fyne.NewSize(width, metrics.height))

	visible := 0
	for _, txt := range renderer.texts {
		if txt.Visible() {
			visible++
		}
	}

	return visible
}

// DiffRowVisibleTextRuns applies first then second as the highlighted tokens of a
// single recycled diff row and returns how many pooled text runs are visible
// afterward. It verifies the renderer's pooling invariant: text runs left over
// from a denser line must be hidden when a sparser line reuses the widget, so the
// visible count equals len(second), not the high-water mark.
func DiffRowVisibleTextRuns(first, second []string) int {
	// Metrics are irrelevant to the pooling invariant (only positions depend on
	// them), so a zero value avoids a computeMetrics call here.
	dr := newDiffRow(rowMetrics{}, palette{}, diffTextSize)
	renderer, ok := dr.CreateRenderer().(*diffRowRenderer)
	if !ok {
		return -1
	}

	apply := func(runs []string) {
		dr.data = row{
			kind:   rowLine,
			line:   diff.Line{Kind: diff.LineContext, OldNum: 1, NewNum: 1, Content: "x", Segments: nil},
			tokens: testRunTokens(runs),
		}
		dr.hasData = true
		renderer.Refresh()
	}

	apply(first)
	apply(second)

	visible := 0
	for _, txt := range renderer.texts {
		if txt.Visible() {
			visible++
		}
	}

	return visible
}

// PrefixLines re-exports prefixLines for tests.
func PrefixLines(text string, n int) string {
	return prefixLines(text, n)
}

// PrefixExtentLines builds unified line rows from (hlOld, hlIndex) pairs and
// returns prefixExtent over the first maxRows, so tests can verify the prefix
// reaches far enough to cover deep hunks.
func PrefixExtentLines(hlOld []bool, hlIndex []int, maxRows int) (oldN, newN int) {
	rows := make([]row, len(hlIndex))
	for i := range hlIndex {
		rows[i] = row{kind: rowLine, hlOld: hlOld[i], hlIndex: hlIndex[i]}
	}

	return prefixExtent(rows, maxRows)
}

// DiffShowsLoading reports whether the diff view is currently showing its
// "loading" placeholder, letting tests verify the lazy-load placeholder/swap.
func (c *Content) DiffShowsLoading() bool {
	return c.diffView.loading.Visible()
}

// ScanningShown reports whether the file panel's "Scanning…" indicator is
// visible, so tests can verify it shows during a scan and clears on SetFiles.
func (c *Content) ScanningShown() bool {
	return c.fileList.scanning.Visible()
}

// DiffRowCount reports how many rows the diff view has flattened, so tests can
// assert that a file renders (rows > 0) or is preserved across a restyle.
func (c *Content) DiffRowCount() int {
	return len(c.diffView.rows)
}

// TreeLeafPaths returns the sorted leaf UIDs of the nested-view model for files.
// Leaves are keyed by full path, so tests assert these equal the input paths —
// the invariant RefreshFile relies on to refresh a tree leaf by file.Path.
func TreeLeafPaths(files []*diff.File) []string {
	model := buildTreeModel(allEntries(files))
	paths := make([]string, 0, len(model.leaves))
	for uid := range model.leaves {
		paths = append(paths, uid)
	}
	sort.Strings(paths)

	return paths
}

// FlattenRowKinds re-exports the unexported flatten helper for the external
// ui_test package, returning a compact description of each produced row: true
// for a hunk separator, false for a diff line. Tests assert on this to verify a
// file flattens into the expected separator+line sequence.
func FlattenRowKinds(file *diff.File) []bool {
	rows := flatten(file, computeMetrics(diffTextSize))
	kinds := make([]bool, len(rows))
	for i := range rows {
		kinds[i] = rows[i].kind == rowSeparator
	}

	return kinds
}

// SplitRowShapes re-exports the unexported flattenSplit helper, describing each
// produced row: "sep" for a hunk separator, otherwise a two-character column
// presence marker — "LR" both sides, "L-" old only, "-R" new only.
func SplitRowShapes(file *diff.File) []string {
	rows := flattenSplit(file, computeMetrics(diffTextSize))
	shapes := make([]string, len(rows))
	for i := range rows {
		if rows[i].kind == rowSeparator {
			shapes[i] = "sep"

			continue
		}
		left, right := "-", "-"
		if rows[i].left.present {
			left = "L"
		}
		if rows[i].right.present {
			right = "R"
		}
		shapes[i] = left + right
	}

	return shapes
}

// SplitToggle drives the hamburger "Split view" action and reports the diff
// view's resulting layout, letting tests verify the toggle wiring end to end.
func (c *Content) SplitToggle() bool {
	c.toggleSplitView()

	return c.diffView.Split()
}

// TreeChildPaths re-exports the nested-view directory model: it builds the tree
// for files and returns the child UIDs of parent ("" for the root), so tests can
// assert the directory grouping and ordering.
func TreeChildPaths(files []*diff.File, parent string) []string {
	model := buildTreeModel(allEntries(files))

	return model.children[parent]
}

// TreeToggle drives the hamburger "Nested tree" action and reports whether the
// file panel switched to the nested view, verifying the toggle wiring.
func (c *Content) TreeToggle() bool {
	c.toggleTreeView()

	return c.fileList.nested
}
