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

// TreeChildPaths re-exports the nested-view directory model: it builds the tree
// for files and returns the child UIDs of parent ("" for the root), so tests can
// assert the directory grouping and ordering.
func TreeChildPaths(files []*diff.File, parent string) []string {
	model := buildTreeModel(allEntries(files))

	return model.children[parent]
}

// TreeToggle drives the hamburger "Nested tree" action and reports whether the
// file panel switched to the nested view, verifying the toggle wiring.
func (c *Content) TreeToggle() bool {
	c.toggleTreeView()

	return c.fileList.nested
}
