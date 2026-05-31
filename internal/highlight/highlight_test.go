package highlight_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/highlight"
)

const goSnippet = `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`

// joinLines reconstructs the source by concatenating every token's text and
// joining lines with '\n', mirroring how a renderer would lay the tokens out.
func joinLines(lines []highlight.Line) string {
	rendered := make([]string, len(lines))
	for index, line := range lines {
		var builder strings.Builder
		for _, tok := range line.Tokens {
			builder.WriteString(tok.Text)
		}
		rendered[index] = builder.String()
	}
	return strings.Join(rendered, "\n")
}

func testStyle(t *testing.T) *chroma.Style {
	t.Helper()
	style := styles.Get("github")
	require.NotNil(t, style)
	return style
}

func newHighlighter(t *testing.T) *highlight.Highlighter {
	t.Helper()

	return highlight.New(0)
}

func TestHighlightLineCountAndReconstruction(t *testing.T) {
	t.Parallel()
	highlighter := newHighlighter(t)

	lines, err := highlighter.Highlight("main.go", goSnippet, testStyle(t))
	require.NoError(t, err)

	// goSnippet has a trailing newline; the normalized result drops exactly one,
	// so the rendered text equals the source without its final '\n'.
	want := strings.Split(strings.TrimSuffix(goSnippet, "\n"), "\n")
	assert.Len(t, lines, len(want))
	assert.Equal(t, strings.TrimSuffix(goSnippet, "\n"), joinLines(lines))
}

func TestHighlightKeywordColorDiffersFromText(t *testing.T) {
	t.Parallel()
	highlighter := newHighlighter(t)
	style := testStyle(t)

	lines, err := highlighter.Highlight("main.go", goSnippet, style)
	require.NoError(t, err)

	keywordColor, keywordFound := findTokenColor(lines, "package")
	identColor, identFound := findTokenColor(lines, "main")
	require.True(t, keywordFound, "expected a 'package' keyword token")
	require.True(t, identFound, "expected a 'main' identifier token")

	assert.NotEqual(t, keywordColor, identColor)
}

func TestHighlightCacheHitReturnsEqualData(t *testing.T) {
	t.Parallel()
	highlighter := newHighlighter(t)
	style := testStyle(t)

	first, err := highlighter.Highlight("main.go", goSnippet, style)
	require.NoError(t, err)
	second, err := highlighter.Highlight("main.go", goSnippet, style)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

func TestHighlightConcurrent(t *testing.T) {
	t.Parallel()
	highlighter := newHighlighter(t)
	style := testStyle(t)

	const goroutines = 32
	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutines)
	for worker := range goroutines {
		go func(seed int) {
			defer waitGroup.Done()
			// Mixing two inputs exercises both cache hits and concurrent inserts.
			content := goSnippet
			if seed%2 == 0 {
				content = "x := 1\ny := 2\n"
			}
			lines, err := highlighter.Highlight("main.go", content, style)
			assert.NoError(t, err)
			assert.NotEmpty(t, lines)
		}(worker)
	}
	waitGroup.Wait()
}

func TestHighlightUnknownExtensionFallsBack(t *testing.T) {
	t.Parallel()
	highlighter := newHighlighter(t)

	content := "just some\nplain text\n"
	lines, err := highlighter.Highlight("notes.unknownext", content, testStyle(t))
	require.NoError(t, err)
	assert.Len(t, lines, 2)
	assert.Equal(t, strings.TrimSuffix(content, "\n"), joinLines(lines))
}

func TestHighlightNoTrailingBlankLine(t *testing.T) {
	t.Parallel()
	highlighter := newHighlighter(t)

	// Content without a trailing newline must not gain or lose a line.
	content := "a := 1\nb := 2"
	lines, err := highlighter.Highlight("main.go", content, testStyle(t))
	require.NoError(t, err)
	assert.Len(t, lines, 2)
	assert.Equal(t, content, joinLines(lines))
}

func TestLanguageName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "go file", path: "x.go", want: "Go"},
		{name: "no extension", path: "noext", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, highlight.LanguageName(tc.path))
		})
	}
}

// findTokenColor returns the color of the first token whose trimmed text equals
// want, and whether such a token was found.
func findTokenColor(lines []highlight.Line, want string) (color, bool) {
	for _, line := range lines {
		for _, tok := range line.Tokens {
			if strings.TrimSpace(tok.Text) == want {
				return color{tok.Color.R, tok.Color.G, tok.Color.B}, true
			}
		}
	}
	return color{}, false
}

type color struct{ r, g, b uint8 }
