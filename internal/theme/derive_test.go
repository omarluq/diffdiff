package theme_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/omarluq/diffdiff/internal/theme"
)

func TestBlendEndpoints(t *testing.T) {
	t.Parallel()

	first := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	second := color.NRGBA{R: 200, G: 150, B: 100, A: 255}

	atZero := theme.Blend(first, second, 0)
	assert.Equal(t, first.R, atZero.R)
	assert.Equal(t, first.G, atZero.G)
	assert.Equal(t, first.B, atZero.B)

	atOne := theme.Blend(first, second, 1)
	assert.Equal(t, second.R, atOne.R)
	assert.Equal(t, second.G, atOne.G)
	assert.Equal(t, second.B, atOne.B)
}

func TestBlendMidpoint(t *testing.T) {
	t.Parallel()

	first := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	second := color.NRGBA{R: 100, G: 100, B: 100, A: 255}

	mid := theme.Blend(first, second, 0.5)
	assert.Equal(t, uint8(50), mid.R)
	assert.Equal(t, uint8(50), mid.G)
	assert.Equal(t, uint8(50), mid.B)
}

func TestBlendAlwaysOpaque(t *testing.T) {
	t.Parallel()

	first := color.NRGBA{R: 1, G: 2, B: 3, A: 0}
	second := color.NRGBA{R: 4, G: 5, B: 6, A: 10}

	assert.Equal(t, uint8(255), theme.Blend(first, second, 0).A)
	assert.Equal(t, uint8(255), theme.Blend(first, second, 0.5).A)
	assert.Equal(t, uint8(255), theme.Blend(first, second, 1).A)
}

func TestBlendClampsParameter(t *testing.T) {
	t.Parallel()

	first := color.NRGBA{R: 10, G: 10, B: 10, A: 255}
	second := color.NRGBA{R: 250, G: 250, B: 250, A: 255}

	// t below 0 clamps to the first endpoint.
	below := theme.Blend(first, second, -5)
	assert.Equal(t, first.R, below.R)

	// t above 1 clamps to the second endpoint.
	above := theme.Blend(first, second, 9)
	assert.Equal(t, second.R, above.R)
}

func TestLuminanceDarkVsLight(t *testing.T) {
	t.Parallel()

	black := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	assert.InDelta(t, 0.0, theme.Luminance(black), 0.0001)
	assert.InDelta(t, 1.0, theme.Luminance(white), 0.0001)
	assert.Less(t, theme.Luminance(black), 0.5)
	assert.Greater(t, theme.Luminance(white), 0.5)
}

func TestLuminanceWeightsGreenHighest(t *testing.T) {
	t.Parallel()

	red := color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	green := color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	blue := color.NRGBA{R: 0, G: 0, B: 255, A: 255}

	// Rec. 709 weights green far above red and blue.
	assert.Greater(t, theme.Luminance(green), theme.Luminance(red))
	assert.Greater(t, theme.Luminance(red), theme.Luminance(blue))
}
