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
	// scanBarCap is the fill ceiling the bar eases toward while the scan runs;
	// since the scan reports no real progress, the fill is a decelerating
	// time-estimate that never reaches the end until Complete snaps it to 1.
	scanBarCap = 0.9
	// scanBarEase is the fraction of the remaining distance covered each tick, so
	// the bar fills quickly then slows as it approaches the cap.
	scanBarEase = 0.04
	scanBarTick = 40 * time.Millisecond
)

// ScanBar is a determinate progress bar for the working-tree scan. The scan has
// no real progress to report, so the fill eases toward scanBarCap on a ticker
// (fast then slow, like a browser loader) and Complete snaps it to 100% when the
// scan finishes. It animates via Refresh rather than fyne.Animation, so it works
// under the no_animations build tag.
type ScanBar struct {
	widget.BaseWidget

	mu   sync.Mutex
	pos  float32
	stop chan struct{}
}

// NewScanBar builds a stopped, empty progress bar.
func NewScanBar() *ScanBar {
	bar := &ScanBar{}
	bar.ExtendBaseWidget(bar)

	return bar
}

// Start begins filling on a background ticker. Call it on the UI goroutine.
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

// Complete stops the fill and snaps the bar to 100%. Call it on the UI goroutine.
func (b *ScanBar) Complete() {
	if b.stop != nil {
		close(b.stop)
		b.stop = nil
	}
	b.mu.Lock()
	b.pos = 1
	b.mu.Unlock()
	b.Refresh()
}

// advance eases the fill toward the cap by a fraction of the remaining distance.
func (b *ScanBar) advance() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pos += (scanBarCap - b.pos) * scanBarEase
}

// fraction is the current fill in [0,1].
func (b *ScanBar) fraction() float32 {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.pos
}

// CreateRenderer wires the track and the left-anchored fill.
func (b *ScanBar) CreateRenderer() fyne.WidgetRenderer {
	track := canvas.NewRectangle(color.Transparent)
	track.CornerRadius = scanBarHeight / 2
	fill := canvas.NewRectangle(color.Transparent)
	fill.CornerRadius = scanBarHeight / 2

	return &scanBarRenderer{bar: b, track: track, fill: fill}
}

// scanBarRenderer paints the track and a fill that grows from the left, pulling
// colors from the active Fyne theme so the bar matches the app palette.
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

	r.fill.Resize(fyne.NewSize(size.Width*r.bar.fraction(), size.Height))
	r.fill.Move(fyne.NewPos(0, 0))
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
