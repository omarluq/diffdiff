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
	rowSplit
)

// splitCell is one side (old or new) of a split-view row. present is false when
// the line has no counterpart on this side (rendered as a blank cell). tokens is
// filled asynchronously after highlighting; hlIndex records the cell's position
// within the reconstructed old/new body so the async result can be indexed.
type splitCell struct {
	present bool
	line    diff.Line
	tokens  []highlight.Token
	hlIndex int
}

// row is one entry in the flattened diff. Separator rows carry only header
// text. Unified line rows reference a diff.Line plus its cached highlighted
// tokens for the appropriate side (tokens/hlIndex/hlOld). Split rows pair an old
// cell (left) with a new cell (right). tokens is filled in asynchronously after
// highlighting; hlIndex/hlOld records which reconstructed-file line this row maps
// to so the async result can be indexed without re-deriving line offsets.
type row struct {
	kind    rowKind
	header  string
	line    diff.Line
	tokens  []highlight.Token
	hlIndex int
	hlOld   bool
	left    splitCell
	right   splitCell
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

// setRow swaps in new content and refreshes. metrics is re-applied too so a
// recycled row picks up new monospace measurements after a theme/font change
// (the list reuses widgets rather than recreating them). data is taken by
// pointer to avoid copying the wide row struct on every recycle.
func (dr *diffRow) setRow(data *row, pal palette, metrics rowMetrics) {
	dr.data = *data
	dr.palette = pal
	dr.metrics = metrics
	dr.hasData = true
	dr.Refresh()
}

// CreateRenderer wires the row's canvas objects into a renderer.
func (dr *diffRow) CreateRenderer() fyne.WidgetRenderer {
	rend := &diffRowRenderer{
		row:        dr,
		background: canvas.NewRectangle(color.Transparent),
		leftBg:     canvas.NewRectangle(color.Transparent),
		rightBg:    canvas.NewRectangle(color.Transparent),
		divider:    canvas.NewRectangle(color.Transparent),
		oldNum:     dr.newText(dr.palette.muted, fyne.TextAlignTrailing),
		newNum:     dr.newText(dr.palette.muted, fyne.TextAlignTrailing),
		sign:       dr.newText(dr.palette.muted, fyne.TextAlignCenter),
		header:     dr.newText(dr.palette.muted, fyne.TextAlignLeading),
		emphasis:   nil,
		texts:      nil,
		width:      0,
	}

	return rend
}

// newText builds an empty monospace canvas text in this row's size and style,
// used for the fixed chrome cells (gutters, sign, header) whose content is set
// later; the pooled content runs use acquireText instead.
func (dr *diffRow) newText(col color.Color, align fyne.TextAlign) *canvas.Text {
	return newMonoText("", col, dr.textSize, false, align)
}

// diffRowRenderer lays out one diff row. In unified mode background spans the
// full width; in split mode leftBg/rightBg tint the two columns and divider
// separates them. emphasis holds intra-line change rectangles; texts holds the
// syntax-colored runs (both columns in split mode). Gutter/sign texts are
// reused; emphasis and texts are rebuilt per refresh because their counts vary.
// width caches the last laid-out width so a data-only Refresh can re-place the
// width-dependent split columns without waiting for a resize.
// emphasis and texts are reused pools, not rebuilt per refresh: liveEmph/
// liveTexts mark how many are active this frame, acquireEmph/acquireText hand
// out (and grow) entries, and the surplus is hidden. This keeps a scroll frame
// from allocating a canvas.Text per token.
type diffRowRenderer struct {
	row        *diffRow
	background *canvas.Rectangle
	leftBg     *canvas.Rectangle
	rightBg    *canvas.Rectangle
	divider    *canvas.Rectangle
	oldNum     *canvas.Text
	newNum     *canvas.Text
	sign       *canvas.Text
	header     *canvas.Text
	emphasis   []*canvas.Rectangle
	texts      []*canvas.Text
	liveEmph   int
	liveTexts  int
	width      float32
	// objs caches the Objects() slice. The render walk and every mouse hit-test
	// call Objects(); rebuilding the slice only when the pooled object count
	// changes avoids a per-walk allocation (the top render-path allocator).
	objs []fyne.CanvasObject
}

// acquireText returns the next pooled text run, growing the pool if needed and
// marking it visible. The caller sets its content, color, style, and position.
func (r *diffRowRenderer) acquireText() *canvas.Text {
	if r.liveTexts < len(r.texts) {
		txt := r.texts[r.liveTexts]
		txt.Show()
		r.liveTexts++

		return txt
	}
	txt := newMonoText("", r.row.palette.foreground, r.row.textSize, false, fyne.TextAlignLeading)
	r.texts = append(r.texts, txt)
	r.liveTexts++

	return txt
}

// acquireEmph returns the next pooled emphasis rectangle, growing the pool if
// needed and marking it visible. The caller sets its color and geometry.
func (r *diffRowRenderer) acquireEmph() *canvas.Rectangle {
	if r.liveEmph < len(r.emphasis) {
		rect := r.emphasis[r.liveEmph]
		rect.Show()
		r.liveEmph++

		return rect
	}
	rect := canvas.NewRectangle(color.Transparent)
	r.emphasis = append(r.emphasis, rect)
	r.liveEmph++

	return rect
}

// hideSurplus hides the pooled texts and emphasis rectangles not used this
// frame, so a row recycled from a denser line draws no stale leftovers.
func (r *diffRowRenderer) hideSurplus() {
	for i := r.liveTexts; i < len(r.texts); i++ {
		r.texts[i].Hide()
	}
	for i := r.liveEmph; i < len(r.emphasis); i++ {
		r.emphasis[i].Hide()
	}
}

// rebuildSplit lays out the split row from scratch with the pool reset and the
// surplus hidden. buildSplit is width-dependent and runs from both Refresh (which
// brackets the pool itself) and Layout (which does not), so the Layout path must
// reset/hide here or it would stack a fresh set of runs on top of the previous
// frame's — the overlap that showed up on long split lines.
func (r *diffRowRenderer) rebuildSplit(width float32) {
	r.liveTexts = 0
	r.liveEmph = 0
	r.buildSplit(width)
	r.hideSurplus()
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
	r.liveTexts = 0
	r.liveEmph = 0
	switch r.row.data.kind {
	case rowSeparator:
		r.refreshSeparator()
	case rowSplit:
		r.buildSplit(r.width)
	case rowLine:
		r.refreshLine()
	default:
		r.refreshLine()
	}
	r.hideSurplus()
	canvas.Refresh(r.row)
}

// Layout repositions cells when the row is resized. The unified background must
// stretch to the new width so the change color fills the whole line; split rows
// rebuild their two columns against the new width.
func (r *diffRowRenderer) Layout(size fyne.Size) {
	r.width = size.Width
	r.background.Resize(fyne.NewSize(size.Width, r.row.metrics.height))
	r.background.Move(fyne.NewPos(0, 0))
	if !r.row.hasData {
		return
	}
	switch r.row.data.kind {
	case rowSeparator:
		r.header.Resize(fyne.NewSize(size.Width-r.row.metrics.padding, r.row.metrics.height))
	case rowSplit:
		r.rebuildSplit(size.Width)
	case rowLine:
	}
}

// Objects returns every drawable in paint order: backgrounds (full-width unified
// plus the two split columns and their divider), intra-line emphasis, gutter/sign
// chrome, then the syntax text runs on top.
func (r *diffRowRenderer) Objects() []fyne.CanvasObject {
	// 8 fixed cells (4 backgrounds/divider + 4 gutter/sign/header) plus the two
	// pools. The pools only grow, so the cached slice is valid until the count
	// changes; rebuild only then.
	want := 8 + len(r.emphasis) + len(r.texts)
	if len(r.objs) == want {
		return r.objs
	}

	r.objs = make([]fyne.CanvasObject, 0, want)
	r.objs = append(r.objs, r.background, r.leftBg, r.rightBg, r.divider)
	for _, emph := range r.emphasis {
		r.objs = append(r.objs, emph)
	}
	r.objs = append(r.objs, r.oldNum, r.newNum, r.sign, r.header)
	for _, txt := range r.texts {
		r.objs = append(r.objs, txt)
	}

	return r.objs
}
