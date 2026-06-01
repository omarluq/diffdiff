// Package icons maps file paths and directory names to Material Icon Theme
// icons. The icons are PNGs rasterized from the Material Icon Theme SVGs with
// resvg: Fyne's own SVG engine mis-renders many Material icons — paths with
// holes formed by winding fill solid, so e.g. the Go logo and the GitHub
// octocat collapse into blobs (fyne-io/fyne#5240) — whereas resvg renders them
// correctly, and Fyne downscales the PNG to row height crisply. Colorful icons
// can't recolor to an arbitrary theme, but the Material set ships light variants
// for icons that would wash out on a light background; those are selected when
// dark is false. The PNGs are embedded via the assets package; the lookup
// tables live in icons_gen.go.
package icons

import (
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"

	"github.com/omarluq/diffdiff/assets"
)

var (
	mu     sync.Mutex
	byName = make(map[string]fyne.Resource)
)

// For returns the Material icon resource for the file at path, preferring a
// whole-filename match over the extension and falling back to a generic file
// icon. On light themes (dark false) it uses an icon's light variant when one
// exists. It never returns nil and is safe for concurrent use.
func For(path string, dark bool) fyne.Resource {
	base := strings.ToLower(filepath.Base(path))
	if name, ok := pick(nameIcon, nameIconLight, base, dark); ok {
		return resource(name)
	}

	ext := strings.TrimPrefix(filepath.Ext(base), ".")
	if name, ok := pick(extIcon, extIconLight, ext, dark); ok {
		return resource(name)
	}

	return resource(defaultFileIcon)
}

// FolderFor returns the Material folder icon for the directory named dir,
// choosing the open or closed variant (and its light variant on light themes)
// and falling back to the generic folder when the name is not special-cased. It
// never returns nil and is concurrent-safe.
func FolderFor(dir string, open, dark bool) fyne.Resource {
	key := strings.ToLower(dir)
	if open {
		if name, ok := pick(dirIconOpen, dirIconOpenLight, key, dark); ok {
			return resource(name)
		}

		return resource(defaultFolderOpenIcon)
	}
	if name, ok := pick(dirIcon, dirIconLight, key, dark); ok {
		return resource(name)
	}

	return resource(defaultFolderIcon)
}

// pick resolves key in base, substituting the light-variant icon when dark is
// false and a light override exists for the key.
func pick(base, light map[string]string, key string, dark bool) (string, bool) {
	name, ok := base[key]
	if !ok {
		return "", false
	}
	if !dark {
		if variant, ok := light[key]; ok {
			return variant, true
		}
	}

	return name, true
}

// resource loads and memoizes the embedded PNG for an icon name, returning the
// default file icon when the name is unknown.
func resource(name string) fyne.Resource {
	mu.Lock()
	defer mu.Unlock()

	if cached, ok := byName[name]; ok {
		return cached
	}

	data, err := assets.Icons.ReadFile("imgs/icons/" + name + ".png")
	if err != nil {
		name = defaultFileIcon
		data, err = assets.Icons.ReadFile("imgs/icons/" + defaultFileIcon + ".png")
		if err != nil {
			return nil
		}
	}

	res := fyne.NewStaticResource(name+".png", data)
	byName[name] = res

	return res
}
