package theme_test

import (
	"sort"
	"testing"

	"github.com/alecthomas/chroma/v2/styles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/theme"
)

func TestNewRegistryLoadsAllCuratedThemes(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()
	require.NotNil(t, reg)

	assert.Len(t, reg.Names(), 20)
}

func TestRegistryNamesSortedAndComplete(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()
	names := reg.Names()

	require.Len(t, names, 20)
	assert.True(t, sort.StringsAreSorted(names), "names must be sorted")

	for _, name := range names {
		th, ok := reg.Get(name)
		require.Truef(t, ok, "Get(%q) should succeed", name)
		assert.Equal(t, name, th.Name())
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

func TestEveryCuratedStyleResolves(t *testing.T) {
	t.Parallel()

	reg := theme.NewRegistry()

	for _, name := range reg.Names() {
		themeEntry, ok := reg.Get(name)
		require.True(t, ok)

		style := themeEntry.ChromaStyle()
		require.NotNil(t, style)
		// styles.Get returns the shared Fallback for unknown names; a curated
		// theme must derive from a real, distinct style.
		assert.NotSame(t, styles.Fallback, style, "theme %q resolved to fallback style", name)

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
