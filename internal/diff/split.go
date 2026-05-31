package diff

// SplitRow is one row of a side-by-side (split) diff. A separator row carries a
// hunk header and spans both columns. A line row pairs an optional old line
// (Left) with an optional new line (Right); a nil side is rendered blank.
//
// LeftIndex and RightIndex give each side's position within the reconstructed
// old/new file body — the same ordering the highlighter tokenizes — so cached
// tokens can be looked up by index. They are -1 for an absent side.
type SplitRow struct {
	Separator bool
	Section   string
	OldStart  int
	OldLines  int
	NewStart  int
	NewLines  int

	Left       *Line
	LeftIndex  int
	Right      *Line
	RightIndex int
}

// SplitRows arranges a file's hunks for side-by-side display: each hunk opens
// with a separator row, context lines occupy both columns, and consecutive
// deleted/added lines are paired so a change shows old-on-left and new-on-right
// (with extra lines on one side leaving the other blank).
func SplitRows(file *File) []SplitRow {
	rows := make([]SplitRow, 0, file.TotalLines()+len(file.Hunks))
	oldIdx, newIdx := 0, 0

	for hi := range file.Hunks {
		hunk := &file.Hunks[hi]
		rows = append(rows, SplitRow{
			Separator:  true,
			Section:    hunk.Section,
			OldStart:   hunk.OldStart,
			OldLines:   hunk.OldLines,
			NewStart:   hunk.NewStart,
			NewLines:   hunk.NewLines,
			Left:       nil,
			LeftIndex:  -1,
			Right:      nil,
			RightIndex: -1,
		})
		rows, oldIdx, newIdx = appendHunkSplit(rows, hunk.Lines, oldIdx, newIdx)
	}

	return rows
}

// appendHunkSplit emits the split rows for one hunk's lines, threading the
// per-side body indices so they keep matching the reconstructed bodies.
func appendHunkSplit(
	rows []SplitRow, lines []Line, oldIdx, newIdx int,
) (out []SplitRow, oldNext, newNext int) {
	i := 0
	for i < len(lines) {
		if lines[i].Kind == LineContext {
			rows = append(rows, contextSplitRow(&lines[i], oldIdx, newIdx))
			oldIdx++
			newIdx++
			i++

			continue
		}

		delStart := i
		for i < len(lines) && lines[i].Kind == LineDeleted {
			i++
		}
		delEnd := i

		addStart := i
		for i < len(lines) && lines[i].Kind == LineAdded {
			i++
		}
		addEnd := i

		rows, oldIdx, newIdx = pairChange(rows, lines, delStart, delEnd, addStart, addEnd, oldIdx, newIdx)
	}

	return rows, oldIdx, newIdx
}

// contextSplitRow places a context line in both columns.
func contextSplitRow(line *Line, oldIdx, newIdx int) SplitRow {
	return SplitRow{
		Separator: false, Section: "", OldStart: 0, OldLines: 0, NewStart: 0, NewLines: 0,
		Left: line, LeftIndex: oldIdx, Right: line, RightIndex: newIdx,
	}
}

// pairChange pairs a run of deleted lines (left) with a run of added lines
// (right) row by row; the longer run leaves blanks opposite its surplus lines.
func pairChange(
	rows []SplitRow, lines []Line, delStart, delEnd, addStart, addEnd, oldIdx, newIdx int,
) (out []SplitRow, oldNext, newNext int) {
	delCount := delEnd - delStart
	addCount := addEnd - addStart

	for k := range max(delCount, addCount) {
		split := SplitRow{
			Separator: false, Section: "", OldStart: 0, OldLines: 0, NewStart: 0, NewLines: 0,
			Left: nil, LeftIndex: -1, Right: nil, RightIndex: -1,
		}
		if k < delCount {
			split.Left = &lines[delStart+k]
			split.LeftIndex = oldIdx
			oldIdx++
		}
		if k < addCount {
			split.Right = &lines[addStart+k]
			split.RightIndex = newIdx
			newIdx++
		}
		rows = append(rows, split)
	}

	return rows, oldIdx, newIdx
}
