package diff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/diff"
)

func TestBuildHunksModify(t *testing.T) {
	t.Parallel()

	old := "line one\nline two\nline three\n"
	newContent := "line one\nline 2\nline three\n"

	hunks, added, deleted, err := diff.BuildHunks(old, newContent)
	require.NoError(t, err)
	assert.Equal(t, 1, added)
	assert.Equal(t, 1, deleted)
	require.Len(t, hunks, 1)

	h := hunks[0]
	assert.Equal(t, 1, h.OldStart)
	assert.Equal(t, 1, h.NewStart)

	// Find the changed pair and confirm numbering + intra-line segmentation.
	var del, ins *diff.Line
	for i := range h.Lines {
		switch h.Lines[i].Kind {
		case diff.LineDeleted:
			del = &h.Lines[i]
		case diff.LineAdded:
			ins = &h.Lines[i]
		case diff.LineContext:
		}
	}
	require.NotNil(t, del)
	require.NotNil(t, ins)
	assert.Equal(t, "line two", del.Content)
	assert.Equal(t, "line 2", ins.Content)
	assert.Equal(t, 2, del.OldNum)
	assert.Equal(t, 2, ins.NewNum)

	// Intra-line: the shared "line " prefix must be unchanged on both sides.
	require.NotEmpty(t, del.Segments)
	require.NotEmpty(t, ins.Segments)
	assert.False(t, del.Segments[0].Intraline)
	assert.Equal(t, "line ", del.Segments[0].Text)
	assert.True(t, hasIntralineChange(del.Segments))
	assert.True(t, hasIntralineChange(ins.Segments))

	// Segment text must reconstruct the line exactly.
	assert.Equal(t, del.Content, concat(del.Segments))
	assert.Equal(t, ins.Content, concat(ins.Segments))
}

func TestBuildHunksAddAndDeleteNumbering(t *testing.T) {
	t.Parallel()

	old := "a\nb\nc\n"
	newContent := "a\nb\nc\nd\n"

	hunks, added, deleted, err := diff.BuildHunks(old, newContent)
	require.NoError(t, err)
	assert.Equal(t, 1, added)
	assert.Equal(t, 0, deleted)
	require.Len(t, hunks, 1)

	last := hunks[0].Lines[len(hunks[0].Lines)-1]
	assert.Equal(t, diff.LineAdded, last.Kind)
	assert.Equal(t, "d", last.Content)
	assert.Equal(t, 4, last.NewNum)
	assert.Equal(t, 0, last.OldNum)
}

func TestBuildHunksIdentical(t *testing.T) {
	t.Parallel()

	hunks, added, deleted, err := diff.BuildHunks("same\n", "same\n")
	require.NoError(t, err)
	assert.Nil(t, hunks)
	assert.Zero(t, added)
	assert.Zero(t, deleted)
}

func hasIntralineChange(segs []diff.Segment) bool {
	for _, s := range segs {
		if s.Intraline {
			return true
		}
	}
	return false
}

func concat(segs []diff.Segment) string {
	out := ""
	for _, s := range segs {
		out += s.Text
	}
	return out
}
