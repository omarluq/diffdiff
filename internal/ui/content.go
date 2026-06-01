package ui

import (
	"os"
	"strings"

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

// Popup sizing for the project and option (theme/font) pickers.
const (
	projectMenuWidth   float32 = 360
	pickerWidth        float32 = 220
	pickerVisibleItems         = 5
)

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
	statusBar     *statusBar
	projectButton *widget.Button
	menuButton    *widget.Button
	themeButton   *widget.Button
	fontButton    *widget.Button
	active        *theme.Theme
	activeFont    string
	current       *diff.File
	recent        []string
	splitView     bool
	nestedTree    bool
	onOpenProject func(string)
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
		statusBar:     newStatusBar(),
		projectButton: nil,
		menuButton:    nil,
		themeButton:   nil,
		fontButton:    nil,
		active:        themes.Default(),
		activeFont:    fonts.DefaultName(),
		current:       nil,
		recent:        nil,
		splitView:     false,
		nestedTree:    false,
		onOpenProject: nil,
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
// a project picker, a hamburger menu, the file filter, then the theme and font
// selectors.
func (c *Content) assemble() fyne.CanvasObject {
	c.projectButton = widget.NewButtonWithIcon("", fynetheme.FolderOpenIcon(), c.showProjectMenu)
	c.projectButton.Importance = widget.LowImportance

	c.menuButton = widget.NewButtonWithIcon("", fynetheme.MenuIcon(), c.showMenu)
	c.menuButton.Importance = widget.LowImportance

	filter := widget.NewEntry()
	filter.SetPlaceHolder("Filter files…")
	filter.OnChanged = c.fileList.SetFilter

	c.themeButton = widget.NewButton(c.active.Name(), func() {
		c.showOptionsMenu(c.themeButton, c.themes.Names(), c.active.Name(), c.SetTheme)
	})
	c.fontButton = widget.NewButton(c.activeFont, func() {
		c.showOptionsMenu(c.fontButton, c.fonts.Names(), c.activeFont, c.SetFont)
	})

	left := container.NewHBox(c.projectButton, c.menuButton)
	selectors := container.NewHBox(c.themeButton, c.fontButton)
	toolbar := container.NewBorder(nil, nil, left, selectors, filter)

	split := container.NewHSplit(c.fileList, c.diffView)
	split.SetOffset(splitOffset)

	return container.NewBorder(toolbar, c.statusBar.object(), nil, nil, split)
}

// showProjectMenu pops up the project picker beneath its button: a path input
// (submit to open) above buttons for the recently opened projects.
func (c *Content) showProjectMenu() {
	canvas := fyne.CurrentApp().Driver().CanvasForObject(c.projectButton)
	if canvas == nil {
		return
	}

	var popUp *widget.PopUp
	open := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		popUp.Hide()
		if c.onOpenProject != nil {
			c.onOpenProject(path)
		}
	}

	entry := widget.NewEntry()
	entry.SetPlaceHolder("Open repository path…")
	entry.OnSubmitted = open

	items := []fyne.CanvasObject{entry}
	for index, path := range c.recent {
		recent := path
		button := widget.NewButton(displayPath(recent), func() { open(recent) })
		button.Alignment = widget.ButtonAlignLeading
		button.Importance = widget.LowImportance
		if index == 0 { // the most-recent entry is the currently open project
			button.Icon = fynetheme.ConfirmIcon()
		}
		items = append(items, button)
	}

	popUp = widget.NewPopUp(container.NewVBox(items...), canvas)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(c.projectButton)
	popUp.Resize(fyne.NewSize(projectMenuWidth, popUp.MinSize().Height))
	popUp.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+c.projectButton.Size().Height))
	canvas.Focus(entry)
}

// showOptionsMenu pops up a scrollable list of options beneath anchor. The
// active option carries a leading check mark; choosing one invokes onPick and
// dismisses the popup.
func (c *Content) showOptionsMenu(anchor fyne.CanvasObject, options []string, active string, onPick func(string)) {
	canvas := fyne.CurrentApp().Driver().CanvasForObject(anchor)
	if canvas == nil {
		return
	}

	var popUp *widget.PopUp
	activeIndex := -1
	items := make([]fyne.CanvasObject, 0, len(options))
	for index, option := range options {
		choice := option
		button := widget.NewButton(choice, func() {
			popUp.Hide()
			onPick(choice)
		})
		button.Alignment = widget.ButtonAlignLeading
		button.Importance = widget.LowImportance
		if choice == active {
			activeIndex = index
			button.Icon = fynetheme.ConfirmIcon()
		}
		items = append(items, button)
	}

	box := container.NewVBox(items...)
	full := box.MinSize().Height
	viewport := full
	if len(options) > pickerVisibleItems {
		viewport = full * float32(pickerVisibleItems) / float32(len(options))
	}

	scroll := container.NewVScroll(box)
	scroll.SetMinSize(fyne.NewSize(pickerWidth, viewport))

	up := widget.NewIcon(fynetheme.MenuDropUpIcon())
	down := widget.NewIcon(fynetheme.MenuDropDownIcon())
	up.Hide()
	down.Hide()
	carets := container.NewBorder(container.NewCenter(up), container.NewCenter(down), nil, nil)

	maxOffset := full - viewport
	syncCarets := func(offset float32) {
		setVisible(up, offset > 1)
		setVisible(down, offset < maxOffset-1)
	}
	scroll.OnScrolled = func(pos fyne.Position) { syncCarets(pos.Y) }

	popUp = widget.NewPopUp(container.NewStack(scroll, carets), canvas)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	popUp.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+anchor.Size().Height))
	scrollToActive(scroll, box, len(options), activeIndex, viewport)
	syncCarets(scroll.Offset.Y)
}

// scrollToActive centers the active option in the picker's viewport so its
// indicator is visible the moment the menu opens.
func scrollToActive(scroll *container.Scroll, box fyne.CanvasObject, count, active int, viewport float32) {
	if active <= 0 || count <= 1 {
		return
	}

	itemHeight := box.MinSize().Height / float32(count)
	offset := float32(active)*itemHeight - viewport/2 + itemHeight/2
	if offset < 0 {
		offset = 0
	}

	scroll.Offset = fyne.NewPos(0, offset)
	scroll.Refresh()
}

// setVisible shows or hides a canvas object.
func setVisible(obj fyne.CanvasObject, visible bool) {
	if visible {
		obj.Show()
	} else {
		obj.Hide()
	}
}

// OnOpenProject registers a listener invoked when the user opens a project,
// receiving the entered or selected path.
func (c *Content) OnOpenProject(fn func(string)) {
	c.onOpenProject = fn
}

// SetRecentProjects replaces the recent-project list shown in the picker.
func (c *Content) SetRecentProjects(paths []string) {
	c.recent = make([]string, len(paths))
	copy(c.recent, paths)
}

// displayPath abbreviates the user's home directory to "~" for compact display.
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && (path == home || strings.HasPrefix(path, home+string(os.PathSeparator))) {
		return "~" + path[len(home):]
	}

	return path
}

// showMenu pops up the hamburger menu beneath its button. The "Split view" and
// "Nested tree" items are checkable and reflect the current layout each time the
// menu opens.
func (c *Content) showMenu() {
	split := fyne.NewMenuItem("Split view", c.toggleSplitView)
	split.Checked = c.splitView
	tree := fyne.NewMenuItem("Nested tree", c.toggleTreeView)
	tree.Checked = c.nestedTree
	menu := fyne.NewMenu("", split, tree)

	canvas := fyne.CurrentApp().Driver().CanvasForObject(c.menuButton)
	if canvas == nil {
		return
	}

	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(c.menuButton)
	popUp := widget.NewPopUpMenu(menu, canvas)
	popUp.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+c.menuButton.Size().Height))
}

// toggleSplitView flips between the unified (stacked) and split (side-by-side)
// diff layouts, re-rendering the current file in the chosen layout.
func (c *Content) toggleSplitView() {
	c.splitView = !c.splitView
	c.diffView.SetSplit(c.splitView)
}

// toggleTreeView flips the file panel between the flat path list and the nested
// directory tree.
func (c *Content) toggleTreeView() {
	c.nestedTree = !c.nestedTree
	c.fileList.SetTree(c.nestedTree)
}

// SetFiles loads the file set into the file list, updates the status-bar
// summary, and opens the first file so the diff view is populated immediately
// rather than blank until the first click.
func (c *Content) SetFiles(files []*diff.File) {
	c.fileList.SetFiles(files)

	added, deleted := 0, 0
	for _, file := range files {
		added += file.Added
		deleted += file.Deleted
	}
	c.statusBar.setSummary(len(files), added, deleted)

	if len(files) > 0 {
		c.handleSelect(files[0])
	}
}

// SetGitInfo updates the status bar with the active repository's branch and
// short HEAD hash.
func (c *Content) SetGitInfo(branch, head string) {
	c.statusBar.setGit(branch, head)
}

// SetTheme switches the active theme by display name. An unknown name is ignored.
func (c *Content) SetTheme(name string) {
	thm, ok := c.themes.Get(name)
	if !ok {
		return
	}

	c.active = thm
	if c.themeButton != nil {
		c.themeButton.SetText(name)
	}
	c.restyle()
}

// SetFont switches the active monospace font by display name. An unknown name is
// ignored.
func (c *Content) SetFont(name string) {
	if _, ok := c.fonts.Get(name); !ok {
		return
	}

	c.activeFont = name
	if c.fontButton != nil {
		c.fontButton.SetText(name)
	}
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

	pal := c.active.Palette()
	c.statusBar.setPalette(paletteFrom(&pal))
	c.fileList.SetTheme(c.active)
	c.diffView.SetFile(c.current, c.active)
}

// ActiveTheme returns the theme currently applied.
func (c *Content) ActiveTheme() *theme.Theme {
	return c.active
}
