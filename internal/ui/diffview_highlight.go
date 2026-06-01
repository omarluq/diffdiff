package ui

import (
	"fyne.io/fyne/v2"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/highlight"
	"github.com/omarluq/diffdiff/internal/theme"
)

const (
	// highlightFullBytes is the combined old+new content size below which a file
	// is tokenized in a single pass. Above it, the visible rows are highlighted
	// from a cheap prefix first and the remainder completes in the background.
	highlightFullBytes = 128 * 1024
	// highlightPrefixRows is how many leading flattened rows the prefix pass must
	// cover (a few screens' worth); the prefix lexes up to the deepest body line
	// those rows reference.
	highlightPrefixRows = 400
	// highlightPrefixBuffer pads the prefix past the last covered row so a small
	// scroll still lands on highlighted lines before the full pass arrives.
	highlightPrefixBuffer = 64
)

// startHighlight tokenizes the file's old and new bodies on a background
// goroutine and applies the result on the UI goroutine via fyne.Do. The
// generation captured at launch is compared on completion so a result for a file
// the user already navigated away from is discarded.
//
// Small files highlight in a single pass. Large files (combined body over
// highlightFullBytes) first highlight only a prefix covering the visible rows —
// so on-screen color appears promptly — then re-highlight the whole file to fill
// the rest. The two passes run sequentially so the full result always lands
// last; chroma is deterministic from line 0, so its tokens are identical to the
// prefix's for the overlapping lines and the swap is invisible.
func (v *DiffView) startHighlight(file *diff.File, thm *theme.Theme, generation uint64) {
	oldText, newText := fileText(file)
	style := thm.ChromaStyle()
	oldPath, newPath := highlightPaths(file)

	tokenize := func(path, text string) []highlight.Line {
		lines, err := v.highlighter.Highlight(path, text, style)
		if err != nil {
			return nil
		}

		return lines
	}
	apply := func(oldLines, newLines []highlight.Line) {
		fyne.Do(func() { v.applyHighlight(generation, oldLines, newLines) })
	}

	if len(oldText)+len(newText) <= highlightFullBytes {
		go func() { apply(tokenize(oldPath, oldText), tokenize(newPath, newText)) }()

		return
	}

	oldN, newN := prefixExtent(v.rows, highlightPrefixRows)
	go func() {
		apply(tokenize(oldPath, prefixLines(oldText, oldN)), tokenize(newPath, prefixLines(newText, newN)))
		apply(tokenize(oldPath, oldText), tokenize(newPath, newText))
	}()
}

// prefixExtent reports how far into the reconstructed old and new bodies the
// first maxRows flattened rows reach (max line index + 1, plus a buffer). The
// prefix pass lexes up to these line counts so it covers exactly the rows shown
// first — correct even when a file's only hunk sits deep in the file, where
// those rows carry large indices.
func prefixExtent(rows []row, maxRows int) (oldN, newN int) {
	limit := min(len(rows), maxRows)
	for i := range limit {
		current := &rows[i]
		switch current.kind {
		case rowLine:
			if current.hlOld {
				oldN = max(oldN, current.hlIndex+1)
			} else {
				newN = max(newN, current.hlIndex+1)
			}
		case rowSplit:
			if current.left.present {
				oldN = max(oldN, current.left.hlIndex+1)
			}
			if current.right.present {
				newN = max(newN, current.right.hlIndex+1)
			}
		case rowSeparator:
		}
	}
	if oldN > 0 {
		oldN += highlightPrefixBuffer
	}
	if newN > 0 {
		newN += highlightPrefixBuffer
	}

	return oldN, newN
}

// prefixLines returns the first n lines of text (through the n-th newline), or
// the whole string when it has fewer than n lines. It lets the prefix pass feed
// chroma only the leading lines while preserving correct lexer state from line 0.
func prefixLines(text string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range len(text) {
		if text[i] == '\n' {
			count++
			if count == n {
				return text[:i+1]
			}
		}
	}

	return text
}

// highlightPaths returns the paths used to pick a lexer for each side. Renames
// lex the old side under its prior path so language detection stays correct.
func highlightPaths(file *diff.File) (oldPath, newPath string) {
	newPath = file.Path
	oldPath = file.Path
	if file.OldPath != "" {
		oldPath = file.OldPath
	}

	return oldPath, newPath
}

// applyHighlight attaches tokens to each line row by its recorded reconstructed
// -file index, then refreshes the list. It is a no-op if the view has moved on
// to a newer file since the highlight was launched.
func (v *DiffView) applyHighlight(generation uint64, oldLines, newLines []highlight.Line) {
	if generation != v.generation {
		return
	}
	for i := range v.rows {
		current := &v.rows[i]
		switch current.kind {
		case rowLine:
			if current.hlOld {
				current.tokens = lineTokens(oldLines, current.hlIndex)
			} else {
				current.tokens = lineTokens(newLines, current.hlIndex)
			}
		case rowSplit:
			if current.left.present {
				current.left.tokens = lineTokens(oldLines, current.left.hlIndex)
			}
			if current.right.present {
				current.right.tokens = lineTokens(newLines, current.right.hlIndex)
			}
		case rowSeparator:
		}
	}
	v.list.Refresh()
}

// lineTokens returns the tokens at a 0-based index, or nil when out of range so
// the row degrades to plain text rather than mis-coloring.
func lineTokens(lines []highlight.Line, index int) []highlight.Token {
	if index < 0 || index >= len(lines) {
		return nil
	}

	return lines[index].Tokens
}
