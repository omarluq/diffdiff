package theme_test

import (
	"sort"
	"testing"

	"github.com/alecthomas/chroma/v2/styles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/theme"
)

func TestNewRegistryLoadsEveryStyle(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()
	require.NotNil(t, reg)

	// Every chroma style becomes a theme (none of the bundled styles is the
	// Fallback and the display names do not collide), so the counts match.
	assert.Len(t, reg.Names(), len(styles.Names()))

	// Spot-check that the popular themes are present under their nice names.
	for _, name := range []string{
		"One Dark", "Dracula", "Nord", "GitHub Dark", "GitHub Light",
		"Monokai", "Gruvbox", "Catppuccin Mocha", "Tokyo Night", "Rosé Pine",
		"Solarized Dark", "Solarized Light", "Kanagawa Wave",
	} {
		_, ok := reg.Get(name)
		assert.Truef(t, ok, "expected theme %q to be present", name)
	}
}

func TestRegistryNamesSortedAndComplete(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()
	names := reg.Names()

	require.NotEmpty(t, names)
	assert.True(t, sort.StringsAreSorted(names), "names must be sorted")

	for _, name := range names {
		th, ok := reg.Get(name)
		require.Truef(t, ok, "Get(%q) should succeed", name)
		assert.Equal(t, name, th.Name())
	}
}

func TestEveryThemeMeetsContrast(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()

	// A tiny tolerance absorbs the rounding in the per-channel blend so a color
	// the derivation just pushed to the threshold isn't reported as a hair short.
	const tol = 0.02

	for _, name := range reg.Names() {
		th, ok := reg.Get(name)
		require.True(t, ok)
		pal := th.Palette()

		assert.GreaterOrEqualf(t, theme.ContrastRatio(pal.Foreground, pal.Background), theme.MinTextContrast-tol,
			"theme %q: foreground/background below AA text contrast", name)
		assert.GreaterOrEqualf(t, theme.ContrastRatio(pal.Muted, pal.Background), theme.MinMutedContrast-tol,
			"theme %q: muted/background below readable contrast", name)
		assert.GreaterOrEqualf(t, theme.ContrastRatio(pal.Accent, pal.Background), theme.MinAccentContrast-tol,
			"theme %q: accent/background below UI contrast", name)
	}
}

func TestRegistryNamesReturnsCopy(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()
	first := reg.Names()
	require.NotEmpty(t, first)

	first[0] = "mutated"

	assert.NotEqual(t, "mutated", reg.Names()[0], "Names must not expose internal slice")
}

func TestEveryThemeStyleResolves(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()

	for _, name := range reg.Names() {
		themeEntry, ok := reg.Get(name)
		require.True(t, ok)

		style := themeEntry.ChromaStyle()
		require.NotNil(t, style)

		// The stored style name must round-trip back to the same style object.
		// (chroma's Fallback style is registered as "swapoff", so we deliberately
		// do not exclude it — it is a real, selectable theme.)
		resolved := styles.Get(themeEntry.Palette().StyleName)
		assert.Same(t, style, resolved, "theme %q style mismatch", name)
	}
}

func TestDefaultIsOneDark(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()

	def := reg.Default()
	require.NotNil(t, def)
	assert.Equal(t, "One Dark", def.Name())
	assert.NotNil(t, def.ChromaStyle())

	viaGet, ok := reg.Get("One Dark")
	require.True(t, ok)
	assert.Same(t, def, viaGet)
}

func TestGetUnknownReturnsFalse(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()

	got, ok := reg.Get("Nonexistent Theme")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestEveryPaletteColorIsOpaque(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()

	for _, name := range reg.Names() {
		th, ok := reg.Get(name)
		require.True(t, ok)

		pal := th.Palette()
		colors := map[string]uint8{
			"Background": pal.Background.A,
			"Surface":    pal.Surface.A,
			"Overlay":    pal.Overlay.A,
			"Foreground": pal.Foreground.A,
			"Muted":      pal.Muted.A,
			"Border":     pal.Border.A,
			"Accent":     pal.Accent.A,
			"AddBg":      pal.AddBg.A,
			"AddEmph":    pal.AddEmph.A,
			"DelBg":      pal.DelBg.A,
			"DelEmph":    pal.DelEmph.A,
		}

		for field, alpha := range colors {
			assert.Equalf(t, uint8(255), alpha, "theme %q field %s must be opaque", name, field)
		}
	}
}

func TestDarkFlagMatchesKnownThemes(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()

	tests := []struct {
		name string
		dark bool
	}{
		{"Dracula", true},
		{"GitHub Light", false},
		{"GitHub Dark", true},
		{"Solarized Light", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			th, ok := reg.Get(tt.name)
			require.True(t, ok)
			assert.Equal(t, tt.dark, th.Palette().Dark)
		})
	}
}
