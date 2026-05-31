package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/highlight"
)

// rowKind distinguishes a hunk separator from an ordinary diff line so the
// renderer can pick its layout without re-inspecting the underlying model.
type rowKind uint8

const (
	rowLine rowKind = iota
	rowSeparator
)

// row is one entry in the flattened diff. Separator rows carry only header
// text; line rows reference a diff.Line plus its cached highlighted tokens for
// the appropriate side. tokens is filled in asynchronously after highlighting;
// hlIndex/hlOld records which reconstructed-file line this row maps to so the
// async result can be indexed without re-deriving line offsets.
type row struct {
	kind    rowKind
	header  string
	line    diff.Line
	tokens  []highlight.Token
	hlIndex int
	hlOld   bool
	gutterW float32
}

// signCell renders the +/-/space marker; its glyph width is one mono advance.
func signGlyph(kind diff.LineKind) string {
	switch kind {
	case diff.LineAdded:
		return "+"
	case diff.LineDeleted:
		return "-"
	case diff.LineContext:
		return " "
	default:
		return " "
	}
}

// diffRow is the per-list-item widget. One instance is created per visible
// slot and reused across scroll positions via setRow, so its CanvasObjects are
// allocated once and only their content/visibility change on update.
type diffRow struct {
	widget.BaseWidget

	metrics  rowMetrics
	palette  palette
	data     row
	hasData  bool
	textSize float32
}

// rowMetrics holds the monospace measurements a row needs to position cells.
type rowMetrics struct {
	advance  float32
	height   float32
	padding  float32
	gutterW  float32
	signW    float32
	contentX float32
}

// newDiffRow builds an empty row widget; setRow populates it before display.
func newDiffRow(metrics rowMetrics, pal palette, textSize float32) *diffRow {
	dr := &diffRow{
		BaseWidget: widget.BaseWidget{},
		metrics:    metrics,
		palette:    pal,
		data:       row{kind: rowLine, header: "", line: diff.Line{}, tokens: nil, gutterW: 0},
		hasData:    false,
		textSize:   textSize,
	}
	dr.ExtendBaseWidget(dr)

	return dr
}

// setRow swaps in new content and refreshes; metrics stay fixed for the row's
// lifetime so only the renderer's objects are rebuilt. data is taken by pointer
// to avoid copying the wide row struct on every recycle.
func (dr *diffRow) setRow(data *row, pal palette) {
	dr.data = *data
	dr.palette = pal
	dr.hasData = true
	dr.Refresh()
}

// CreateRenderer wires the row's canvas objects into a renderer.
func (dr *diffRow) CreateRenderer() fyne.WidgetRenderer {
	rend := &diffRowRenderer{
		row:        dr,
		background: canvas.NewRectangle(color.Transparent),
		oldNum:     dr.newText("", dr.palette.muted, fyne.TextAlignTrailing),
		newNum:     dr.newText("", dr.palette.muted, fyne.TextAlignTrailing),
		sign:       dr.newText("", dr.palette.muted, fyne.TextAlignCenter),
		header:     dr.newText("", dr.palette.muted, fyne.TextAlignLeading),
		emphasis:   nil,
		texts:      nil,
	}

	return rend
}

// newText builds a monospace canvas text in this row's size and style.
func (dr *diffRow) newText(content string, col color.Color, align fyne.TextAlign) *canvas.Text {
	txt := canvas.NewText(content, col)
	txt.TextSize = dr.textSize
	txt.TextStyle = fyne.TextStyle{
		Bold: false, Italic: false, Monospace: true, Symbol: false, TabWidth: 0, Underline: false,
	}
	txt.Alignment = align

	return txt
}

// diffRowRenderer lays out one diff row. background spans the full width;
// emphasis holds intra-line change rectangles; texts holds the syntax-colored
// runs. Gutter/sign texts are reused; emphasis and texts are rebuilt per setRow
// because their counts vary by line.
type diffRowRenderer struct {
	row        *diffRow
	background *canvas.Rectangle
	oldNum     *canvas.Text
	newNum     *canvas.Text
	sign       *canvas.Text
	header     *canvas.Text
	emphasis   []*canvas.Rectangle
	texts      []*canvas.Text
}

// Destroy has no resources to release.
func (r *diffRowRenderer) Destroy() {}

// MinSize reports a single mono line's height; width is driven by the list.
func (r *diffRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, r.row.metrics.height)
}

// Refresh rebuilds the variable-count objects (emphasis rects, text runs) and
// repositions everything for the current row data.
func (r *diffRowRenderer) Refresh() {
	if !r.row.hasData {
		return
	}
	if r.row.data.kind == rowSeparator {
		r.refreshSeparator()
	} else {
		r.refreshLine()
	}
	canvas.Refresh(r.row)
}

// Layout repositions cells when the row is resized; the background must stretch
// to the new width so the change color fills the whole line.
func (r *diffRowRenderer) Layout(size fyne.Size) {
	r.background.Resize(fyne.NewSize(size.Width, r.row.metrics.height))
	r.background.Move(fyne.NewPos(0, 0))
	if r.row.hasData && r.row.data.kind == rowSeparator {
		r.header.Resize(fyne.NewSize(size.Width-r.row.metrics.padding, r.row.metrics.height))
	}
}

// Objects returns every drawable in paint order: background, intra-line
// emphasis, gutter/sign chrome, then the syntax text runs on top.
func (r *diffRowRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, 4+len(r.emphasis)+len(r.texts))
	objs = append(objs, r.background)
	for _, emph := range r.emphasis {
		objs = append(objs, emph)
	}
	objs = append(objs, r.oldNum, r.newNum, r.sign, r.header)
	for _, txt := range r.texts {
		objs = append(objs, txt)
	}

	return objs
}
