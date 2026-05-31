package diff

import udiff "github.com/aymanbagabas/go-udiff"

// intralineSegments computes character-level change segments for a paired
// deleted/added line. It returns the segmentation of the old line (delSegs) and
// the new line (insSegs); concatenating each side's segment text reproduces the
// original line exactly. Segments with Intraline=true are the runs that
// actually changed between the two lines.
func intralineSegments(oldLine, newLine string) (delSegs, insSegs []Segment) {
	edits := udiff.Strings(oldLine, newLine)
	if len(edits) == 0 {
		whole := []Segment{{Text: oldLine, Intraline: false}}
		return whole, []Segment{{Text: newLine, Intraline: false}}
	}

	delSegs = make([]Segment, 0, len(edits)*2+1)
	insSegs = make([]Segment, 0, len(edits)*2+1)

	pos := 0
	for _, e := range edits {
		if e.Start > pos {
			common := oldLine[pos:e.Start]
			delSegs = append(delSegs, Segment{Text: common, Intraline: false})
			insSegs = append(insSegs, Segment{Text: common, Intraline: false})
		}
		if e.End > e.Start {
			delSegs = append(delSegs, Segment{Text: oldLine[e.Start:e.End], Intraline: true})
		}
		if e.New != "" {
			insSegs = append(insSegs, Segment{Text: e.New, Intraline: true})
		}
		pos = e.End
	}
	if pos < len(oldLine) {
		tail := oldLine[pos:]
		delSegs = append(delSegs, Segment{Text: tail, Intraline: false})
		insSegs = append(insSegs, Segment{Text: tail, Intraline: false})
	}

	return delSegs, insSegs
}
