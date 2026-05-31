// Package highlight turns file content into per-line colored tokens using
// chroma v2. Results are cached in an LRU keyed by (style, language, content
// hash) so re-highlighting an unchanged file is free, and every exported method
// is safe to call concurrently from off-UI goroutines.
package highlight

import (
	"image/color"

	"github.com/alecthomas/chroma/v2"
)

// fallbackColor is used when neither a token's style entry nor the style's
// default Text entry specify a color; near-white keeps text legible on the
// dark backgrounds diffdiff renders against.
var fallbackColor = color.NRGBA{R: 0xE6, G: 0xE6, B: 0xE6, A: 0xFF}

// Token is a contiguous run of text sharing one set of visual attributes.
type Token struct {
	Text   string
	Color  color.NRGBA
	Bold   bool
	Italic bool
}

// Line is a single source line's tokens, carrying no trailing newline.
type Line struct {
	Tokens []Token
}

// nrgba converts a chroma color into a fully opaque NRGBA, substituting
// fallbackColor when the color is unset.
func nrgba(col chroma.Colour) color.NRGBA { //nolint:misspell // chroma's API spells it "Colour"
	if !col.IsSet() {
		return fallbackColor
	}
	return color.NRGBA{R: col.Red(), G: col.Green(), B: col.Blue(), A: 0xFF}
}
