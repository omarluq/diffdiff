// Package theme is the single source of truth for diffdiff's visual identity.
// It derives a coherent UI/diff palette from any chroma v2 built-in style and
// exposes a curated registry plus a Fyne theme adapter so syntax highlighting
// and application chrome always share the same colors.
package theme

import (
	"image/color"

	"github.com/alecthomas/chroma/v2"
)

// Palette is a fully-resolved set of colors derived from a chroma style.
// Every field is opaque (alpha 255) and ready to use for both diff rendering
// and Fyne chrome.
type Palette struct {
	// Name is the human-facing display name.
	Name string
	// StyleName is the chroma style the palette was derived from.
	StyleName string
	// Dark reports whether the background is a dark color.
	Dark bool
	// Background is the editor surface color.
	Background color.NRGBA
	// Surface is used for side panels, headers, and gutters.
	Surface color.NRGBA
	// Overlay is the hover and selection background.
	Overlay color.NRGBA
	// Foreground is the primary text color.
	Foreground color.NRGBA
	// Muted is secondary text and line numbers.
	Muted color.NRGBA
	// Border is the divider and outline color.
	Border color.NRGBA
	// Accent is the focus and active-selection color.
	Accent color.NRGBA
	// AddBg is the full-row background for an added line.
	AddBg color.NRGBA
	// AddEmph is the intra-line emphasis background for added text.
	AddEmph color.NRGBA
	// DelBg is the full-row background for a deleted line.
	DelBg color.NRGBA
	// DelEmph is the intra-line emphasis background for deleted text.
	DelEmph color.NRGBA
}

// Theme pairs a derived Palette with the chroma style it came from.
type Theme struct {
	style   *chroma.Style
	palette Palette
}

// Name returns the theme's display name.
func (t *Theme) Name() string { return t.palette.Name }

// Palette returns the derived color palette.
func (t *Theme) Palette() Palette { return t.palette }

// ChromaStyle returns the underlying chroma style for syntax highlighting.
func (t *Theme) ChromaStyle() *chroma.Style { return t.style }
