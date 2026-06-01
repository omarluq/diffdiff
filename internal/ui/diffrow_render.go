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

// buildEmphasis lays an emphasis rectangle behind each intra-line change run,
// sized by the run's rune count at the mono advance.
func (r *diffRowRenderer) buildEmphasis(line diff.Line) {
	emphColor := emphasisColor(r.row.palette, line.Kind)
	if len(line.Segments) == 0 || emphColor == (color.NRGBA{}) {
		return
	}

	metrics := r.row.metrics
	col := 0
	for _, seg := range line.Segments {
		runes := utf8.RuneCountInString(seg.Text)
		if seg.Intraline && runes > 0 {
			rect := r.acquireEmph()
			rect.FillColor = emphColor
			rect.Move(fyne.NewPos(metrics.contentX+float32(col)*metrics.advance, 0))
			rect.Resize(fyne.NewSize(float32(runes)*metrics.advance, metrics.height))
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

// buildHighlightedTexts positions one pooled text per highlight token, advancing
// the column cursor by each token's rune count.
func (r *diffRowRenderer) buildHighlightedTexts() {
	metrics := r.row.metrics
	col := 0
	for _, tok := range r.row.data.tokens {
		runes := utf8.RuneCountInString(tok.Text)
		if runes == 0 {
			continue
		}
		txt := r.acquireText()
		setMonoText(txt, tok.Text, tok.Color, r.row.textSize, tok.Bold, tok.Italic)
		txt.Move(fyne.NewPos(metrics.contentX+float32(col)*metrics.advance, 0))
		col += runes
	}
}

// buildPlainText renders the whole line in the foreground color when no syntax
// tokens are present (e.g. before highlighting finishes or for plain text).
func (r *diffRowRenderer) buildPlainText(content string) {
	if content == "" {
		return
	}
	txt := r.acquireText()
	setMonoText(txt, content, r.row.palette.foreground, r.row.textSize, false, false)
	txt.Move(fyne.NewPos(r.row.metrics.contentX, 0))
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

	r.appendCellTexts(left, leftContentX, columnsFor(mid, leftContentX, metrics))
	r.appendCellTexts(right, rightContentX, columnsFor(width, rightContentX, metrics))
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
		if col >= maxCols {
			break
		}
		runes := utf8.RuneCountInString(tok.Text)
		if runes == 0 {
			continue
		}
		text := tok.Text
		if col+runes > maxCols {
			text = firstRunes(tok.Text, maxCols-col)
		}
		txt := r.acquireText()
		setMonoText(txt, text, tok.Color, r.row.textSize, tok.Bold, tok.Italic)
		txt.Move(fyne.NewPos(contentX+float32(col)*metrics.advance, 0))
		col += runes
	}
}

// appendCellPlain renders a split cell's content as a single foreground run,
// clipped to maxCols, used before highlight tokens arrive.
func (r *diffRowRenderer) appendCellPlain(content string, contentX float32, maxCols int) {
	if content == "" {
		return
	}
	text := content
	if utf8.RuneCountInString(content) > maxCols {
		text = firstRunes(content, maxCols)
	}
	txt := r.acquireText()
	setMonoText(txt, text, r.row.palette.foreground, r.row.textSize, false, false)
	txt.Move(fyne.NewPos(contentX, 0))
}

// firstRunes returns the first n runes of s (the whole string when it is
// shorter), used to clip overflowing split-column content.
func firstRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for index := range s {
		if count == n {
			return s[:index]
		}
		count++
	}

	return s
}
