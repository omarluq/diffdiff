package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/highlight"
	"github.com/omarluq/diffdiff/internal/theme"
)

// splitOffset is the fraction of width given to the file list; the diff view
// takes the remainder.
const splitOffset = 0.28

// Content is the top-level diff browser: a toolbar (hamburger menu + file filter
// + theme and font selectors) over a horizontal split of the file list and the
// diff view. It owns the active theme and font and keeps both child widgets and
// the application chrome styled to them.
type Content struct {
	themes        *theme.Registry
	fonts         *theme.FontRegistry
	highlighter   *highlight.Highlighter
	fileList      *FileList
	diffView      *DiffView
	menuButton    *widget.Button
	active        *theme.Theme
	activeFont    string
	current       *diff.File
	showIgnored   bool
	onShowIgnored func(bool)
}

// NewContent assembles the diff browser, returning its root canvas object and a
// Content handle for driving it. The initial theme and font are the registry
// defaults.
func NewContent(
	themes *theme.Registry, fonts *theme.FontRegistry, highlighter *highlight.Highlighter,
) (fyne.CanvasObject, *Content) {
	content := &Content{
		themes:        themes,
		fonts:         fonts,
		highlighter:   highlighter,
		fileList:      nil,
		diffView:      NewDiffView(highlighter),
		menuButton:    nil,
		active:        themes.Default(),
		activeFont:    fonts.DefaultName(),
		current:       nil,
		showIgnored:   false,
		onShowIgnored: nil,
	}
	content.fileList = NewFileList(content.handleSelect)
	root := content.assemble()
	content.restyle()

	return root, content
}

// handleSelect loads the chosen file into the diff view under the active theme,
// remembering it so a later theme or font switch can restyle it in place.
func (c *Content) handleSelect(file *diff.File) {
	c.current = file
	c.diffView.SetFile(file, c.active)
}

// assemble builds the toolbar and split layout. The toolbar runs left to right:
// a hamburger menu, the file filter, then the theme and font selectors.
func (c *Content) assemble() fyne.CanvasObject {
	c.menuButton = widget.NewButtonWithIcon("", fynetheme.MenuIcon(), c.showMenu)
	c.menuButton.Importance = widget.LowImportance

	filter := widget.NewEntry()
	filter.SetPlaceHolder("Filter files…")
	filter.OnChanged = c.fileList.SetFilter

	themeSelect := widget.NewSelect(c.themes.Names(), c.SetTheme)
	themeSelect.SetSelected(c.active.Name())

	fontSelect := widget.NewSelect(c.fonts.Names(), c.SetFont)
	fontSelect.SetSelected(c.activeFont)

	selectors := container.NewHBox(themeSelect, fontSelect)
	toolbar := container.NewBorder(nil, nil, c.menuButton, selectors, filter)

	split := container.NewHSplit(c.fileList, c.diffView)
	split.SetOffset(splitOffset)

	return container.NewBorder(toolbar, nil, nil, nil, split)
}

// showMenu pops up the hamburger menu beneath its button. The "Show ignored"
// item is checkable and reflects the current state each time the menu opens.
func (c *Content) showMenu() {
	item := fyne.NewMenuItem("Show ignored", c.toggleShowIgnored)
	item.Checked = c.showIgnored
	menu := fyne.NewMenu("", item)

	canvas := fyne.CurrentApp().Driver().CanvasForObject(c.menuButton)
	if canvas == nil {
		return
	}

	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(c.menuButton)
	popUp := widget.NewPopUpMenu(menu, canvas)
	popUp.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+c.menuButton.Size().Height))
}

// toggleShowIgnored flips the show-ignored state and notifies the listener so
// the host can re-scan the repository.
func (c *Content) toggleShowIgnored() {
	c.showIgnored = !c.showIgnored
	if c.onShowIgnored != nil {
		c.onShowIgnored(c.showIgnored)
	}
}

// OnShowIgnored registers a listener invoked when the "show ignored" option
// toggles, receiving the new state.
func (c *Content) OnShowIgnored(fn func(bool)) {
	c.onShowIgnored = fn
}

// SetFiles loads the file set into the file list and opens the first file so the
// diff view is populated immediately rather than blank until the first click.
func (c *Content) SetFiles(files []*diff.File) {
	c.fileList.SetFiles(files)
	if len(files) > 0 {
		c.handleSelect(files[0])
	}
}

// SetTheme switches the active theme by display name. An unknown name is ignored.
func (c *Content) SetTheme(name string) {
	thm, ok := c.themes.Get(name)
	if !ok {
		return
	}

	c.active = thm
	c.restyle()
}

// SetFont switches the active monospace font by display name. An unknown name is
// ignored.
func (c *Content) SetFont(name string) {
	if _, ok := c.fonts.Get(name); !ok {
		return
	}

	c.activeFont = name
	c.restyle()
}

// restyle reapplies the active theme and font to the application chrome and both
// panels. The application theme is set first so the monospace metrics
// (glyph advance and line height) are re-measured against the active font before
// the file list and diff view relayout.
func (c *Content) restyle() {
	if app := fyne.CurrentApp(); app != nil {
		if font, ok := c.fonts.Get(c.activeFont); ok {
			app.Settings().SetTheme(theme.NewFyneTheme(c.active, font))
		}
	}

	c.fileList.SetTheme(c.active)
	c.diffView.SetFile(c.current, c.active)
}

// ActiveTheme returns the theme currently applied.
func (c *Content) ActiveTheme() *theme.Theme {
	return c.active
}
