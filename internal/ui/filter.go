package ui

import (
	"github.com/sahilm/fuzzy"

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

	paths := make([]string, len(files))
	for i := range files {
		paths[i] = files[i].Path
	}

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
	entries := make([]fileEntry, len(files))
	for i := range files {
		entries[i] = fileEntry{file: files[i], matched: nil}
	}

	return entries
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
