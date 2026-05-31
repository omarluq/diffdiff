package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// statusBarTextSize is the point size for the bottom status bar.
const statusBarTextSize float32 = 12

// statusBar is the bottom bar: repository git details on the left (branch and
// short HEAD) and the changed-files summary on the right (file count and total
// additions/deletions). It is a plain container with colored text, restyled
// from the active palette.
type statusBar struct {
	background *canvas.Rectangle
	git        *canvas.Text
	files      *canvas.Text
	adds       *canvas.Text
	dels       *canvas.Text
	root       *fyne.Container
}

// newStatusBar builds an empty status bar.
func newStatusBar() *statusBar {
	text := func() *canvas.Text {
		txt := canvas.NewText("", color.Gray{Y: 0x88})
		txt.TextSize = statusBarTextSize

		return txt
	}

	bar := &statusBar{
		background: canvas.NewRectangle(color.Transparent),
		git:        text(),
		files:      text(),
		adds:       text(),
		dels:       text(),
		root:       nil,
	}

	left := container.NewHBox(bar.git)
	right := container.NewHBox(bar.files, bar.adds, bar.dels)
	row := container.NewBorder(nil, nil, left, right, nil)
	bar.root = container.NewStack(bar.background, container.NewPadded(row))

	return bar
}

// object returns the status bar's canvas object for placement.
func (b *statusBar) object() fyne.CanvasObject {
	return b.root
}

// setPalette recolors the bar from the active palette.
func (b *statusBar) setPalette(pal palette) {
	b.background.FillColor = pal.surface
	b.git.Color = pal.muted
	b.files.Color = pal.muted
	b.adds.Color = pal.addEmph
	b.dels.Color = pal.delEmph

	b.background.Refresh()
	b.refreshText()
}

// setGit shows the current branch and short HEAD hash.
func (b *statusBar) setGit(branch, head string) {
	switch {
	case branch == "" && head == "":
		b.git.Text = "no commits"
	case branch == "":
		b.git.Text = head // detached HEAD
	default:
		b.git.Text = branch + "  ·  " + head
	}

	b.git.Refresh()
}

// setSummary shows the changed-file count and total additions/deletions.
func (b *statusBar) setSummary(files, added, deleted int) {
	noun := "files"
	if files == 1 {
		noun = "file"
	}

	b.files.Text = fmt.Sprintf("%d %s", files, noun)
	b.adds.Text = fmt.Sprintf("  +%d", added)
	b.dels.Text = fmt.Sprintf("  -%d", deleted)
	b.refreshText()
}

// refreshText repaints the text segments after a content or color change.
func (b *statusBar) refreshText() {
	b.git.Refresh()
	b.files.Refresh()
	b.adds.Refresh()
	b.dels.Refresh()
}
