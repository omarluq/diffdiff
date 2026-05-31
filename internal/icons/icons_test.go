package icons_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/icons"
)

func TestForResolvesByExtensionAndName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"main.go":          "go.svg",
		"src/app.ts":       "typescript.svg",
		"component.tsx":    "react.svg",
		"Dockerfile":       "docker.svg",
		"go.mod":           "go-mod.svg",
		"deps/yarn.lock":   "lock.svg",
		"README.md":        "readme.svg",
		"styles/main.scss": "sass.svg",
		"photo.PNG":        "image.svg",
		"mystery.xyzzy":    "document.svg",
		"noextension":      "document.svg",
	}

	for path, want := range cases {
		res := icons.For(path)
		require.NotNil(t, res, "icon for %q must not be nil", path)
		assert.Equal(t, want, res.Name(), "icon name for %q", path)
		assert.NotEmpty(t, res.Content(), "icon %q must have SVG bytes", want)
	}
}

func TestForCachesResource(t *testing.T) {
	t.Parallel()

	first := icons.For("a.go")
	second := icons.For("b.go")
	assert.Same(t, first, second, "the same icon name must reuse one cached resource")
}
