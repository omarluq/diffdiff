package theme

import (
	"embed"
	"sort"

	"fyne.io/fyne/v2"
)

// fontFS holds the bundled monospace programming fonts. They are embedded so the
// app ships a consistent set regardless of what is installed, and so the diff
// and file-list text use a face with generous descent metrics (Fyne's default
// monospace face clips the underscore's bottom pixel row at low display scales).
//
//go:embed fonts/*.ttf
var fontFS embed.FS

// defaultFontName is the font selected on startup.
const defaultFontName = "JetBrains Mono"

// fontDef maps a display name to its embedded TTF file.
type fontDef struct {
	name string
	file string
}

// fontDefs is the curated set of popular open-source programming fonts, in no
// particular order (the registry sorts display names).
var fontDefs = []fontDef{
	{name: "JetBrains Mono", file: "JetBrainsMono-Regular.ttf"},
	{name: "Fira Mono", file: "FiraMono-Regular.ttf"},
	{name: "Hack", file: "Hack-Regular.ttf"},
	{name: "Source Code Pro", file: "SourceCodePro-Regular.ttf"},
	{name: "IBM Plex Mono", file: "IBMPlexMono-Regular.ttf"},
	{name: "Roboto Mono", file: "RobotoMono-Regular.ttf"},
	{name: "Ubuntu Mono", file: "UbuntuMono-Regular.ttf"},
	{name: "Space Mono", file: "SpaceMono-Regular.ttf"},
	{name: "Anonymous Pro", file: "AnonymousPro-Regular.ttf"},
	{name: "DM Mono", file: "DMMono-Regular.ttf"},
}

// FontRegistry holds the bundled monospace fonts addressable by display name.
type FontRegistry struct {
	fonts map[string]fyne.Resource
	names []string
}

// NewFontRegistry loads every embedded font. It never returns nil; a font that
// fails to load is skipped.
func NewFontRegistry() *FontRegistry {
	registry := &FontRegistry{fonts: make(map[string]fyne.Resource, len(fontDefs)), names: nil}

	for _, def := range fontDefs {
		data, err := fontFS.ReadFile("fonts/" + def.file)
		if err != nil {
			continue
		}
		registry.fonts[def.name] = fyne.NewStaticResource(def.file, data)
		registry.names = append(registry.names, def.name)
	}

	sort.Strings(registry.names)

	return registry
}

// Names returns the available font display names, sorted.
func (r *FontRegistry) Names() []string {
	out := make([]string, len(r.names))
	copy(out, r.names)

	return out
}

// Get returns the font resource for a display name.
func (r *FontRegistry) Get(name string) (fyne.Resource, bool) {
	res, ok := r.fonts[name]

	return res, ok
}

// DefaultName returns the display name of the startup font, falling back to the
// first available name if the preferred default is missing.
func (r *FontRegistry) DefaultName() string {
	if _, ok := r.fonts[defaultFontName]; ok {
		return defaultFontName
	}
	if len(r.names) > 0 {
		return r.names[0]
	}

	return ""
}

// Default returns the startup font resource (nil only if no fonts loaded).
func (r *FontRegistry) Default() fyne.Resource {
	res, _ := r.Get(r.DefaultName())

	return res
}
