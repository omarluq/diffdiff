package ui

import (
	"github.com/sahilm/fuzzy"
	"github.com/samber/lo"

	"github.com/omarluq/diffdiff/internal/diff"
)

// filterFiles returns the entries to display for a query. An empty query yields
// every file in original order with no match highlighting; a non-empty query
// runs a fuzzy match over the paths and returns survivors in descending score
// order, each tagged with the rune positions that matched.
func filterFiles(files []*diff.File, query string) []fileEntry {
	if query == "" {
		return allEntries(files)
	}

	paths := lo.Map(files, func(file *diff.File, _ int) string { return file.Path })

	matches := fuzzy.Find(query, paths)
	entries := make([]fileEntry, 0, len(matches))
	for _, match := range matches {
		if match.Index < 0 || match.Index >= len(files) {
			continue
		}
		entries = append(entries, fileEntry{
			file:    files[match.Index],
			matched: indexSet(match.MatchedIndexes),
		})
	}

	return entries
}

// allEntries wraps every file as an unfiltered entry preserving input order.
func allEntries(files []*diff.File) []fileEntry {
	return lo.Map(files, func(file *diff.File, _ int) fileEntry {
		return fileEntry{file: file, matched: nil}
	})
}

// indexSet turns a list of matched byte offsets into a set for O(1) lookup
// while rendering per-rune emphasis.
func indexSet(indexes []int) map[int]bool {
	if len(indexes) == 0 {
		return nil
	}
	set := make(map[int]bool, len(indexes))
	for _, idx := range indexes {
		set[idx] = true
	}

	return set
}

// fuzzyOptions returns the options matching query in descending score order, or
// all options in their original order for an empty query. It powers the filter
// box in the theme and font pickers.
func fuzzyOptions(options []string, query string) []string {
	if query == "" {
		return options
	}

	matches := fuzzy.Find(query, options)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.Index >= 0 && match.Index < len(options) {
			out = append(out, options[match.Index])
		}
	}

	return out
}
