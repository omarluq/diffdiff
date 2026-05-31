package theme

import (
	"image/color"
	"math"

	"github.com/alecthomas/chroma/v2"
)

// Diff accent anchors. Backgrounds are blended toward these so every theme,
// dark or light, gets recognizable add/delete tints regardless of its palette.
var (
	diffGreen = color.NRGBA{R: 0x3F, G: 0xB9, B: 0x50, A: 255}
	diffRed   = color.NRGBA{R: 0xF8, G: 0x51, B: 0x49, A: 255}
)

// defaultDarkBg is the fallback editor surface when a style omits its background.
var defaultDarkBg = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 255}

// fallbackAccent is used when a style exposes neither a function nor a
// namespace-keyword color to anchor the accent on.
var fallbackAccent = color.NRGBA{R: 88, G: 166, B: 255, A: 255}

// derivePalette builds a coherent UI/diff palette from a chroma style.
func derivePalette(name, styleName string, style *chroma.Style) Palette {
	background := backgroundColor(style)
	dark := luminance(background) < 0.5
	foreground := foregroundColor(style, dark)
	muted := mutedColor(style, foreground, background)
	accent := accentColor(style)

	amount := 0.18
	if dark {
		amount = 0.14
	}

	emph := math.Min(amount*2.1, 0.55)

	return Palette{
		Name:       name,
		StyleName:  styleName,
		Dark:       dark,
		Background: background,
		Surface:    blend(background, foreground, 0.04),
		Overlay:    blend(background, foreground, 0.10),
		Foreground: foreground,
		Muted:      muted,
		Border:     blend(background, foreground, 0.16),
		Accent:     accent,
		AddBg:      blend(background, diffGreen, amount),
		AddEmph:    blend(background, diffGreen, emph),
		DelBg:      blend(background, diffRed, amount),
		DelEmph:    blend(background, diffRed, emph),
	}
}

// backgroundColor reads the style's editor background, falling back to a dark
// default when the style leaves it unset.
func backgroundColor(style *chroma.Style) color.NRGBA {
	if entry := style.Get(chroma.Background); entry.Background.IsSet() {
		return chromaToNRGBA(entry.Background)
	}

	return defaultDarkBg
}

// foregroundColor reads the style's primary text color, falling back to a
// near-white or near-black tone chosen by the background's darkness.
func foregroundColor(style *chroma.Style, dark bool) color.NRGBA {
	if text := styleText(style.Get(chroma.Background)); text.IsSet() {
		return chromaToNRGBA(text)
	}

	if dark {
		return color.NRGBA{R: 0xEA, G: 0xEA, B: 0xEA, A: 255}
	}

	return color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 255}
}

// mutedColor prefers the comment text color, otherwise blends foreground
// halfway toward the background for a recognizable secondary tone.
func mutedColor(style *chroma.Style, foreground, background color.NRGBA) color.NRGBA {
	if text := styleText(style.Get(chroma.Comment)); text.IsSet() {
		return chromaToNRGBA(text)
	}

	return blend(foreground, background, 0.45)
}

// accentColor anchors the focus color on the style's function color, then its
// namespace-keyword color, then a neutral blue fallback.
func accentColor(style *chroma.Style) color.NRGBA {
	if text := styleText(style.Get(chroma.NameFunction)); text.IsSet() {
		return chromaToNRGBA(text)
	}

	if text := styleText(style.Get(chroma.KeywordNamespace)); text.IsSet() {
		return chromaToNRGBA(text)
	}

	return fallbackAccent
}

// styleText returns a style entry's text color, isolating the one reference to
// chroma's British-spelled field so the rest of the package stays US-spelled.
func styleText(entry chroma.StyleEntry) chroma.Colour { //nolint:misspell // chroma type is spelled "Colour"
	return entry.Colour //nolint:misspell // chroma's exported field is spelled "Colour"
}

// luminance returns relative perceptual brightness in [0,1] using Rec. 709
// coefficients.
func luminance(c color.NRGBA) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255.0
}

// blend linearly interpolates each channel from first toward second by ratio,
// clamped to [0,1], and always returns an opaque color.
func blend(first, second color.NRGBA, ratio float64) color.NRGBA {
	ratio = math.Min(math.Max(ratio, 0), 1)

	return color.NRGBA{
		R: lerpChannel(first.R, second.R, ratio),
		G: lerpChannel(first.G, second.G, ratio),
		B: lerpChannel(first.B, second.B, ratio),
		A: 255,
	}
}

// lerpChannel interpolates a single 8-bit channel and rounds to nearest. The
// result is clamped to [0,255] before conversion, so the narrowing is safe.
func lerpChannel(from, to uint8, ratio float64) uint8 {
	value := float64(from) + (float64(to)-float64(from))*ratio
	clamped := math.Round(math.Min(math.Max(value, 0), 255))

	return uint8(clamped)
}

// chromaToNRGBA converts a chroma text color to an opaque NRGBA, returning a
// zero (transparent) value when the color is unset.
func chromaToNRGBA(c chroma.Colour) color.NRGBA { //nolint:misspell // chroma type is spelled "Colour"
	if !c.IsSet() {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	}

	return color.NRGBA{R: c.Red(), G: c.Green(), B: c.Blue(), A: 255}
}
