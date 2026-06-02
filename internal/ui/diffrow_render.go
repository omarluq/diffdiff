package ui

import (
	"image/color"
	"strconv"
	"unicode/utf8"

	"fyne.io/fyne/v2"

	"github.com/omarluq/diffdiff/internal/diff"
)

// refreshSeparator renders a hunk header (@@ ... @@) on the surface color and
// hides the line-specific cells.
func (r *diffRowRenderer) refreshSeparator() {
	r.background.Show()
	r.background.FillColor = r.row.palette.surface
	r.hideSplit()
	r.oldNum.Hide()
	r.newNum.Hide()
	r.sign.Hide()

	r.header.Show()
	r.header.Text = r.row.data.header
	r.header.Color = r.row.palette.muted
	r.header.Move(fyne.NewPos(r.row.metrics.padding, 0))
}

// refreshLine renders a single diff line: row background, intra-line emphasis
// rectangles, gutters, sign, and the highlighted (or plain) text runs.
func (r *diffRowRenderer) refreshLine() {
	line := r.row.data.line
	r.header.Hide()
	r.background.Show()
	r.hideSplit()
	r.applyLineBackground(line.Kind)
	r.layoutGutters(line)
	r.buildEmphasis(line)
	r.buildTexts(line)
}

// hideSplit hides the split-only column rectangles so a recycled row that was
// last used for a split line draws cleanly as a unified line or separator.
func (r *diffRowRenderer) hideSplit() {
	r.leftBg.Hide()
	r.rightBg.Hide()
	r.divider.Hide()
}

// applyLineBackground tints the full-width background per line kind.
func (r *diffRowRenderer) applyLineBackground(kind diff.LineKind) {
	switch kind {
	case diff.LineAdded:
		r.background.FillColor = r.row.palette.addBg
	case diff.LineDeleted:
		r.background.FillColor = r.row.palette.delBg
	case diff.LineContext:
		r.background.FillColor = color.Transparent
	default:
		r.background.FillColor = color.Transparent
	}
}

// layoutGutters positions the old/new line-number cells and the sign marker.
// A zero line number is rendered blank (the side does not apply to this line).
func (r *diffRowRenderer) layoutGutters(line diff.Line) {
	metrics := r.row.metrics
	r.oldNum.Show()
	r.newNum.Show()
	r.sign.Show()

	r.oldNum.Text = gutterLabel(line.OldNum)
	r.newNum.Text = gutterLabel(line.NewNum)
	r.oldNum.Color = metricsMuted(r.row.palette)
	r.newNum.Color = metricsMuted(r.row.palette)

	r.oldNum.Move(fyne.NewPos(0, 0))
	r.oldNum.Resize(fyne.NewSize(metrics.gutterW, metrics.height))
	r.newNum.Move(fyne.NewPos(metrics.gutterW, 0))
	r.newNum.Resize(fyne.NewSize(metrics.gutterW, metrics.height))

	r.sign.Text = signGlyph(line.Kind)
	r.sign.Color = signColor(r.row.palette, line.Kind)
	r.sign.Move(fyne.NewPos(metrics.gutterW*2, 0))
	r.sign.Resize(fyne.NewSize(metrics.signW, metrics.height))
}

// unifiedCols is how many glyph columns of content fit from the content origin to
// the row's right edge — the visible window width for a unified line.
func (r *diffRowRenderer) unifiedCols() int {
	return columnsFor(r.width, r.row.metrics.contentX, r.row.metrics)
}

// buildEmphasis lays an emphasis rectangle behind each intra-line change run that
// falls in the visible horizontal window, sized by its on-screen rune count.
func (r *diffRowRenderer) buildEmphasis(line diff.Line) {
	emphColor := emphasisColor(r.row.palette, line.Kind)
	if len(line.Segments) == 0 || emphColor == (color.NRGBA{}) {
		return
	}

	metrics := r.row.metrics
	maxCols := r.unifiedCols()
	col := 0
	for _, seg := range line.Segments {
		runes := utf8.RuneCountInString(seg.Text)
		if seg.Intraline && runes > 0 {
			if vis, screenCol, ok := windowRun(seg.Text, runes, col, r.row.hScroll, maxCols); ok {
				rect := r.acquireEmph()
				rect.FillColor = emphColor
				rect.Move(fyne.NewPos(metrics.contentX+float32(screenCol)*metrics.advance, 0))
				rect.Resize(fyne.NewSize(float32(utf8.RuneCountInString(vis))*metrics.advance, metrics.height))
			}
		}
		col += runes
	}
}

// buildTexts lays out the line content as syntax-colored runs when highlight
// tokens are available, falling back to a single foreground run otherwise.
func (r *diffRowRenderer) buildTexts(line diff.Line) {
	if len(r.row.data.tokens) > 0 {
		r.buildHighlightedTexts()

		return
	}
	r.buildPlainText(line.Content)
}

// buildHighlightedTexts positions one pooled text per highlight token, windowed to
// the visible horizontal range so only on-screen glyphs are laid out.
func (r *diffRowRenderer) buildHighlightedTexts() {
	metrics := r.row.metrics
	maxCols := r.unifiedCols()
	col := 0
	for _, tok := range r.row.data.tokens {
		runes := utf8.RuneCountInString(tok.Text)
		if runes == 0 {
			continue
		}
		if vis, screenCol, ok := windowRun(tok.Text, runes, col, r.row.hScroll, maxCols); ok {
			txt := r.acquireText()
			setMonoText(txt, vis, tok.Color, r.row.textSize, tok.Bold, tok.Italic)
			txt.Move(fyne.NewPos(metrics.contentX+float32(screenCol)*metrics.advance, 0))
		}
		col += runes
	}
}

// buildPlainText renders the line in the foreground color when no syntax tokens
// are present (e.g. before highlighting finishes), windowed to the visible range.
func (r *diffRowRenderer) buildPlainText(content string) {
	if content == "" {
		return
	}
	metrics := r.row.metrics
	runes := utf8.RuneCountInString(content)
	if vis, screenCol, ok := windowRun(content, runes, 0, r.row.hScroll, r.unifiedCols()); ok {
		txt := r.acquireText()
		setMonoText(txt, vis, r.row.palette.foreground, r.row.textSize, false, false)
		txt.Move(fyne.NewPos(metrics.contentX+float32(screenCol)*metrics.advance, 0))
	}
}

// gutterLabel renders a 1-based line number, or blank for the zero sentinel.
func gutterLabel(num int) string {
	if num == 0 {
		return ""
	}

	return strconv.Itoa(num)
}

// metricsMuted returns the gutter text color.
func metricsMuted(pal palette) color.NRGBA { return pal.muted }

// signColor picks the +/- marker color: emphasis tints for changes, muted for
// context.
func signColor(pal palette, kind diff.LineKind) color.NRGBA {
	switch kind {
	case diff.LineAdded:
		return pal.addEmph
	case diff.LineDeleted:
		return pal.delEmph
	case diff.LineContext:
		return pal.muted
	default:
		return pal.muted
	}
}

// emphasisColor returns the intra-line highlight color for a line kind, or the
// zero value when the kind carries no emphasis.
func emphasisColor(pal palette, kind diff.LineKind) color.NRGBA {
	switch kind {
	case diff.LineAdded:
		return pal.addEmph
	case diff.LineDeleted:
		return pal.delEmph
	case diff.LineContext:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	default:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	}
}

// applyTint paints the row's column tint for hover or selection: the hovered
// column takes priority, otherwise the selected column (selCol) persists. It
// covers a unified line edge to edge or just one split column, each in its own
// kind color (brightened green/red for changes, the neutral overlay for context).
// Hunk separators never tint. It runs from both Refresh and Layout because the
// split geometry depends on the row width. The tint sits above the cell
// backgrounds but below emphasis and text (see Objects), so a hovered or selected
// line brightens without hiding its glyphs or intra-line emphasis.
func (r *diffRowRenderer) applyTint() {
	col := r.row.selCol
	if r.row.hovered {
		col = r.row.hoverCol
	}
	if col == colNone || r.row.data.kind == rowSeparator {
		r.hover.Hide()

		return
	}
	r.hover.Show()
	height := r.row.metrics.height

	if r.row.data.kind != rowSplit {
		r.hover.FillColor = hoverColor(r.row.palette, r.row.data.line.Kind)
		r.hover.Move(fyne.NewPos(0, 0))
		r.hover.Resize(fyne.NewSize(r.width, height))

		return
	}

	mid := r.width / 2
	if col == hoverRight {
		r.hover.FillColor = hoverColor(r.row.palette, cellKind(&r.row.data.right))
		r.hover.Move(fyne.NewPos(mid, 0))
		r.hover.Resize(fyne.NewSize(r.width-mid, height))

		return
	}
	r.hover.FillColor = hoverColor(r.row.palette, cellKind(&r.row.data.left))
	r.hover.Move(fyne.NewPos(0, 0))
	r.hover.Resize(fyne.NewSize(mid, height))
}

// cellKind reports a split cell's line kind, treating an absent cell as context
// so hovering a blank column shows the neutral tint rather than a stale color.
func cellKind(cell *splitCell) diff.LineKind {
	if !cell.present {
		return diff.LineContext
	}

	return cell.line.Kind
}

// hoverColor is the hover tint for a line kind: the line's own brightened green or
// red for changes, the neutral overlay for context (and any other kind).
func hoverColor(pal palette, kind diff.LineKind) color.NRGBA {
	switch kind {
	case diff.LineAdded:
		return pal.addHover
	case diff.LineDeleted:
		return pal.delHover
	case diff.LineContext:
		return pal.overlay
	default:
		return pal.overlay
	}
}

// splitDividerWidth is the logical width of the rule between the two columns.
const splitDividerWidth float32 = 1

// buildSplit lays out a split row's two columns against the current width: a
// tinted background, gutter, and (truncated) content for each of the old (left)
// and new (right) cells, with a divider down the middle. It is width-dependent,
// so it runs from both Layout (on resize) and Refresh (on data change, using the
// cached width). Content beyond a column's width is clipped so a long line never
// bleeds into the opposite column.
func (r *diffRowRenderer) buildSplit(width float32) {
	metrics := r.row.metrics
	r.background.Hide()
	r.sign.Hide()
	r.header.Hide()
	r.leftBg.Show()
	r.rightBg.Show()
	r.divider.Show()
	r.oldNum.Show()
	r.newNum.Show()

	mid := width / 2
	left := &r.row.data.left
	right := &r.row.data.right

	r.leftBg.FillColor = splitCellBg(r.row.palette, left)
	r.leftBg.Move(fyne.NewPos(0, 0))
	r.leftBg.Resize(fyne.NewSize(mid, metrics.height))
	r.rightBg.FillColor = splitCellBg(r.row.palette, right)
	r.rightBg.Move(fyne.NewPos(mid, 0))
	r.rightBg.Resize(fyne.NewSize(width-mid, metrics.height))
	r.divider.FillColor = r.row.palette.border
	r.divider.Move(fyne.NewPos(mid, 0))
	r.divider.Resize(fyne.NewSize(splitDividerWidth, metrics.height))

	r.oldNum.Text = splitGutterLabel(left, true)
	r.oldNum.Color = r.row.palette.muted
	r.oldNum.Move(fyne.NewPos(metrics.padding, 0))
	r.oldNum.Resize(fyne.NewSize(metrics.gutterW, metrics.height))
	rightGutterX := mid + splitDividerWidth + metrics.padding
	r.newNum.Text = splitGutterLabel(right, false)
	r.newNum.Color = r.row.palette.muted
	r.newNum.Move(fyne.NewPos(rightGutterX, 0))
	r.newNum.Resize(fyne.NewSize(metrics.gutterW, metrics.height))

	leftContentX := metrics.padding + metrics.gutterW + metrics.advance
	rightContentX := rightGutterX + metrics.gutterW + metrics.advance
	leftCols := columnsFor(mid, leftContentX, metrics)
	rightCols := columnsFor(width, rightContentX, metrics)

	// Emphasis before text: Objects() paints all emphasis rects beneath all text
	// runs, so the intra-line change tint sits behind the glyphs in each column —
	// matching the unified view, which split rows previously omitted entirely.
	r.appendCellEmphasis(left, leftContentX, leftCols)
	r.appendCellEmphasis(right, rightContentX, rightCols)
	r.appendCellTexts(left, leftContentX, leftCols)
	r.appendCellTexts(right, rightContentX, rightCols)
}

// appendCellEmphasis lays an emphasis rectangle behind each intra-line change run
// of a split cell — the per-character change tint — offset to the column's
// contentX and clipped at maxCols so it never crosses the divider. It mirrors the
// unified view's buildEmphasis, which the split path previously lacked.
func (r *diffRowRenderer) appendCellEmphasis(cell *splitCell, contentX float32, maxCols int) {
	if !cell.present || maxCols <= 0 {
		return
	}
	emphColor := emphasisColor(r.row.palette, cell.line.Kind)
	if len(cell.line.Segments) == 0 || emphColor == (color.NRGBA{}) {
		return
	}

	metrics := r.row.metrics
	col := 0
	for _, seg := range cell.line.Segments {
		runes := utf8.RuneCountInString(seg.Text)
		if seg.Intraline && runes > 0 {
			if vis, screenCol, ok := windowRun(seg.Text, runes, col, r.row.hScroll, maxCols); ok {
				rect := r.acquireEmph()
				rect.FillColor = emphColor
				rect.Move(fyne.NewPos(contentX+float32(screenCol)*metrics.advance, 0))
				rect.Resize(fyne.NewSize(float32(utf8.RuneCountInString(vis))*metrics.advance, metrics.height))
			}
		}
		col += runes
	}
}

// columnsFor reports how many monospace glyphs fit between contentX and the
// column's right edge, leaving one padding gap before the edge.
func columnsFor(edge, contentX float32, metrics rowMetrics) int {
	avail := edge - metrics.padding - contentX
	if avail <= 0 {
		return 0
	}

	return int(avail / metrics.advance)
}

// windowRun clips a run occupying logical columns [col, col+runes) to the visible
// horizontal window [hScroll, hScroll+maxCols). It returns the on-screen slice of
// the run, the screen column (0-based from the content origin) where it begins,
// and false when none of the run is in the window. This single helper does both
// the horizontal-scroll offset (hScroll) and the right-edge clip (maxCols), so a
// line renders only its visible glyphs under a frozen gutter.
func windowRun(text string, runes, col, hScroll, maxCols int) (visible string, screenCol int, ok bool) {
	start := max(col, hScroll)
	end := min(col+runes, hScroll+maxCols)
	if start >= end {
		return "", 0, false
	}

	return runeSlice(text, start-col, end-col), start - hScroll, true
}

// runeSlice returns runes [from, to) of s, counted by rune so multibyte content
// is sliced on rune boundaries. from/to are clamped by the caller via windowRun.
func runeSlice(s string, from, to int) string {
	start, idx := -1, 0
	for bytePos := range s {
		if idx == from {
			start = bytePos
		}
		if idx == to {
			return s[start:bytePos]
		}
		idx++
	}
	if start < 0 {
		return ""
	}

	return s[start:]
}

// splitCellBg tints a split cell by its line kind; an absent or context cell is
// transparent so the view background shows through.
func splitCellBg(pal palette, cell *splitCell) color.Color {
	if !cell.present {
		return color.Transparent
	}
	switch cell.line.Kind {
	case diff.LineAdded:
		return pal.addBg
	case diff.LineDeleted:
		return pal.delBg
	case diff.LineContext:
		return color.Transparent
	default:
		return color.Transparent
	}
}

// splitGutterLabel renders the old (or new) line number for a split cell, blank
// when the cell is absent.
func splitGutterLabel(cell *splitCell, old bool) string {
	if !cell.present {
		return ""
	}
	if old {
		return gutterLabel(cell.line.OldNum)
	}

	return gutterLabel(cell.line.NewNum)
}

// appendCellTexts lays out one split cell's content at contentX as syntax-colored
// runs (or plain foreground text before highlighting), stopping at maxCols so the
// run never crosses into the other column.
func (r *diffRowRenderer) appendCellTexts(cell *splitCell, contentX float32, maxCols int) {
	if !cell.present || maxCols <= 0 {
		return
	}
	if len(cell.tokens) == 0 {
		r.appendCellPlain(cell.line.Content, contentX, maxCols)

		return
	}

	metrics := r.row.metrics
	col := 0
	for _, tok := range cell.tokens {
		runes := utf8.RuneCountInString(tok.Text)
		if runes == 0 {
			continue
		}
		if vis, screenCol, ok := windowRun(tok.Text, runes, col, r.row.hScroll, maxCols); ok {
			txt := r.acquireText()
			setMonoText(txt, vis, tok.Color, r.row.textSize, tok.Bold, tok.Italic)
			txt.Move(fyne.NewPos(contentX+float32(screenCol)*metrics.advance, 0))
		}
		col += runes
	}
}

// appendCellPlain renders a split cell's content as a single foreground run,
// windowed to the visible range, used before highlight tokens arrive.
func (r *diffRowRenderer) appendCellPlain(content string, contentX float32, maxCols int) {
	if content == "" {
		return
	}
	runes := utf8.RuneCountInString(content)
	if vis, screenCol, ok := windowRun(content, runes, 0, r.row.hScroll, maxCols); ok {
		txt := r.acquireText()
		setMonoText(txt, vis, r.row.palette.foreground, r.row.textSize, false, false)
		txt.Move(fyne.NewPos(contentX+float32(screenCol)*r.row.metrics.advance, 0))
	}
}
