package icons_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/icons"
)

const dark, light = true, false

func TestForResolvesByExtensionAndName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"main.go":          "go.png",
		"src/app.ts":       "typescript.png",
		"Dockerfile":       "docker.png",
		"go.mod":           "go-mod.png",
		"README.md":        "readme.png",
		"styles/main.scss": "sass.png",
		"photo.PNG":        "image.png", // extension match is case-insensitive
		"mystery.xyzzy":    "file.png",  // unknown extension falls back
		"noextension":      "file.png",  // no extension falls back
	}

	for path, want := range cases {
		res := icons.For(path, dark)
		require.NotNil(t, res, "icon for %q", path)
		assert.Equal(t, want, res.Name(), "icon name for %q", path)
		assert.NotEmpty(t, res.Content(), "icon %q must have PNG bytes", want)
	}
}

func TestForLightVariant(t *testing.T) {
	t.Parallel()

	// A .toml file uses its normal icon on dark themes and the light variant on
	// light themes (Material ships toml_light for light backgrounds).
	assert.Equal(t, "toml.png", icons.For("config.toml", dark).Name())
	assert.Equal(t, "toml_light.png", icons.For("config.toml", light).Name())

	// An icon without a light variant is unchanged across themes.
	assert.Equal(t, icons.For("main.go", dark).Name(), icons.For("main.go", light).Name())
}

func TestFolderForResolvesSpecialAndDefault(t *testing.T) {
	t.Parallel()

	cases := []struct {
		dir  string
		open bool
		want string
	}{
		{".github", false, "folder-github.png"},
		{".github", true, "folder-github-open.png"},
		{"SRC", false, "folder-src.png"}, // case-insensitive
		{"totally-unknown-dir", false, "folder.png"},
		{"totally-unknown-dir", true, "folder-open.png"},
	}

	for _, c := range cases {
		res := icons.FolderFor(c.dir, c.open, dark)
		require.NotNil(t, res, "folder icon for %q", c.dir)
		assert.Equal(t, c.want, res.Name(), "folder icon for %q (open=%v)", c.dir, c.open)
		assert.NotEmpty(t, res.Content())
	}
}

func TestFolderForLightVariant(t *testing.T) {
	t.Parallel()

	// .cursor has a light folder variant; .github does not.
	assert.Equal(t, "folder-cursor.png", icons.FolderFor(".cursor", false, dark).Name())
	assert.Equal(t, "folder-cursor_light.png", icons.FolderFor(".cursor", false, light).Name())
	assert.Equal(t,
		icons.FolderFor(".github", false, dark).Name(),
		icons.FolderFor(".github", false, light).Name(),
	)
}

func TestForCachesResource(t *testing.T) {
	t.Parallel()

	first := icons.For("a.go", dark)
	second := icons.For("b.go", dark)
	assert.Same(t, first, second, "the same icon name must reuse one cached resource")
}
