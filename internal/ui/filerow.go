package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// fileRowTextSize is the point size for file-list rows; slightly larger than
// diff text since the panel is less dense.
const fileRowTextSize float32 = 13

// glyphGap is the spacing (in glyph advances) between the status glyph, counts,
// and the path within a row.
const glyphGap = 1

// fileRow is one row in the FileList: a status glyph, +adds/-dels counts, and
// the path with basename and fuzzy matches emphasized. It is a recycled list
// item; set swaps in new content without reallocating the widget.
type fileRow struct {
	widget.BaseWidget

	palette palette
	entry   fileEntry
	hasData bool
	advance float32
}

// newFileRow builds an empty row for the given palette.
func newFileRow(pal palette) *fileRow {
	row := &fileRow{
		BaseWidget: widget.BaseWidget{},
		palette:    pal,
		entry:      fileEntry{file: nil, matched: nil},
		hasData:    false,
		advance:    measureAdvance(fileRowTextSize),
	}
	row.ExtendBaseWidget(row)

	return row
}

// set swaps in a new entry/palette and refreshes.
func (fr *fileRow) set(entry fileEntry, pal palette) {
	fr.entry = entry
	fr.palette = pal
	fr.hasData = true
	fr.Refresh()
}

// CreateRenderer assembles the row's renderer with reusable chrome objects.
func (fr *fileRow) CreateRenderer() fyne.WidgetRenderer {
	icon := canvas.NewImageFromResource(nil)
	icon.FillMode = canvas.ImageFillContain

	return &fileRowRenderer{
		row:    fr,
		icon:   icon,
		glyph:  fr.newText("", fr.palette.foreground, false),
		adds:   fr.newText("", fr.palette.addEmph, false),
		dels:   fr.newText("", fr.palette.delEmph, false),
		runes:  nil,
		height: fyne.MeasureText(monoText, fileRowTextSize, monoStyle()).Height,
	}
}

// newText builds a monospace canvas text at the row's size.
func (fr *fileRow) newText(content string, col color.Color, bold bool) *canvas.Text {
	txt := canvas.NewText(content, col)
	txt.TextSize = fileRowTextSize
	style := monoStyle()
	style.Bold = bold
	txt.TextStyle = style

	return txt
}

// monoStyle is the shared monospace text style for file-list rows.
func monoStyle() fyne.TextStyle {
	return fyne.TextStyle{
		Bold: false, Italic: false, Monospace: true, Symbol: false, TabWidth: 0, Underline: false,
	}
}
