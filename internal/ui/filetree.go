package ui

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/theme"
)

// FileList is the changed-files panel: a virtualized, fuzzy-filterable list of
// files in a diff. Each row shows a status glyph, +adds/-dels counts, and the
// file path with its basename emphasized; under an active filter, the matched
// path runes are accented. Selecting a row invokes the onSelect callback.
type FileList struct {
	widget.BaseWidget

	onSelect func(*diff.File)
	files    []*diff.File
	visible  []fileEntry
	palette  palette
	list     *widget.List
}

// fileEntry is a file made visible by the current filter, paired with the
// fuzzy-matched rune positions (nil when unfiltered) used for emphasis.
type fileEntry struct {
	file    *diff.File
	matched map[int]bool
}

// NewFileList builds an empty file list; onSelect may be nil.
func NewFileList(onSelect func(*diff.File)) *FileList {
	list := &FileList{
		BaseWidget: widget.BaseWidget{},
		onSelect:   onSelect,
		files:      nil,
		visible:    nil,
		palette:    palette{},
		list:       nil,
	}
	list.ExtendBaseWidget(list)
	list.buildList()

	return list
}

// buildList wires the recycling list and its select handler once.
func (l *FileList) buildList() {
	l.list = widget.NewList(
		func() int { return len(l.visible) },
		func() fyne.CanvasObject { return newFileRow(l.palette) },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row, ok := obj.(*fileRow)
			if !ok || id < 0 || id >= len(l.visible) {
				return
			}
			row.set(l.visible[id], l.palette)
		},
	)
	l.list.OnSelected = l.handleSelect
}

// handleSelect forwards a row selection to onSelect, ignoring out-of-range ids.
func (l *FileList) handleSelect(id widget.ListItemID) {
	if l.onSelect == nil || id < 0 || id >= len(l.visible) {
		return
	}
	l.onSelect(l.visible[id].file)
}

// CreateRenderer renders the backing list.
func (l *FileList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(l.list)
}

// SetFiles replaces the file set and resets the view to show all files in
// their original order.
func (l *FileList) SetFiles(files []*diff.File) {
	l.files = files
	l.SetFilter("")
}

// setPalette restyles the list to the given palette and refreshes.
func (l *FileList) setPalette(pal palette) {
	l.palette = pal
	l.list.Refresh()
}

// SetTheme restyles the list to the given theme by deriving its palette.
func (l *FileList) SetTheme(thm *theme.Theme) {
	pal := thm.Palette()
	l.setPalette(paletteFrom(&pal))
}

// SetFilter narrows the visible files by fuzzy-matching query against each
// path. An empty query restores every file in original order; otherwise files
// are shown in descending fuzzy-score order with matched runes recorded for
// emphasis.
func (l *FileList) SetFilter(query string) {
	l.visible = filterFiles(l.files, query)
	l.list.UnselectAll()
	l.list.Refresh()
	l.list.ScrollToTop()
}

// Select selects the visible row at index, invoking the onSelect callback. An
// out-of-range index is ignored.
func (l *FileList) Select(index int) {
	if index < 0 || index >= len(l.visible) {
		return
	}
	l.list.Select(index)
}

// VisiblePaths returns the paths currently shown, in display order. It exists
// chiefly so tests can assert filtering and ordering behavior.
func (l *FileList) VisiblePaths() []string {
	paths := make([]string, len(l.visible))
	for i := range l.visible {
		paths[i] = l.visible[i].file.Path
	}

	return paths
}

// statusGlyph maps a file status to its single-character marker.
func statusGlyph(status diff.Status) string {
	switch status {
	case diff.StatusAdded:
		return "A"
	case diff.StatusDeleted:
		return "D"
	case diff.StatusModified:
		return "M"
	case diff.StatusRenamed:
		return "R"
	case diff.StatusUntracked:
		return "?"
	default:
		return "M"
	}
}

// statusColor tints the status glyph: added/untracked green, deleted red,
// otherwise the accent color.
func statusColor(pal palette, status diff.Status) color.NRGBA {
	switch status {
	case diff.StatusAdded, diff.StatusUntracked:
		return pal.addEmph
	case diff.StatusDeleted:
		return pal.delEmph
	case diff.StatusModified, diff.StatusRenamed:
		return pal.accent
	default:
		return pal.accent
	}
}

// countsLabels formats the +adds and -dels labels for a file.
func countsLabels(file *diff.File) (adds, dels string) {
	return "+" + strconv.Itoa(file.Added), "-" + strconv.Itoa(file.Deleted)
}
