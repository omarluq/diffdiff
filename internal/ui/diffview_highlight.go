package ui

import (
	"fyne.io/fyne/v2"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/highlight"
	"github.com/omarluq/diffdiff/internal/theme"
)

// startHighlight tokenizes the file's old and new bodies on a background
// goroutine, then applies the result on the UI goroutine via fyne.Do. The
// generation captured at launch is compared on completion so a result for a
// file the user already navigated away from is discarded.
func (v *DiffView) startHighlight(file *diff.File, thm *theme.Theme, generation uint64) {
	oldText, newText := fileText(file)
	style := thm.ChromaStyle()
	oldPath, newPath := highlightPaths(file)

	go func() {
		oldLines, errOld := v.highlighter.Highlight(oldPath, oldText, style)
		newLines, errNew := v.highlighter.Highlight(newPath, newText, style)
		if errOld != nil {
			oldLines = nil
		}
		if errNew != nil {
			newLines = nil
		}

		fyne.Do(func() {
			v.applyHighlight(generation, oldLines, newLines)
		})
	}()
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
		if current.kind != rowLine {
			continue
		}
		if current.hlOld {
			current.tokens = lineTokens(oldLines, current.hlIndex)
		} else {
			current.tokens = lineTokens(newLines, current.hlIndex)
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
