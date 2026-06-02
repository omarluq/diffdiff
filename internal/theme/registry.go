package theme

import (
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2/styles"
)

// defaultThemeName is the theme returned by Registry.Default.
const defaultThemeName = "One Dark"

// displayNames overrides the auto-generated title for chroma styles whose
// humanized id would read poorly — brand casing (GitHub, Xcode), accented or
// multi-word names (Rosé Pine, Tokyo Night), and acronyms. Styles not listed
// fall back to humanize(styleName).
var displayNames = map[string]string{
	"onedark":              "One Dark",
	"github":               "GitHub Light",
	"github-dark":          "GitHub Dark",
	"vs":                   "Visual Studio",
	"rose-pine":            "Rosé Pine",
	"rose-pine-dawn":       "Rosé Pine Dawn",
	"rose-pine-moon":       "Rosé Pine Moon",
	"tokyonight-night":     "Tokyo Night",
	"tokyonight-storm":     "Tokyo Night Storm",
	"tokyonight-moon":      "Tokyo Night Moon",
	"tokyonight-day":       "Tokyo Night Day",
	"catppuccin-mocha":     "Catppuccin Mocha",
	"catppuccin-macchiato": "Catppuccin Macchiato",
	"catppuccin-frappe":    "Catppuccin Frappé",
	"catppuccin-latte":     "Catppuccin Latte",
	"kanagawa-wave":        "Kanagawa Wave",
	"kanagawa-dragon":      "Kanagawa Dragon",
	"kanagawa-lotus":       "Kanagawa Lotus",
	"doom-one":             "Doom One",
	"doom-one2":            "Doom One 2",
	"xcode":                "Xcode",
	"xcode-dark":           "Xcode Dark",
	"solarized-dark":       "Solarized Dark",
	"solarized-dark256":    "Solarized Dark 256",
	"solarized-light":      "Solarized Light",
	"gruvbox-light":        "Gruvbox Light",
	"monokailight":         "Monokai Light",
	"base16-snazzy":        "Base16 Snazzy",
	"hr_high_contrast":     "HR High Contrast",
	"hrdark":               "HR Dark",
	"modus-operandi":       "Modus Operandi",
	"modus-vivendi":        "Modus Vivendi",
	"paraiso-dark":         "Paraiso Dark",
	"paraiso-light":        "Paraiso Light",
	"aura-theme-dark":      "Aura Dark",
	"aura-theme-dark-soft": "Aura Dark Soft",
	"rainbow_dash":         "Rainbow Dash",
	"algol_nu":             "Algol Nu",
	"bw":                   "Black & White",
	"rrt":                  "RRT",
	"rpgle":                "RPGLE",
	"abap":                 "ABAP",
}

// Registry holds every derived theme keyed by display name.
type Registry struct {
	themes map[string]*Theme
	names  []string
}

// NewRegistry derives a theme from every chroma style, so diffdiff offers the
// full catalog of popular editor themes. Each theme's display name comes from
// displayNames (brand casing) or a humanized form of the style id; a style that
// resolves to chroma's shared Fallback, or whose name collides with one already
// loaded, is skipped. The returned registry is never nil.
func NewRegistry() *Registry {
	styleNames := styles.Names()
	reg := &Registry{
		themes: make(map[string]*Theme, len(styleNames)),
		names:  make([]string, 0, len(styleNames)),
	}

	for _, styleName := range styleNames {
		// styles.Names lists only registered styles, so Get always resolves to a
		// real style — including chroma's own Fallback style, which is registered
		// under the name "swapoff" and is a legitimate theme in its own right.
		style := styles.Get(styleName)

		name := displayName(styleName)
		if _, exists := reg.themes[name]; exists {
			continue
		}

		reg.themes[name] = &Theme{
			palette: derivePalette(name, styleName, style),
			style:   style,
		}
		reg.names = append(reg.names, name)
	}

	sort.Strings(reg.names)

	return reg
}

// displayName resolves a chroma style id to its human-facing title.
func displayName(styleName string) string {
	if name, ok := displayNames[styleName]; ok {
		return name
	}

	return humanize(styleName)
}

// humanize turns a chroma style id like "paraiso-dark" into "Paraiso Dark":
// split on '-'/'_' and capitalize each word. Style ids are ASCII, so byte
// slicing the first letter is safe.
func humanize(styleName string) string {
	fields := strings.FieldsFunc(styleName, func(r rune) bool { return r == '-' || r == '_' })
	for i, field := range fields {
		if field == "" {
			continue
		}
		fields[i] = strings.ToUpper(field[:1]) + field[1:]
	}

	return strings.Join(fields, " ")
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
