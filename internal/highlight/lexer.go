package highlight

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// resolveLexer picks a lexer for path/content, preferring a filename match,
// then content analysis, then the plain-text fallback. The result is coalesced
// so adjacent same-type tokens are merged before tokenisation.
func resolveLexer(path, content string) chroma.Lexer {
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Analyse(content) //nolint:misspell // chroma's API spells it "Analyse"
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	return chroma.Coalesce(lexer)
}

// LanguageName returns the chroma lexer config name for path, or "" when no
// lexer matches the filename. Content is not analyzed; matching is by path only.
func LanguageName(path string) string {
	lexer := lexers.Match(path)
	if lexer == nil {
		return ""
	}
	return lexer.Config().Name
}
