package ui

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/samber/lo"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/icons"
	"github.com/omarluq/diffdiff/internal/theme"
)

// scanningNotice is shown in the file panel while the working-tree scan runs.
const scanningNotice = "Scanning…"

// FileList is the changed-files panel: a fuzzy-filterable view of the files in a
// diff, shown either flat (one row per full path) or as a nested directory tree.
// Each file shows a status glyph, +adds/-dels counts, and its name with the
// basename emphasized; under an active filter the matched runes are accented.
// Both views are virtualized and kept in sync; SetTree swaps between them.
// Selecting a file invokes the onSelect callback.
type FileList struct {
	widget.BaseWidget

	onSelect func(*diff.File)
	files    []*diff.File
	visible  []fileEntry
	model    treeModel
	palette  palette
	nested   bool
	list     *widget.List
	tree     *widget.Tree
	holder   *fyne.Container
	// scanning is a centered notice shown in place of the views while the
	// working-tree scan runs (go-git status can take a moment on huge worktrees).
	scanning *canvas.Text
}

// fileEntry is a file made visible by the current filter, paired with the
// fuzzy-matched rune positions (nil when unfiltered) used for emphasis.
type fileEntry struct {
	file    *diff.File
	matched map[int]bool
}

// NewFileList builds an empty file list; onSelect may be nil. The flat list is
// shown initially with the nested tree hidden behind it.
func NewFileList(onSelect func(*diff.File)) *FileList {
	list := &FileList{
		BaseWidget: widget.BaseWidget{},
		onSelect:   onSelect,
		files:      nil,
		visible:    nil,
		model:      treeModel{children: nil, leaves: nil},
		palette:    palette{},
		nested:     false,
		list:       nil,
		tree:       nil,
		holder:     nil,
		scanning:   nil,
	}
	list.ExtendBaseWidget(list)
	list.buildList()
	list.buildTree()

	list.scanning = canvas.NewText(scanningNotice, color.NRGBA{})
	list.scanning.Alignment = fyne.TextAlignCenter
	list.scanning.Hide()

	list.holder = container.NewStack(list.list, list.tree, container.NewCenter(list.scanning))
	list.tree.Hide()

	return list
}

// SetScanning shows or hides the "Scanning…" notice in place of the file views.
// While scanning, both views are hidden so the panel reads clearly as busy;
// SetFiles ends the scanning state when results arrive.
func (l *FileList) SetScanning(scanning bool) {
	if scanning {
		l.list.Hide()
		l.tree.Hide()
		l.scanning.Show()
		l.scanning.Refresh()

		return
	}

	l.scanning.Hide()
	if l.nested {
		l.tree.Show()
	} else {
		l.list.Show()
	}
}

// buildList wires the recycling flat list and its select handler once.
func (l *FileList) buildList() {
	l.list = widget.NewList(
		func() int { return len(l.visible) },
		func() fyne.CanvasObject { return newFileRow(l.palette, false) },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row, ok := obj.(*fileRow)
			if !ok || id < 0 || id >= len(l.visible) {
				return
			}
			row.set(l.visible[id], l.palette)
		},
	)
	// Hide inter-row separators: each row is exactly one glyph tall, so a
	// separator line lands on the underscore (the lowest glyph) and swallows it.
	l.list.HideSeparators = true
	l.list.OnSelected = l.handleSelect
}

// buildTree wires the recycling nested tree once. Its node funcs read the
// directory model rebuilt on every filter change; leaves reuse fileRow in
// basename-only mode while directories use branchRow.
func (l *FileList) buildTree() {
	l.tree = widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID { return l.model.children[uid] },
		func(uid widget.TreeNodeID) bool { return l.model.isBranch(uid) },
		func(branch bool) fyne.CanvasObject {
			if branch {
				return newBranchRow(l.palette)
			}

			return newFileRow(l.palette, true)
		},
		l.updateNode,
	)
	l.tree.HideSeparators = true
	l.tree.OnSelected = l.handleTreeSelect
}

// updateNode binds a tree node to its model entry: a directory name for a
// branch, or the file in basename-only mode for a leaf.
func (l *FileList) updateNode(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
	if branch {
		if row, ok := obj.(*branchRow); ok {
			label := l.model.label(uid)
			row.set(label, icons.FolderFor(label, l.tree.IsBranchOpen(uid), l.palette.dark), l.palette)
		}

		return
	}
	if row, ok := obj.(*fileRow); ok {
		if entry, ok := l.model.leaves[uid]; ok {
			row.set(entry, l.palette)
		}
	}
}

// handleSelect forwards a flat-list row selection to onSelect, ignoring
// out-of-range ids.
func (l *FileList) handleSelect(id widget.ListItemID) {
	if l.onSelect == nil || id < 0 || id >= len(l.visible) {
		return
	}
	l.onSelect(l.visible[id].file)
}

// handleTreeSelect drives a tree-node tap. Because the disclosure caret is
// hidden, tapping a directory row toggles it open/closed (then clears the
// selection so the same row can be tapped again); tapping a file selects it.
func (l *FileList) handleTreeSelect(uid widget.TreeNodeID) {
	if l.model.isBranch(uid) {
		l.tree.ToggleBranch(uid)
		l.tree.Unselect(uid)

		return
	}
	if l.onSelect == nil {
		return
	}
	if entry, ok := l.model.leaves[uid]; ok {
		l.onSelect(entry.file)
	}
}

// CreateRenderer renders the stacked flat-list/tree holder.
func (l *FileList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(l.holder)
}

// SetTree switches between the flat list and the nested directory tree.
func (l *FileList) SetTree(nested bool) {
	if l.nested == nested {
		return
	}
	l.nested = nested
	if nested {
		l.list.Hide()
		l.tree.Show()
		// The tree was not refreshed while hidden during streaming; bring it up to
		// date now so any counts that arrived meanwhile are shown.
		l.tree.Refresh()
	} else {
		l.tree.Hide()
		l.list.Show()
		l.list.Refresh()
	}
	l.holder.Refresh()
}

// SetFiles replaces the file set and resets the view to show all files in
// their original order.
func (l *FileList) SetFiles(files []*diff.File) {
	l.files = files
	l.SetScanning(false)
	l.SetFilter("")
}

// RefreshFile repaints just the row for file in place — its streamed counts and
// the blob-corrected status glyph — without disturbing the selection or scroll
// position. Unlike SetFilter's full refresh, RefreshItem re-runs only the one
// matching row's update, which is what keeps background-loaded results from
// flickering or jumping the list. The tree leaf is keyed by the file's path.
func (l *FileList) RefreshFile(file *diff.File) {
	// Only the visible view is refreshed as results stream in; the hidden view is
	// brought up to date once, on toggle, by SetTree. Refreshing both per file
	// doubled the per-row render work (and icon work) during the sweep.
	if l.nested {
		l.tree.RefreshItem(file.Path)

		return
	}
	for index := range l.visible {
		if l.visible[index].file == file {
			l.list.RefreshItem(index)

			break
		}
	}
}

// setPalette restyles both views to the given palette and refreshes them.
func (l *FileList) setPalette(pal palette) {
	l.palette = pal
	l.scanning.Color = pal.muted
	l.scanning.Refresh()
	l.list.Refresh()
	l.tree.Refresh()
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
	l.model = buildTreeModel(l.visible)

	l.list.UnselectAll()
	l.list.Refresh()
	l.list.ScrollToTop()

	l.tree.UnselectAll()
	l.tree.Refresh()
	l.tree.OpenAllBranches()
	l.tree.ScrollToTop()
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
	return lo.Map(l.visible, func(entry fileEntry, _ int) string { return entry.file.Path })
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

// countsLabels formats the +adds and -dels labels for a file. While the file's
// diff is still being built (counts not yet known) it returns blanks, so the row
// shows its status glyph immediately without a misleading "+0/-0"; the counts
// appear once the file streams in.
func countsLabels(file *diff.File) (adds, dels string) {
	if !file.HasCounts() {
		return "", ""
	}

	return "+" + strconv.Itoa(file.Added), "-" + strconv.Itoa(file.Deleted)
}
