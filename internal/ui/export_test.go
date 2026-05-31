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

// SplitRowShapes re-exports the unexported flattenSplit helper, describing each
// produced row: "sep" for a hunk separator, otherwise a two-character column
// presence marker — "LR" both sides, "L-" old only, "-R" new only.
func SplitRowShapes(file *diff.File) []string {
	rows := flattenSplit(file, computeMetrics(diffTextSize))
	shapes := make([]string, len(rows))
	for i := range rows {
		if rows[i].kind == rowSeparator {
			shapes[i] = "sep"

			continue
		}
		left, right := "-", "-"
		if rows[i].left.present {
			left = "L"
		}
		if rows[i].right.present {
			right = "R"
		}
		shapes[i] = left + right
	}

	return shapes
}

// SplitToggle drives the hamburger "Split view" action and reports the diff
// view's resulting layout, letting tests verify the toggle wiring end to end.
func (c *Content) SplitToggle() bool {
	c.toggleSplitView()

	return c.diffView.Split()
}
