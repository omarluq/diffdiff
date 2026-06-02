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
	dr := newDiffRow(metrics, palette{})
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

// DiffRowSplitVisibleEmphasis builds a split row whose left cell carries an
// intra-line change segment, renders it through the Refresh+Layout path, and
// returns how many emphasis rectangles are visible. It guards the regression
// where split rows drew no intra-line emphasis at all — the per-character change
// tint the unified view shows over the specific changed letters.
func DiffRowSplitVisibleEmphasis() int {
	metrics := rowMetrics{advance: 8, height: 16, padding: 8, gutterW: 48, signW: 8, contentX: 112}
	dr := newDiffRow(metrics, palette{delEmph: color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0x80}})
	renderer, ok := dr.CreateRenderer().(*diffRowRenderer)
	if !ok {
		return -1
	}

	dr.data = row{
		kind: rowSplit,
		left: splitCell{
			present: true,
			line: diff.Line{
				Kind: diff.LineDeleted, OldNum: 1, NewNum: 0, Content: "foobar",
				Segments: []diff.Segment{
					{Text: "foo", Intraline: false},
					{Text: "bar", Intraline: true},
				},
			},
			tokens:  nil,
			hlIndex: 0,
		},
		right: splitCell{
			present: false,
			line:    diff.Line{Kind: diff.LineAdded, OldNum: 0, NewNum: 0, Content: "", Segments: nil},
			tokens:  nil,
			hlIndex: 0,
		},
	}
	dr.hasData = true

	renderer.width = 800
	renderer.Refresh()
	renderer.Layout(fyne.NewSize(800, metrics.height))

	visible := 0
	for _, emph := range renderer.emphasis {
		if emph.Visible() {
			visible++
		}
	}

	return visible
}

// hoverTestPalette returns a palette with distinct, opaque add/del/overlay tints
// and their derived hover colors, so hover-tint assertions can tell the kind
// colors apart from the neutral gray.
func hoverTestPalette() palette {
	pal := palette{
		addBg:   color.NRGBA{R: 0x12, G: 0x3a, B: 0x12, A: 0xff},
		addEmph: color.NRGBA{R: 0x24, G: 0x74, B: 0x24, A: 0xff},
		delBg:   color.NRGBA{R: 0x3a, G: 0x12, B: 0x12, A: 0xff},
		delEmph: color.NRGBA{R: 0x74, G: 0x24, B: 0x24, A: 0xff},
		overlay: color.NRGBA{R: 0x40, G: 0x40, B: 0x40, A: 0xff},
	}
	pal.addHover = hoverTint(pal.addBg, pal.addEmph)
	pal.delHover = hoverTint(pal.delBg, pal.delEmph)

	return pal
}

// DiffRowHoverIsKindColored builds a unified row of the given kind, hovers it edge
// to edge, and reports whether the hover tint is shown, whether it equals that
// kind's hover color, and whether it is the neutral gray overlay. It guards that a
// hovered added/deleted line tints in its own color instead of washing out gray.
func DiffRowHoverIsKindColored(kind diff.LineKind) (shown, matchesKind, isGray bool) {
	pal := hoverTestPalette()
	metrics := rowMetrics{advance: 8, height: 16, padding: 8, gutterW: 48, signW: 8, contentX: 112}
	dr := newDiffRow(metrics, pal)
	renderer, ok := dr.CreateRenderer().(*diffRowRenderer)
	if !ok {
		return false, false, false
	}

	dr.data = row{kind: rowLine, line: diff.Line{Kind: kind, OldNum: 1, NewNum: 1, Content: "x", Segments: nil}}
	dr.hasData = true
	dr.hovered = true
	dr.hoverCol = hoverWhole

	renderer.width = 400
	renderer.Refresh()
	renderer.Layout(fyne.NewSize(400, metrics.height))

	// FillColor is a color.Color holding the NRGBA set by applyHover; comparing the
	// interface to the concrete NRGBA is well-defined and avoids a type assertion.
	return renderer.hover.Visible(),
		renderer.hover.FillColor == hoverColor(pal, kind),
		renderer.hover.FillColor == pal.overlay
}

// DiffRowSplitHoverBounds builds a split row (deleted left, added right), maps the
// pointer X to a column exactly as MouseIn does, renders, and returns the hover
// rectangle's left edge and width plus whether its fill is the hovered column's
// kind color. It guards per-column hover: one side's highlight never spans both.
func DiffRowSplitHoverBounds(x, width float32) (left, hoverW float32, kindColored bool) {
	pal := hoverTestPalette()
	metrics := rowMetrics{advance: 8, height: 16, padding: 8, gutterW: 48, signW: 8, contentX: 112}
	dr := newDiffRow(metrics, pal)
	renderer, ok := dr.CreateRenderer().(*diffRowRenderer)
	if !ok {
		return -1, -1, false
	}

	dr.data = row{
		kind: rowSplit,
		left: splitCell{
			present: true,
			line:    diff.Line{Kind: diff.LineDeleted, OldNum: 1, NewNum: 0, Content: "a", Segments: nil},
			tokens:  nil, hlIndex: 0,
		},
		right: splitCell{
			present: true,
			line:    diff.Line{Kind: diff.LineAdded, OldNum: 0, NewNum: 1, Content: "b", Segments: nil},
			tokens:  nil, hlIndex: 0,
		},
	}
	dr.hasData = true
	dr.Resize(fyne.NewSize(width, metrics.height)) // so hoverColumn sees the width
	dr.hovered = true
	dr.hoverCol = dr.hoverColumn(x)

	renderer.width = width
	renderer.Refresh()
	renderer.Layout(fyne.NewSize(width, metrics.height))

	cell := &dr.data.left
	if dr.hoverCol == hoverRight {
		cell = &dr.data.right
	}

	return renderer.hover.Position().X,
		renderer.hover.Size().Width,
		renderer.hover.FillColor == hoverColor(pal, cellKind(cell))
}

// WindowRun and RuneSlice re-export the horizontal-window helpers for tests.
func WindowRun(text string, runes, col, hScroll, maxCols int) (visible string, screenCol int, ok bool) {
	return windowRun(text, runes, col, hScroll, maxCols)
}

// RuneSlice re-exports runeSlice for tests.
func RuneSlice(s string, from, to int) string { return runeSlice(s, from, to) }

// DiffRowUnifiedVisibleText renders a unified line at horizontal offset hScroll
// and returns the first visible text run, so tests can confirm the content window
// skips the scrolled-past leading runes (the gutter, rendered separately, is not
// included). A wide viewport keeps the clip from interfering.
func DiffRowUnifiedVisibleText(content string, hScroll int) string {
	metrics := rowMetrics{advance: 8, height: 16, padding: 0, gutterW: 0, signW: 0, contentX: 0}
	dr := newDiffRow(metrics, palette{foreground: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}})
	dr.hScroll = hScroll
	renderer, ok := dr.CreateRenderer().(*diffRowRenderer)
	if !ok {
		return "<err>"
	}

	dr.data = row{
		kind: rowLine,
		line: diff.Line{Kind: diff.LineContext, OldNum: 1, NewNum: 1, Content: content, Segments: nil},
	}
	dr.hasData = true
	renderer.width = 400
	renderer.Refresh()

	for _, txt := range renderer.texts {
		if txt.Visible() && txt.Text != "" {
			return txt.Text
		}
	}

	return ""
}

// DiffRowVisibleTextRuns applies first then second as the highlighted tokens of a
// single recycled diff row and returns how many pooled text runs are visible
// afterward. It verifies the renderer's pooling invariant: text runs left over
// from a denser line must be hidden when a sparser line reuses the widget, so the
// visible count equals len(second), not the high-water mark.
func DiffRowVisibleTextRuns(first, second []string) int {
	// Rendering now windows to the visible width, so the row needs a real advance
	// and a viewport wide enough that every single-rune run falls inside the
	// window; the pooling invariant (hiding surplus runs) is what we measure.
	metrics := rowMetrics{advance: 8, height: 16, padding: 0, gutterW: 0, signW: 0, contentX: 0}
	dr := newDiffRow(metrics, palette{})
	renderer, ok := dr.CreateRenderer().(*diffRowRenderer)
	if !ok {
		return -1
	}
	renderer.width = 400

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

// ScanBarStep / ScanBarFraction expose the indeterminate bar's internal sweep
// so tests can verify it bounces and stays in range without rendering.
func ScanBarStep(b *ScanBar)            { b.advance() }
func ScanBarFraction(b *ScanBar) float32 { return b.fraction() }

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
