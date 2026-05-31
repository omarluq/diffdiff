// Package icons maps file paths to Material Icon Theme file-type icons. The
// icons are SVGs embedded at build time (sourced from the iconify
// material-icon-theme set) and served as Fyne resources, cached by name so a
// large file list reuses one resource per type.
package icons

import (
	"embed"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
)

//go:embed svg/*.svg
var svgFS embed.FS

// defaultIcon backs any path with no more specific match; it is always embedded.
const defaultIcon = "document"

var (
	mu     sync.Mutex
	byName = make(map[string]fyne.Resource)
)

// Lookup tables are declared icon-first (one icon name → its triggers) so each
// icon name appears exactly once; the flat ext→icon and filename→icon maps used
// at lookup time are derived by inverting these at init.

// filenamesByIcon lists whole filenames (lowercased) that a bare extension would
// miss.
var filenamesByIcon = map[string][]string{
	"docker":   {"dockerfile"},
	"makefile": {"makefile"},
	"go-mod":   {"go.mod", "go.sum"},
	"nodejs":   {"package.json"},
	"license":  {"license", "license.md"},
	"readme":   {"readme", "readme.md"},
	"git":      {".gitignore", ".gitattributes"},
}

// extsByIcon lists the lowercase extensions (without dot) each icon represents.
var extsByIcon = map[string][]string{
	"go":         {"go"},
	"javascript": {"js", "mjs", "cjs"},
	"react":      {"jsx", "tsx"},
	"typescript": {"ts"},
	"python":     {"py"},
	"rust":       {"rs"},
	"java":       {"java"},
	"vue":        {"vue"},
	"c":          {"c", "h"},
	"cpp":        {"cc", "cpp", "cxx", "hpp"},
	"csharp":     {"cs"},
	"html":       {"html", "htm"},
	"css":        {"css"},
	"sass":       {"scss", "sass"},
	"json":       {"json"},
	"yaml":       {"yaml", "yml"},
	"markdown":   {"md", "markdown"},
	"console":    {"sh", "bash", "zsh", "fish"},
	"ruby":       {"rb"},
	"php":        {"php"},
	"swift":      {"swift"},
	"kotlin":     {"kt", "kts"},
	"database":   {"sql"},
	"toml":       {"toml"},
	"xml":        {"xml"},
	"gradle":     {"gradle"},
	"image":      {"png", "jpg", "jpeg", "gif", "svg", "webp", "bmp", "ico"},
	"video":      {"mp4", "mov", "avi", "mkv", "webm"},
	"audio":      {"mp3", "wav", "flac", "ogg"},
	"pdf":        {"pdf"},
	"lock":       {"lock"},
	"zip":        {"zip", "tar", "gz", "tgz", "bz2", "xz", "7z", "rar"},
}

var (
	byFilename = invert(filenamesByIcon)
	byExt      = invert(extsByIcon)
)

// invert flattens an icon-first table into a trigger→icon lookup map.
func invert(table map[string][]string) map[string]string {
	out := make(map[string]string)
	for icon, triggers := range table {
		for _, trigger := range triggers {
			out[trigger] = icon
		}
	}

	return out
}

// For returns the Material icon resource for the file at path, falling back to a
// generic document icon. It never returns nil and is safe for concurrent use.
func For(path string) fyne.Resource {
	return resource(iconName(path))
}

// Folder returns the Material folder icon for a directory, choosing the open or
// closed variant. Like For, it never returns nil and is safe for concurrent use.
func Folder(open bool) fyne.Resource {
	if open {
		return resource("folder-open")
	}

	return resource("folder")
}

// iconName resolves a path to an icon name, preferring a whole-filename match
// over the extension.
func iconName(path string) string {
	base := strings.ToLower(filepath.Base(path))
	if name, ok := byFilename[base]; ok {
		return name
	}

	ext := strings.TrimPrefix(filepath.Ext(base), ".")
	if name, ok := byExt[ext]; ok {
		return name
	}

	return defaultIcon
}

// resource loads and memoizes the embedded SVG for an icon name, returning the
// default icon when the name is unknown.
func resource(name string) fyne.Resource {
	mu.Lock()
	defer mu.Unlock()

	if cached, ok := byName[name]; ok {
		return cached
	}

	data, err := svgFS.ReadFile("svg/" + name + ".svg")
	if err != nil {
		name = defaultIcon
		data, err = svgFS.ReadFile("svg/" + defaultIcon + ".svg")
		if err != nil {
			return nil
		}
	}

	res := fyne.NewStaticResource(name+".svg", data)
	byName[name] = res

	return res
}
