package ui

import (
	"sort"
	"strings"
)

// treeModel is the directory hierarchy derived from a set of visible file
// entries for the nested file view. children maps a node UID — a "/"-joined path
// prefix, with "" for the root — to its sorted child UIDs; leaves maps a file's
// full-path UID to its entry. A UID is a branch (directory) exactly when it is
// not a leaf, so intermediate path segments become collapsible directories and
// the final segment is the file.
type treeModel struct {
	children map[string][]string
	leaves   map[string]fileEntry
}

// buildTreeModel groups entries into a directory tree. Each path segment becomes
// a node beneath its parent prefix; the final segment is recorded as the file
// leaf. Children are ordered directories-first, then by name, so the tree reads
// like a file explorer.
func buildTreeModel(entries []fileEntry) treeModel {
	model := treeModel{children: map[string][]string{}, leaves: map[string]fileEntry{}}
	registered := map[string]bool{}

	for _, entry := range entries {
		if entry.file == nil {
			continue
		}
		segments := strings.Split(entry.file.Path, "/")
		prefix := ""
		for depth, segment := range segments {
			parent := prefix
			if prefix == "" {
				prefix = segment
			} else {
				prefix += "/" + segment
			}
			if edge := parent + "\x00" + prefix; !registered[edge] {
				model.children[parent] = append(model.children[parent], prefix)
				registered[edge] = true
			}
			if depth == len(segments)-1 {
				model.leaves[prefix] = entry
			}
		}
	}

	model.sortChildren()

	return model
}

// sortChildren orders each node's children directories-first, then by base name.
func (m treeModel) sortChildren() {
	for parent := range m.children {
		kids := m.children[parent]
		sort.SliceStable(kids, func(a, b int) bool {
			leftDir, rightDir := m.isBranch(kids[a]), m.isBranch(kids[b])
			if leftDir != rightDir {
				return leftDir
			}

			return baseName(kids[a]) < baseName(kids[b])
		})
	}
}

// isBranch reports whether uid is a directory node: the root or any node that is
// not a file leaf.
func (m treeModel) isBranch(uid string) bool {
	_, leaf := m.leaves[uid]

	return !leaf
}

// label returns the trailing path segment displayed for a node.
func (m treeModel) label(uid string) string {
	return baseName(uid)
}

// baseName returns the path segment after the last slash in uid.
func baseName(uid string) string {
	if idx := strings.LastIndexByte(uid, '/'); idx >= 0 {
		return uid[idx+1:]
	}

	return uid
}
