package ui

import (
	"image/color"
	"strconv"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"github.com/omarluq/diffdiff/internal/diff"
)

// refreshSeparator renders a hunk header (@@ ... @@) on the surface color and
// hides the line-specific cells.
func (r *diffRowRenderer) refreshSeparator() {
	r.background.FillColor = r.row.palette.surface
	r.oldNum.Hide()
	r.newNum.Hide()
	r.sign.Hide()
	r.emphasis = nil
	r.texts = nil

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
	r.applyLineBackground(line.Kind)
	r.layoutGutters(line)
	r.buildEmphasis(line)
	r.buildTexts(line)
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
	r.emphasis = r.emphasis[:0]
	if len(line.Segments) == 0 || emphColor == (color.NRGBA{}) {
		return
	}

	metrics := r.row.metrics
	col := 0
	for _, seg := range line.Segments {
		runes := utf8.RuneCountInString(seg.Text)
		if seg.Intraline && runes > 0 {
			rect := canvas.NewRectangle(emphColor)
			rect.Move(fyne.NewPos(metrics.contentX+float32(col)*metrics.advance, 0))
			rect.Resize(fyne.NewSize(float32(runes)*metrics.advance, metrics.height))
			r.emphasis = append(r.emphasis, rect)
		}
		col += runes
	}
}

// buildTexts lays out the line content as syntax-colored runs when highlight
// tokens are available, falling back to a single foreground run otherwise.
func (r *diffRowRenderer) buildTexts(line diff.Line) {
	r.texts = r.texts[:0]
	if len(r.row.data.tokens) > 0 {
		r.buildHighlightedTexts()

		return
	}
	r.buildPlainText(line.Content)
}

// buildHighlightedTexts positions one canvas.Text per highlight token, advancing
// the column cursor by each token's rune count.
func (r *diffRowRenderer) buildHighlightedTexts() {
	metrics := r.row.metrics
	col := 0
	for _, tok := range r.row.data.tokens {
		runes := utf8.RuneCountInString(tok.Text)
		if runes == 0 {
			continue
		}
		txt := r.row.newText(tok.Text, tok.Color, fyne.TextAlignLeading)
		txt.TextStyle.Bold = tok.Bold
		txt.TextStyle.Italic = tok.Italic
		txt.Move(fyne.NewPos(metrics.contentX+float32(col)*metrics.advance, 0))
		r.texts = append(r.texts, txt)
		col += runes
	}
}

// buildPlainText renders the whole line in the foreground color when no syntax
// tokens are present (e.g. before highlighting finishes or for plain text).
func (r *diffRowRenderer) buildPlainText(content string) {
	if content == "" {
		return
	}
	txt := r.row.newText(content, r.row.palette.foreground, fyne.TextAlignLeading)
	txt.Move(fyne.NewPos(r.row.metrics.contentX, 0))
	r.texts = append(r.texts, txt)
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
