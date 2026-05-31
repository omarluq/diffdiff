package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/highlight"
	"github.com/omarluq/diffdiff/internal/theme"
)

// splitOffset is the fraction of width given to the file list; the diff view
// takes the remainder.
const splitOffset = 0.28

// Content is the top-level diff browser: a toolbar (filter entry + theme
// selector) over a horizontal split of the file list and the diff view. It owns
// the active theme and keeps both child widgets styled to it.
type Content struct {
	registry      *theme.Registry
	highlighter   *highlight.Highlighter
	fileList      *FileList
	diffView      *DiffView
	active        *theme.Theme
	current       *diff.File
	onThemeChange func(*theme.Theme)
}

// NewContent assembles the diff browser, returning its root canvas object and a
// Content handle for driving it. The initial theme is the registry default.
func NewContent(reg *theme.Registry, highlighter *highlight.Highlighter) (fyne.CanvasObject, *Content) {
	content := &Content{
		registry:      reg,
		highlighter:   highlighter,
		fileList:      nil,
		diffView:      NewDiffView(highlighter),
		active:        reg.Default(),
		current:       nil,
		onThemeChange: nil,
	}
	content.fileList = NewFileList(content.handleSelect)

	content.fileList.SetTheme(content.active)
	root := content.assemble()

	return root, content
}

// handleSelect loads the chosen file into the diff view under the active theme,
// remembering it so a later theme switch can restyle it in place.
func (c *Content) handleSelect(file *diff.File) {
	c.current = file
	c.diffView.SetFile(file, c.active)
}

// assemble builds the toolbar and split layout.
func (c *Content) assemble() fyne.CanvasObject {
	filter := widget.NewEntry()
	filter.SetPlaceHolder("Filter files…")
	filter.OnChanged = c.fileList.SetFilter

	selector := widget.NewSelect(c.registry.Names(), c.SetTheme)
	selector.SetSelected(c.active.Name())

	toolbar := container.NewBorder(nil, nil, nil, selector, filter)

	split := container.NewHSplit(c.fileList, c.diffView)
	split.SetOffset(splitOffset)

	return container.NewBorder(toolbar, nil, nil, nil, split)
}

// SetFiles loads the file set into the file list and opens the first file so the
// diff view is populated immediately rather than blank until the first click.
func (c *Content) SetFiles(files []*diff.File) {
	c.fileList.SetFiles(files)
	if len(files) > 0 {
		c.handleSelect(files[0])
	}
}

// SetTheme switches the active theme by display name, restyling the file list
// and the open diff and notifying any OnThemeChange listener. An unknown name is
// ignored.
func (c *Content) SetTheme(name string) {
	thm, ok := c.registry.Get(name)
	if !ok {
		return
	}

	c.active = thm
	c.fileList.SetTheme(thm)
	c.diffView.SetFile(c.current, thm)

	if c.onThemeChange != nil {
		c.onThemeChange(thm)
	}
}

// OnThemeChange registers a listener invoked whenever the active theme changes,
// letting the host (e.g. the window) keep its chrome in step with the diff.
func (c *Content) OnThemeChange(fn func(*theme.Theme)) {
	c.onThemeChange = fn
}

// ActiveTheme returns the theme currently applied.
func (c *Content) ActiveTheme() *theme.Theme {
	return c.active
}
