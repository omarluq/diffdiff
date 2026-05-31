package di

import (
	"github.com/samber/do/v2"

	"github.com/omarluq/diffdiff/internal/highlight"
)

// defaultHighlightCacheEntries bounds the syntax-highlight LRU cache. Entries
// are per (file, theme); a few hundred comfortably covers a large changeset.
const defaultHighlightCacheEntries = 512

// NewHighlighter builds the syntax highlighter backed by a bounded LRU cache.
func NewHighlighter(_ do.Injector) (*highlight.Highlighter, error) {
	return highlight.New(defaultHighlightCacheEntries), nil
}
