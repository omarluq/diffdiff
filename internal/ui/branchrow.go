package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/omarluq/diffdiff/internal/icons"
)

// branchRow is a directory node in the nested file view: a Material folder icon
// (resolved by the caller to the directory's open/closed and light/dark variant)
// followed by the directory name. It is a recycled tree node; set swaps in new
// content. The enclosing widget.Tree supplies depth indentation, and its
// disclosure caret is hidden (see the theme's Icon), so the folder icon alone
// signals open/closed and a row tap toggles it.
type branchRow struct {
	widget.BaseWidget

	palette palette
	label   string
	icon    fyne.Resource
	hasData bool
	advance float32
}

// newBranchRow builds an empty directory node for the given palette.
func newBranchRow(pal palette) *branchRow {
	row := &branchRow{
		BaseWidget: widget.BaseWidget{},
		palette:    pal,
		label:      "",
		icon:       nil,
		hasData:    false,
		advance:    measureAdvance(fileRowTextSize),
	}
	row.ExtendBaseWidget(row)

	return row
}

// set swaps in a new label, folder icon, and palette, then refreshes.
func (br *branchRow) set(label string, icon fyne.Resource, pal palette) {
	br.label = label
	br.icon = icon
	br.palette = pal
	br.hasData = true
	br.Refresh()
}

// CreateRenderer assembles the node's reusable folder icon and name text. The
// icon resource is assigned in Refresh.
func (br *branchRow) CreateRenderer() fyne.WidgetRenderer {
	icon := canvas.NewImageFromResource(nil)
	icon.FillMode = canvas.ImageFillContain

	name := canvas.NewText("", br.palette.foreground)
	name.TextSize = fileRowTextSize
	name.TextStyle = monoStyle()

	return &branchRowRenderer{
		row:    br,
		icon:   icon,
		name:   name,
		height: lineHeight(fileRowTextSize),
	}
}

// branchRowRenderer lays a directory node out as a folder icon at the left edge
// followed by the directory name one glyph-gap to its right.
type branchRowRenderer struct {
	row    *branchRow
	icon   *canvas.Image
	name   *canvas.Text
	height float32
	// iconKey is the resource name of the folder icon currently shown, so Refresh
	// skips re-decoding/re-scaling the PNG when the icon is unchanged.
	iconKey string
	// objs caches the Objects() slice (a fixed icon+name pair) so the render walk
	// and mouse hit-tests don't reallocate it per call.
	objs []fyne.CanvasObject
}

// Destroy has nothing to release.
func (r *branchRowRenderer) Destroy() {}

// MinSize reports one text line's height; width is supplied by the tree.
func (r *branchRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, r.height)
}

// Layout is a no-op: positions are absolute and assigned in Refresh.
func (r *branchRowRenderer) Layout(_ fyne.Size) {}

// Refresh repaints the node for the current directory name and folder icon.
func (r *branchRowRenderer) Refresh() {
	if !r.row.hasData {
		return
	}
	r.icon.Resize(fyne.NewSize(r.height, r.height))
	r.icon.Move(fyne.NewPos(0, 0))

	key := ""
	if r.row.icon != nil {
		key = r.row.icon.Name()
	}
	if key != r.iconKey {
		r.iconKey = key
		if img := icons.Decoded(r.row.icon); img != nil {
			r.icon.Image = img
			r.icon.Resource = nil
		} else {
			r.icon.Resource = r.row.icon
			r.icon.Image = nil
		}
		r.icon.Refresh()
	}

	r.name.Text = r.row.label
	r.name.Color = r.row.palette.foreground
	r.name.Move(fyne.NewPos(r.height+r.row.advance, 0))
	canvas.Refresh(r.row)
}

// Objects returns the node's drawables in paint order. The fixed icon+name pair
// is cached so repeated render-walk and hit-test calls don't reallocate it.
func (r *branchRowRenderer) Objects() []fyne.CanvasObject {
	if r.objs == nil {
		r.objs = []fyne.CanvasObject{r.icon, r.name}
	}

	return r.objs
}
