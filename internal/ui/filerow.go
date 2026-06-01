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

// fileRow is one row in the FileList: a Material file-type icon, a status glyph,
// +adds/-dels counts, and the path with basename and fuzzy matches emphasized.
// It is a recycled list item; set swaps in new content without reallocating the
// widget. When basenameOnly is set (as a leaf of the nested tree view) only the
// file's base name is drawn, since the directory is already conveyed by the tree
// hierarchy.
type fileRow struct {
	widget.BaseWidget

	palette      palette
	entry        fileEntry
	hasData      bool
	advance      float32
	basenameOnly bool
}

// newFileRow builds an empty row for the given palette. basenameOnly renders
// just the file's base name (for nested-tree leaves) rather than the full path.
func newFileRow(pal palette, basenameOnly bool) *fileRow {
	row := &fileRow{
		BaseWidget:   widget.BaseWidget{},
		palette:      pal,
		entry:        fileEntry{file: nil, matched: nil},
		hasData:      false,
		advance:      measureAdvance(fileRowTextSize),
		basenameOnly: basenameOnly,
	}
	row.ExtendBaseWidget(row)

	return row
}

// set swaps in a new entry/palette and refreshes. The glyph advance is
// re-read (from the cached measurement) so a recycled row picks up new
// monospace metrics after a font change rather than keeping its stale advance.
func (fr *fileRow) set(entry fileEntry, pal palette) {
	fr.entry = entry
	fr.palette = pal
	fr.advance = measureAdvance(fileRowTextSize)
	fr.hasData = true
	fr.Refresh()
}

// CreateRenderer assembles the row's renderer with reusable chrome objects.
func (fr *fileRow) CreateRenderer() fyne.WidgetRenderer {
	icon := canvas.NewImageFromResource(nil)
	icon.FillMode = canvas.ImageFillContain

	return &fileRowRenderer{
		row:      fr,
		icon:     icon,
		glyph:    fr.newText("", fr.palette.foreground, false),
		adds:     fr.newText("", fr.palette.addEmph, false),
		dels:     fr.newText("", fr.palette.delEmph, false),
		segments: nil,
		height:   lineHeight(fileRowTextSize),
	}
}

// newText builds a monospace canvas text at the row's size.
func (fr *fileRow) newText(content string, col color.Color, bold bool) *canvas.Text {
	return newMonoText(content, col, fileRowTextSize, bold, fyne.TextAlignLeading)
}

// monoStyle is the shared monospace text style for file-list rows.
func monoStyle() fyne.TextStyle {
	return fyne.TextStyle{
		Bold: false, Italic: false, Monospace: true, Symbol: false, TabWidth: 0, Underline: false,
	}
}
