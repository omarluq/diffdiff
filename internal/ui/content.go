package ui

import (
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/samber/lo"

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
	pickerVisibleItems         = 10
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
	// Streaming summary state: counts arrive per file as their diffs build in the
	// background. fileCount is fixed at SetFiles; addedSum/deletedSum accumulate;
	// counted guards against a file being tallied twice (the sweep and an
	// on-demand load can both report the same file ready).
	fileCount  int
	addedSum   int
	deletedSum int
	counted    map[*diff.File]bool
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
		fileCount:     0,
		addedSum:      0,
		deletedSum:    0,
		counted:       nil,
	}
	content.fileList = NewFileList(content.handleSelect)
	root := content.assemble()
	content.restylePalette()

	return root, content
}

// handleSelect loads the chosen file into the diff view under the active theme,
// remembering it so a later theme or font switch can restyle it in place. An
// unbuilt file shows a loading placeholder; the background sweep's FileReady
// swaps in the rendered diff once it lands.
func (c *Content) handleSelect(file *diff.File) {
	c.current = file
	c.diffView.SetFile(file, c.active)
}

// FileReady streams a freshly built file's results into the UI: it repaints the
// file's row (counts + corrected status glyph) in place, advances the running
// status-bar totals once, and — if the file is the one currently shown — swaps
// the loading placeholder for the rendered diff. It is idempotent (a file may be
// reported ready by both the sweep and an on-demand load) and must run on the UI
// goroutine; callers marshal it via fyne.Do.
func (c *Content) FileReady(file *diff.File) {
	if file == nil {
		return
	}
	c.fileList.RefreshFile(file)

	if c.counted == nil {
		c.counted = map[*diff.File]bool{}
	}
	if !c.counted[file] {
		c.counted[file] = true
		c.addedSum += file.Added
		c.deletedSum += file.Deleted
		c.statusBar.setSummary(c.fileCount, c.addedSum, c.deletedSum)
	}

	if file == c.current {
		c.diffView.FileLoaded(file, c.active)
	}
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

	// rebuild repopulates the option buttons for the current filter query. The
	// active option carries a leading check mark; choosing one dismisses the popup.
	box := container.NewVBox()
	rebuild := func(query string) {
		box.RemoveAll()
		for _, option := range fuzzyOptions(options, query) {
			choice := option
			button := widget.NewButton(choice, func() {
				popUp.Hide()
				onPick(choice)
			})
			button.Alignment = widget.ButtonAlignLeading
			button.Importance = widget.LowImportance
			if choice == active {
				button.Icon = fynetheme.ConfirmIcon()
			}
			box.Add(button)
		}
		box.Refresh()
	}
	rebuild("")

	// Fixed viewport: up to pickerVisibleItems rows of the full list, so typing in
	// the filter scrolls within a stable popup rather than resizing it.
	rowHeight := float32(0)
	if len(options) > 0 {
		rowHeight = box.MinSize().Height / float32(len(options))
	}
	viewport := rowHeight * float32(min(len(options), pickerVisibleItems))

	scroll := container.NewVScroll(box)
	scroll.SetMinSize(fyne.NewSize(pickerWidth, viewport))

	filter := widget.NewEntry()
	filter.SetPlaceHolder("Filter…")
	// Reset to the top on every filter change: the menu opens scrolled to the
	// active option, so without this the (top-ranked) matches would render above
	// the parked viewport and the list would look empty.
	filter.OnChanged = func(query string) {
		rebuild(query)
		scroll.ScrollToTop()
	}

	popUp = widget.NewPopUp(container.NewBorder(filter, nil, nil, nil, scroll), canvas)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	popUp.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+anchor.Size().Height))
	scrollToActive(scroll, box, len(options), lo.IndexOf(options, active), viewport)
	canvas.Focus(filter)
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

// SetFiles loads the file set into the file list and opens the first file. The
// files may be unloaded (status-only): their rows show immediately with status
// glyphs while the counts and diffs stream in via FileReady, so the summary
// starts at zero counts and climbs as files build. The selected file's diff is
// shown as a loading placeholder until it is ready.
func (c *Content) SetFiles(files []*diff.File) {
	c.fileList.SetFiles(files)

	c.fileCount = len(files)
	c.addedSum = 0
	c.deletedSum = 0
	c.counted = map[*diff.File]bool{}
	c.statusBar.setSummary(c.fileCount, 0, 0)

	if len(files) > 0 {
		c.handleSelect(files[0])

		return
	}
	c.current = nil
	c.diffView.SetFile(nil, c.active)
}

// Clear empties the panels (file list, diff view, and status summary), used when
// switching repositories so the previous project's content disappears on the
// spot while the new one is scanned.
func (c *Content) Clear() {
	c.SetFiles(nil)
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
	c.restylePalette()
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
	c.relayoutForFont()
}

// applyFyneTheme installs the active theme + font as the Fyne application theme
// so the window chrome and monospace metrics track the current selection.
func (c *Content) applyFyneTheme() {
	if app := fyne.CurrentApp(); app != nil {
		if font, ok := c.fonts.Get(c.activeFont); ok {
			app.Settings().SetTheme(theme.NewFyneTheme(c.active, font))
		}
	}
}

// restylePalette reapplies the active theme's colors to the chrome and both
// panels in place. It is the theme-switch path: the diff view recolors its rows
// (re-highlighting only if the chroma style changed) without re-flattening or
// resetting the scroll position, so a theme switch never flickers.
func (c *Content) restylePalette() {
	c.applyFyneTheme()

	pal := c.active.Palette()
	c.statusBar.setPalette(paletteFrom(&pal))
	c.fileList.SetTheme(c.active)
	c.diffView.Restyle(c.active)
}

// relayoutForFont reapplies the active font: it re-measures the monospace
// metrics (invalidating the cache first) and repositions both panels' rows in
// place, preserving syntax tokens and scroll position. It is the font-switch
// path, distinct from restylePalette because a font change moves cell geometry
// while a theme change does not.
func (c *Content) relayoutForFont() {
	c.applyFyneTheme()
	invalidateMonoMetrics()

	pal := c.active.Palette()
	c.statusBar.setPalette(paletteFrom(&pal))
	c.fileList.SetTheme(c.active)
	c.diffView.Relayout(c.active)
}

// ActiveTheme returns the theme currently applied.
func (c *Content) ActiveTheme() *theme.Theme {
	return c.active
}
