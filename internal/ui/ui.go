// Package ui assembles the Fyne widgets that render a unified git diff: a
// fuzzy-filterable list of changed files and a virtualized diff view with
// syntax highlighting and intra-line emphasis. It builds only widgets — the
// application and window are created by the command layer — so it never imports
// fyne.io/fyne/v2/app.
package ui

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"github.com/omarluq/diffdiff/internal/theme"
)

// monoText is the reference glyph measured to derive the monospace advance;
// "M" is a conventionally wide character in monospaced fonts.
const monoText = "M"

// diffTextSize is the point size used for diff rows. A modest fixed size keeps
// the dense gutter/sign/content layout legible without depending on theme size.
const diffTextSize float32 = 13

// palette is the subset of theme colors the diff widgets draw with, copied out
// of theme.Palette so rows hold value types rather than chasing a *theme.Theme.
type palette struct {
	background color.NRGBA
	surface    color.NRGBA
	foreground color.NRGBA
	muted      color.NRGBA
	accent     color.NRGBA
	border     color.NRGBA
	addBg      color.NRGBA
	addEmph    color.NRGBA
	delBg      color.NRGBA
	delEmph    color.NRGBA
	// dark reports whether the active theme is dark, used to pick light icon
	// variants on light themes.
	dark bool
}

// paletteFrom projects a theme.Palette into the local palette type. It takes a
// pointer to avoid copying the wide source struct.
func paletteFrom(src *theme.Palette) palette {
	return palette{
		background: src.Background,
		surface:    src.Surface,
		foreground: src.Foreground,
		muted:      src.Muted,
		accent:     src.Accent,
		border:     src.Border,
		addBg:      src.AddBg,
		addEmph:    src.AddEmph,
		delBg:      src.DelBg,
		delEmph:    src.DelEmph,
		dark:       src.Dark,
	}
}

// newMonoText builds a monospace canvas text at the given size, weight, and
// alignment. It is the single constructor shared by every monospace run in the
// UI (diff rows, file-list chrome, path segments) so the text style is defined
// in exactly one place.
func newMonoText(content string, col color.Color, size float32, bold bool, align fyne.TextAlign) *canvas.Text {
	txt := canvas.NewText(content, col)
	setMonoText(txt, content, col, size, bold, false)
	txt.Alignment = align

	return txt
}

// setMonoText reconfigures an existing canvas.Text in place (content, color,
// size, weight) without allocating. Renderers reuse a pool of text objects
// across refreshes and call this to repaint each pooled run, avoiding a fresh
// allocation per token on every scroll frame. Alignment is left untouched since
// it is fixed per call site.
func setMonoText(txt *canvas.Text, content string, col color.Color, size float32, bold, italic bool) {
	txt.Text = content
	txt.Color = col
	txt.TextSize = size
	style := monoStyle()
	style.Bold = bold
	style.Italic = italic
	txt.TextStyle = style
}

// monoMetricsCache memoizes monospace glyph measurements by text size. The
// advance and line height depend only on the size and the registered mono font,
// so the result is stable until the font changes (see invalidateMonoMetrics).
// fyne.MeasureText is comparatively expensive and was previously re-run on every
// row build and file load.
var (
	monoMetricsMu sync.Mutex
	monoAdvance   = map[float32]float32{}
	monoHeight    = map[float32]float32{}
)

// measureAdvance returns the pixel width of a single monospace glyph at size,
// the basis for all column math in the diff view. Results are cached per size.
func measureAdvance(size float32) float32 {
	monoMetricsMu.Lock()
	defer monoMetricsMu.Unlock()
	if w, ok := monoAdvance[size]; ok {
		return w
	}
	w := fyne.MeasureText(monoText, size, monoStyle()).Width
	monoAdvance[size] = w

	return w
}

// lineHeight is the row height for monospace text at size. Results are cached
// per size.
func lineHeight(size float32) float32 {
	monoMetricsMu.Lock()
	defer monoMetricsMu.Unlock()
	if h, ok := monoHeight[size]; ok {
		return h
	}
	h := fyne.MeasureText(monoText, size, monoStyle()).Height
	monoHeight[size] = h

	return h
}

// invalidateMonoMetrics clears the cached glyph measurements. It must be called
// when the active monospace font changes, since the advance and line height are
// font-dependent; the next measure re-reads them from the new font.
func invalidateMonoMetrics() {
	monoMetricsMu.Lock()
	defer monoMetricsMu.Unlock()
	monoAdvance = map[float32]float32{}
	monoHeight = map[float32]float32{}
}
