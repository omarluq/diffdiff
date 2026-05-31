package ui

import "github.com/omarluq/diffdiff/internal/diff"

// FlattenRowKinds re-exports the unexported flatten helper for the external
// ui_test package, returning a compact description of each produced row: true
// for a hunk separator, false for a diff line. Tests assert on this to verify a
// file flattens into the expected separator+line sequence.
func FlattenRowKinds(file *diff.File) []bool {
	rows := flatten(file, computeMetrics(diffTextSize))
	kinds := make([]bool, len(rows))
	for i := range rows {
		kinds[i] = rows[i].kind == rowSeparator
	}

	return kinds
}
