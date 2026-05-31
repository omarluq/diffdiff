package highlight

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/samber/hot"
	"github.com/samber/oops"
)

// defaultMaxEntries bounds the cache when callers pass a non-positive size.
const defaultMaxEntries = 512

// Highlighter tokenizes file content into colored lines, memoizing results in
// an LRU cache (samber/hot). The cache is goroutine-safe, so Highlight may be
// called concurrently without external synchronization.
type Highlighter struct {
	cache *hot.HotCache[string, []Line]
}

// New builds a Highlighter whose LRU cache holds up to maxEntries results; a
// non-positive maxEntries selects the default capacity.
func New(maxEntries int) *Highlighter {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}

	cache := hot.NewHotCache[string, []Line](hot.LRU, maxEntries).Build()

	return &Highlighter{cache: cache}
}

// Highlight tokenizes content for the file at path, using style for colors, and
// returns one Line per source line (never nil on success). Identical
// (style, path, content) inputs are served from cache. Safe for concurrent use.
func (h *Highlighter) Highlight(path, content string, style *chroma.Style) ([]Line, error) {
	lexer := resolveLexer(path, content)
	key := cacheKey(style.Name, lexer.Config().Name, content)

	cached, found, err := h.cache.Get(key)
	if err != nil {
		return nil, oops.In("highlight").Code("cache_get").With("path", path).Wrapf(err, "read highlight cache")
	}
	if found {
		return cached, nil
	}

	iterator, err := lexer.Tokenise(&chroma.TokeniseOptions{
		State:    "root",
		Nested:   false,
		EnsureLF: true,
	}, content)
	if err != nil {
		return nil, oops.
			In("highlight").
			Code("tokenise").
			With("path", path).
			Wrapf(err, "tokenise content")
	}

	lines := buildLines(iterator.Tokens(), style)
	h.cache.Set(key, lines)

	return lines, nil
}

// cacheKey combines style, language, and a content hash; NUL separators keep the
// fields unambiguous regardless of their contents.
func cacheKey(styleName, lexerName, content string) string {
	sum := sha256.Sum256([]byte(content))
	return styleName + "\x00" + lexerName + "\x00" + hex.EncodeToString(sum[:])
}

// buildLines splits tokens into source lines, breaking on every '\n' inside a
// token's value. A final empty line left by EnsureLF's trailing newline is
// dropped so the line count matches the real source.
func buildLines(tokens []chroma.Token, style *chroma.Style) []Line {
	lines := []Line{{Tokens: nil}}
	for _, token := range tokens {
		appendToken(&lines, token, style)
	}
	if last := len(lines) - 1; last > 0 && len(lines[last].Tokens) == 0 {
		lines = lines[:last]
	}
	return lines
}

// appendToken distributes one chroma token across one or more lines, starting a
// new line at each embedded newline. Zero-length segments are skipped so blank
// lines stay empty rather than carrying empty tokens.
func appendToken(lines *[]Line, token chroma.Token, style *chroma.Style) {
	tok := styleToken(token.Type, style)
	segments := strings.Split(token.Value, "\n")
	for index, segment := range segments {
		if index > 0 {
			*lines = append(*lines, Line{Tokens: nil})
		}
		if segment == "" {
			continue
		}
		current := &(*lines)[len(*lines)-1]
		tok.Text = segment
		current.Tokens = append(current.Tokens, tok)
	}
}

// styleToken resolves a token type to a Token template (Text unset), falling
// back to the style's default Text color when the type has none.
func styleToken(ttype chroma.TokenType, style *chroma.Style) Token {
	entry := style.Get(ttype)
	col := entry.Colour //nolint:misspell // chroma's API spells it "Colour"
	if !col.IsSet() {
		col = style.Get(chroma.Text).Colour //nolint:misspell // chroma's API spells it "Colour"
	}
	return Token{
		Text:   "",
		Color:  nrgba(col),
		Bold:   entry.Bold == chroma.Yes,
		Italic: entry.Italic == chroma.Yes,
	}
}
