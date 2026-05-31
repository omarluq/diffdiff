package theme

import (
	"sort"

	"github.com/alecthomas/chroma/v2/styles"
)

// defaultThemeName is the theme returned by Registry.Default.
const defaultThemeName = "One Dark"

// curated maps display names to their chroma style names. These are the only
// themes diffdiff ships, each verified to exist in the chroma styles registry.
var curated = []struct {
	name  string
	style string
}{
	{"Dracula", "dracula"},
	{"Nord", "nord"},
	{"One Dark", "onedark"},
	{"GitHub Dark", "github-dark"},
	{"GitHub Light", "github"},
	{"Monokai", "monokai"},
	{"Solarized Dark", "solarized-dark"},
	{"Solarized Light", "solarized-light"},
	{"Gruvbox", "gruvbox"},
	{"Gruvbox Light", "gruvbox-light"},
	{"Catppuccin Mocha", "catppuccin-mocha"},
	{"Catppuccin Latte", "catppuccin-latte"},
	{"Tokyo Night", "tokyonight-night"},
	{"Tokyo Night Storm", "tokyonight-storm"},
	{"Rosé Pine", "rose-pine"},
	{"Rosé Pine Dawn", "rose-pine-dawn"},
	{"Kanagawa", "kanagawa-wave"},
	{"Doom One", "doom-one"},
	{"Xcode Dark", "xcode-dark"},
	{"Visual Studio", "vs"},
}

// Registry holds the curated set of derived themes keyed by display name.
type Registry struct {
	themes map[string]*Theme
	names  []string
}

// NewRegistry builds every curated theme from its chroma style. A style that
// is unexpectedly missing from the chroma registry is skipped so the rest of
// the curated set still loads. The returned registry is never nil.
func NewRegistry() *Registry {
	reg := &Registry{
		themes: make(map[string]*Theme, len(curated)),
		names:  make([]string, 0, len(curated)),
	}

	for _, entry := range curated {
		style := styles.Get(entry.style)
		// styles.Get returns the shared Fallback style for unknown names;
		// skipping those keeps an unexpected chroma change from corrupting a theme.
		if style == styles.Fallback {
			continue
		}

		reg.themes[entry.name] = &Theme{
			palette: derivePalette(entry.name, entry.style, style),
			style:   style,
		}
		reg.names = append(reg.names, entry.name)
	}

	sort.Strings(reg.names)

	return reg
}

// Names returns the display names of every registered theme, sorted.
func (r *Registry) Names() []string {
	out := make([]string, len(r.names))
	copy(out, r.names)

	return out
}

// Get returns the theme with the given display name and whether it was found.
func (r *Registry) Get(name string) (*Theme, bool) {
	theme, ok := r.themes[name]

	return theme, ok
}

// Default returns the "One Dark" theme.
func (r *Registry) Default() *Theme {
	return r.themes[defaultThemeName]
}
