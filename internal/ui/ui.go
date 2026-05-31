// Package ui assembles the Fyne widgets that render a unified git diff: a
// fuzzy-filterable list of changed files and a virtualized diff view with
// syntax highlighting and intra-line emphasis. It builds only widgets — the
// application and window are created by the command layer — so it never imports
// fyne.io/fyne/v2/app.
package ui

import (
	"image/color"

	"fyne.io/fyne/v2"

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
	addBg      color.NRGBA
	addEmph    color.NRGBA
	delBg      color.NRGBA
	delEmph    color.NRGBA
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
		addBg:      src.AddBg,
		addEmph:    src.AddEmph,
		delBg:      src.DelBg,
		delEmph:    src.DelEmph,
	}
}

// measureAdvance returns the pixel width of a single monospace glyph at size,
// the basis for all column math in the diff view.
func measureAdvance(size float32) float32 {
	return fyne.MeasureText(monoText, size, monoStyle()).Width
}

// lineHeight is the row height for monospace text at size.
func lineHeight(size float32) float32 {
	return fyne.MeasureText(monoText, size, monoStyle()).Height
}
