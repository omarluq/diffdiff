package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
)

// fyneTheme adapts a derived Theme into a full fyne.Theme, mapping Fyne's
// semantic color names onto the palette and delegating fonts, icons, sizes,
// and any unmapped color to Fyne's default theme.
type fyneTheme struct {
	palette Palette
	mono    fyne.Resource
}

// NewFyneTheme returns a fyne.Theme backed by the given diffdiff theme, using
// mono as the monospace font (a nil mono falls back to Fyne's default).
func NewFyneTheme(t *Theme, mono fyne.Resource) fyne.Theme {
	return &fyneTheme{palette: t.palette, mono: mono}
}

// Color maps a Fyne color name onto the palette, delegating unmapped names to
// the default theme using the variant implied by the palette's darkness.
func (f *fyneTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if mapped, ok := f.mapColor(name); ok {
		return mapped
	}

	return fynetheme.DefaultTheme().Color(name, f.variant())
}

// mapColor resolves the palette color for a known Fyne color name.
func (f *fyneTheme) mapColor(name fyne.ThemeColorName) (color.Color, bool) {
	switch name {
	case fynetheme.ColorNameBackground:
		return f.palette.Background, true
	case fynetheme.ColorNameForeground:
		return f.palette.Foreground, true
	case fynetheme.ColorNameButton, fynetheme.ColorNameInputBackground:
		return f.palette.Surface, true
	case fynetheme.ColorNameDisabled, fynetheme.ColorNamePlaceHolder:
		return f.palette.Muted, true
	case fynetheme.ColorNameSeparator:
		return f.palette.Border, true
	case fynetheme.ColorNamePrimary:
		return f.palette.Accent, true
	case fynetheme.ColorNameHover, fynetheme.ColorNameSelection:
		return f.palette.Overlay, true
	case fynetheme.ColorNameMenuBackground, fynetheme.ColorNameOverlayBackground:
		return f.palette.Surface, true
	default:
		return nil, false
	}
}

// variant reports the Fyne theme variant implied by the palette's darkness.
func (f *fyneTheme) variant() fyne.ThemeVariant {
	if f.palette.Dark {
		return fynetheme.VariantDark
	}

	return fynetheme.VariantLight
}

// Font returns the active monospace font for monospace text and delegates other
// styles to the default theme.
func (f *fyneTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace && f.mono != nil {
		return f.mono
	}

	return fynetheme.DefaultTheme().Font(style)
}

// Icon delegates to the default theme's icon for the given name.
func (f *fyneTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return fynetheme.DefaultTheme().Icon(name)
}

// Size delegates to the default theme's size for the given name.
func (f *fyneTheme) Size(name fyne.ThemeSizeName) float32 {
	return fynetheme.DefaultTheme().Size(name)
}
