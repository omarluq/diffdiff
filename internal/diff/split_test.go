package diff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/diff"
)

// fileFrom builds a single-file diff from old/new content for split testing.
func fileFrom(t *testing.T, old, newContent string) *diff.File {
	t.Helper()

	hunks, added, deleted, err := diff.BuildHunks(old, newContent)
	require.NoError(t, err)

	return &diff.File{Path: "a.go", Hunks: hunks, Added: added, Deleted: deleted}
}

func TestSplitRowsModify(t *testing.T) {
	t.Parallel()

	file := fileFrom(t, "line one\nline two\nline three\n", "line one\nline 2\nline three\n")
	rows := diff.SplitRows(file)

	require.NotEmpty(t, rows)
	assert.True(t, rows[0].Separator, "a hunk opens with a separator row")

	// The change is paired: old line on the left, new line on the right, both
	// present on one row.
	var pair *diff.SplitRow
	for i := range rows {
		if rows[i].Left != nil && rows[i].Right != nil &&
			rows[i].Left.Kind == diff.LineDeleted && rows[i].Right.Kind == diff.LineAdded {
			pair = &rows[i]
		}
	}
	require.NotNil(t, pair, "the modified line must pair old-left with new-right")
	assert.Equal(t, "line two", pair.Left.Content)
	assert.Equal(t, "line 2", pair.Right.Content)

	// Body indices match the reconstructed old/new bodies the highlighter sees.
	assert.Equal(t, 1, pair.LeftIndex, "old body: one(0), two(1), three(2)")
	assert.Equal(t, 1, pair.RightIndex, "new body: one(0), 2(1), three(2)")
}

func TestSplitRowsContextOnBothSides(t *testing.T) {
	t.Parallel()

	file := fileFrom(t, "line one\nline two\nline three\n", "line one\nline 2\nline three\n")
	rows := diff.SplitRows(file)

	// A context line occupies both columns with the same content and ascending
	// per-side indices.
	var first *diff.SplitRow
	for i := range rows {
		if !rows[i].Separator && rows[i].Left != nil && rows[i].Right != nil &&
			rows[i].Left.Kind == diff.LineContext {
			first = &rows[i]

			break
		}
	}
	require.NotNil(t, first)
	assert.Equal(t, "line one", first.Left.Content)
	assert.Equal(t, first.Left.Content, first.Right.Content)
	assert.Equal(t, 0, first.LeftIndex)
	assert.Equal(t, 0, first.RightIndex)
}

func TestSplitRowsPureAddition(t *testing.T) {
	t.Parallel()

	file := fileFrom(t, "a\nb\nc\n", "a\nb\nc\nd\n")
	rows := diff.SplitRows(file)

	// The appended line has no old counterpart: left absent, right present.
	var added *diff.SplitRow
	for i := range rows {
		if rows[i].Right != nil && rows[i].Right.Kind == diff.LineAdded {
			added = &rows[i]
		}
	}
	require.NotNil(t, added)
	assert.Nil(t, added.Left, "a pure addition leaves the old column blank")
	assert.Equal(t, -1, added.LeftIndex)
	assert.Equal(t, "d", added.Right.Content)
	assert.Equal(t, 3, added.RightIndex, "new body: a(0), b(1), c(2), d(3)")
}

func TestSplitRowsPureDeletion(t *testing.T) {
	t.Parallel()

	file := fileFrom(t, "a\nb\nc\n", "a\nc\n")
	rows := diff.SplitRows(file)

	// The removed line has no new counterpart: right absent, left present.
	var removed *diff.SplitRow
	for i := range rows {
		if rows[i].Left != nil && rows[i].Left.Kind == diff.LineDeleted {
			removed = &rows[i]
		}
	}
	require.NotNil(t, removed)
	assert.Nil(t, removed.Right, "a pure deletion leaves the new column blank")
	assert.Equal(t, -1, removed.RightIndex)
	assert.Equal(t, "b", removed.Left.Content)
}

func TestSplitRowsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, diff.SplitRows(&diff.File{Path: "a.go", Hunks: nil}))
}
