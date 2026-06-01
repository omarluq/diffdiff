package ui

import (
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	scanBarMinWidth = 260
	scanBarHeight   = 8
	// scanBarFillFrac is the moving segment's width as a fraction of the track.
	scanBarFillFrac = 0.32
	// scanBarStep advances the sweep each tick; scanBarTick is the frame interval.
	scanBarStep = 0.02
	scanBarTick = 16 * time.Millisecond
)

// ScanBar is an indeterminate progress bar: a fill segment sweeps back and forth
// along a track. It is deliberately indeterminate because the working-tree scan
// (go-git status) reports no progress, so there is no honest percentage to show.
// The build disables fyne.Animation (no_animations), which would freeze
// ProgressBarInfinite, so this animates itself with a ticker and Refresh — a
// plain repaint, unaffected by the tag. Start begins the motion; Stop ends it.
type ScanBar struct {
	widget.BaseWidget

	mu      sync.Mutex
	pos     float32
	forward bool
	stop    chan struct{}
}

// NewScanBar builds a stopped indeterminate bar.
func NewScanBar() *ScanBar {
	bar := &ScanBar{forward: true}
	bar.ExtendBaseWidget(bar)

	return bar
}

// Start begins the sweep on a background ticker. Call it on the UI goroutine.
func (b *ScanBar) Start() {
	stop := make(chan struct{})
	b.stop = stop
	go func() {
		ticker := time.NewTicker(scanBarTick)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				b.advance()
				fyne.Do(b.Refresh)
			}
		}
	}()
}

// Stop ends the sweep. Call it on the UI goroutine.
func (b *ScanBar) Stop() {
	if b.stop != nil {
		close(b.stop)
		b.stop = nil
	}
}

// advance steps the sweep position, bouncing at each end.
func (b *ScanBar) advance() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.forward {
		b.pos += scanBarStep
		if b.pos >= 1 {
			b.pos, b.forward = 1, false
		}

		return
	}
	b.pos -= scanBarStep
	if b.pos <= 0 {
		b.pos, b.forward = 0, true
	}
}

// fraction is the current sweep position in [0,1].
func (b *ScanBar) fraction() float32 {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.pos
}

// CreateRenderer wires the track and the sweeping fill.
func (b *ScanBar) CreateRenderer() fyne.WidgetRenderer {
	track := canvas.NewRectangle(color.Transparent)
	track.CornerRadius = scanBarHeight / 2
	fill := canvas.NewRectangle(color.Transparent)
	fill.CornerRadius = scanBarHeight / 2

	return &scanBarRenderer{bar: b, track: track, fill: fill}
}

// scanBarRenderer paints the track and the sweeping fill, pulling colors from the
// active Fyne theme so the bar matches the app palette.
type scanBarRenderer struct {
	bar   *ScanBar
	track *canvas.Rectangle
	fill  *canvas.Rectangle
}

func (r *scanBarRenderer) themeColors() (trackColor, fillColor color.Color) {
	app := fyne.CurrentApp()
	theme := app.Settings().Theme()
	variant := app.Settings().ThemeVariant()

	return theme.Color(fynetheme.ColorNameInputBackground, variant),
		theme.Color(fynetheme.ColorNamePrimary, variant)
}

func (r *scanBarRenderer) Layout(size fyne.Size) {
	r.track.Resize(size)
	r.track.Move(fyne.NewPos(0, 0))

	fillWidth := size.Width * scanBarFillFrac
	x := r.bar.fraction() * (size.Width - fillWidth)
	r.fill.Resize(fyne.NewSize(fillWidth, size.Height))
	r.fill.Move(fyne.NewPos(x, 0))
}

func (r *scanBarRenderer) Refresh() {
	r.track.FillColor, r.fill.FillColor = r.themeColors()
	r.Layout(r.bar.Size())
	canvas.Refresh(r.bar)
}

func (r *scanBarRenderer) MinSize() fyne.Size {
	return fyne.NewSize(scanBarMinWidth, scanBarHeight)
}

func (r *scanBarRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.fill}
}

func (r *scanBarRenderer) Destroy() {}
