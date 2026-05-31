package theme

import "image/color"

// Blend re-exports the unexported blend helper for tests in the external
// theme_test package.
func Blend(first, second color.NRGBA, t float64) color.NRGBA {
	return blend(first, second, t)
}

// Luminance re-exports the unexported luminance helper for tests.
func Luminance(c color.NRGBA) float64 {
	return luminance(c)
}
