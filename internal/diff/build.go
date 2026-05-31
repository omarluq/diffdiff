package diff

import (
	"strings"

	udiff "github.com/aymanbagabas/go-udiff"
	"github.com/samber/oops"
)

// maxIntralineLen bounds the line length we are willing to refine at the
// character level. Beyond this, intra-line highlighting is skipped to keep
// rendering fast on pathological inputs (minified bundles, generated blobs) —
// the same defensive cap Pierre applies before tokenizing.
const maxIntralineLen = 100_000

// BuildHunks computes the diff between old and new file content and returns the
// rendering model: hunks with per-line old/new numbering, intra-line change
// segments, and added/deleted totals.
//
// All line content is cloned off the source strings so the returned model does
// not pin oldContent/newContent in memory.
func BuildHunks(oldContent, newContent string) (hunks []Hunk, added, deleted int, err error) {
	edits := udiff.Strings(oldContent, newContent)
	if len(edits) == 0 {
		return nil, 0, 0, nil
	}

	unified, err := udiff.ToUnifiedDiff("a", "b", oldContent, edits, udiff.DefaultContextLines)
	if err != nil {
		return nil, 0, 0, oops.In("diff").Code("to_unified").Wrapf(err, "convert edits to unified diff")
	}

	hunks = make([]Hunk, 0, len(unified.Hunks))
	for _, uh := range unified.Hunks {
		h := convertHunk(uh)
		refineIntraline(h.Lines)

		a, d := countChanges(h.Lines)
		added += a
		deleted += d
		hunks = append(hunks, h)
	}

	return hunks, added, deleted, nil
}

// countChanges tallies added and deleted lines within a hunk.
func countChanges(lines []Line) (added, deleted int) {
	for i := range lines {
		switch lines[i].Kind {
		case LineAdded:
			added++
		case LineDeleted:
			deleted++
		case LineContext:
		}
	}

	return added, deleted
}

// convertHunk turns a go-udiff hunk into the rendering model, assigning 1-based
// old/new line numbers as it walks the line operations.
func convertHunk(uh *udiff.Hunk) Hunk {
	oldNum := uh.FromLine
	newNum := uh.ToLine
	oldLines, newLines := 0, 0

	lines := make([]Line, 0, len(uh.Lines))
	for _, ul := range uh.Lines {
		content := strings.Clone(strings.TrimSuffix(ul.Content, "\n"))
		line := Line{Kind: LineContext, OldNum: 0, NewNum: 0, Content: content, Segments: nil}

		switch ul.Kind {
		case udiff.Insert:
			line.Kind = LineAdded
			line.NewNum = newNum
			newNum++
			newLines++
		case udiff.Delete:
			line.Kind = LineDeleted
			line.OldNum = oldNum
			oldNum++
			oldLines++
		case udiff.Equal:
			line.OldNum = oldNum
			line.NewNum = newNum
			oldNum++
			newNum++
			oldLines++
			newLines++
		}

		lines = append(lines, line)
	}

	return Hunk{
		OldStart: uh.FromLine,
		OldLines: oldLines,
		NewStart: uh.ToLine,
		NewLines: newLines,
		Section:  "",
		Lines:    lines,
	}
}

// refineIntraline finds runs of consecutive deleted lines immediately followed
// by consecutive inserted lines and pairs them line-by-line, computing
// character-level change segments for each pair. This is the within-line
// highlight that distinguishes a small edit from a wholesale line replacement.
func refineIntraline(lines []Line) {
	i := 0
	for i < len(lines) {
		if lines[i].Kind != LineDeleted {
			i++
			continue
		}

		delStart := i
		for i < len(lines) && lines[i].Kind == LineDeleted {
			i++
		}
		delEnd := i

		insStart := i
		for i < len(lines) && lines[i].Kind == LineAdded {
			i++
		}
		insEnd := i

		pairs := min(delEnd-delStart, insEnd-insStart)
		for k := range pairs {
			del := &lines[delStart+k]
			ins := &lines[insStart+k]
			if len(del.Content) > maxIntralineLen || len(ins.Content) > maxIntralineLen {
				continue
			}
			del.Segments, ins.Segments = intralineSegments(del.Content, ins.Content)
		}
	}
}
