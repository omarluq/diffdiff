package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/highlight"
	"github.com/omarluq/diffdiff/internal/theme"
)

// gutterDigits reserves room for line numbers; six digits covers files past a
// hundred thousand lines without reflowing the gutter mid-file.
const gutterDigits = 6

// binaryNotice is shown in place of rows for files with no textual diff.
const binaryNotice = "Binary file not shown"

// DiffView is a virtualized unified-diff viewer. It flattens a diff.File into a
// flat row model once per file and renders it with a recycling widget.List, so
// only on-screen rows allocate CanvasObjects. Syntax highlighting runs off the
// UI goroutine and is applied back via fyne.Do.
type DiffView struct {
	widget.BaseWidget

	highlighter *highlight.Highlighter
	rows        []row
	palette     palette
	metrics     rowMetrics

	list   *widget.List
	binary *canvas.Text
	holder *fyne.Container

	// split selects side-by-side layout over the unified (stacked) layout. file
	// and thm retain the current input so a layout toggle can re-flatten in place.
	split bool
	file  *diff.File
	thm   *theme.Theme

	// generation guards against a stale highlight result from a previous file
	// landing after the user has already switched files.
	generation uint64
}

// NewDiffView builds an empty diff view backed by the given highlighter.
func NewDiffView(highlighter *highlight.Highlighter) *DiffView {
	view := &DiffView{
		BaseWidget:  widget.BaseWidget{},
		highlighter: highlighter,
		rows:        nil,
		palette:     palette{},
		metrics:     rowMetrics{},
		list:        nil,
		binary:      nil,
		holder:      nil,
		split:       false,
		file:        nil,
		thm:         nil,
		generation:  0,
	}
	view.ExtendBaseWidget(view)
	view.buildList()

	return view
}

// buildList constructs the recycling list and the binary-notice overlay once.
func (v *DiffView) buildList() {
	v.list = widget.NewList(
		func() int { return len(v.rows) },
		func() fyne.CanvasObject { return newDiffRow(v.metrics, v.palette, diffTextSize) },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			dr, ok := obj.(*diffRow)
			if !ok || id < 0 || id >= len(v.rows) {
				return
			}
			dr.setRow(&v.rows[id], v.palette)
		},
	)
	v.list.HideSeparators = true

	v.binary = canvas.NewText(binaryNotice, color.Gray{Y: 0x88})
	v.binary.Alignment = fyne.TextAlignCenter
	v.binary.Hide()

	v.holder = container.NewStack(v.list, container.NewCenter(v.binary))
}

// CreateRenderer renders the list/binary holder.
func (v *DiffView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.holder)
}

// SetFile loads a diff file under theme thm: it recomputes metrics, flattens
// the file into rows, repaints immediately with plain text, then kicks off async
// highlighting. A nil file clears the view.
func (v *DiffView) SetFile(file *diff.File, thm *theme.Theme) {
	v.generation++
	v.file = file
	v.thm = thm
	pal := thm.Palette()
	v.palette = paletteFrom(&pal)
	v.metrics = computeMetrics(diffTextSize)
	v.binary.Color = v.palette.muted

	if file == nil {
		v.showRows(nil)

		return
	}
	if file.Binary {
		v.showBinary()

		return
	}

	v.showRows(v.flattenFile(file))
	v.startHighlight(file, thm, v.generation)
}

// SetSplit selects the split (side-by-side) or unified (stacked) layout. It
// re-flattens the current file in place when the mode actually changes so the
// toggle takes effect without a reselection.
func (v *DiffView) SetSplit(split bool) {
	if v.split == split {
		return
	}
	v.split = split
	if v.file != nil && v.thm != nil {
		v.SetFile(v.file, v.thm)
	}
}

// Split reports whether the side-by-side layout is active.
func (v *DiffView) Split() bool {
	return v.split
}

// flattenFile builds the row model for the active layout.
func (v *DiffView) flattenFile(file *diff.File) []row {
	if v.split {
		return flattenSplit(file, v.metrics)
	}

	return flatten(file, v.metrics)
}

// showRows swaps in a new row model and refreshes the list, hiding the binary
// notice.
func (v *DiffView) showRows(rows []row) {
	v.rows = rows
	v.binary.Hide()
	v.list.Show()
	v.list.Refresh()
	v.list.ScrollToTop()
}

// showBinary clears rows and reveals the centered binary-file notice.
func (v *DiffView) showBinary() {
	v.rows = nil
	v.list.Refresh()
	v.list.Hide()
	v.binary.Show()
	v.binary.Refresh()
}

// flatten turns a file's hunks into a flat row slice: a separator row precedes
// each hunk, followed by one row per diff line. Each line row records the index
// of its content within the reconstructed old or new file body (hlIndex/hlOld)
// so async highlight tokens can be attached by position. Per-row gutter width is
// fixed by the shared metrics so the layout never reflows while scrolling.
func flatten(file *diff.File, metrics rowMetrics) []row {
	rows := make([]row, 0, file.TotalLines()+len(file.Hunks))
	oldIdx, newIdx := 0, 0
	for hi := range file.Hunks {
		hunk := &file.Hunks[hi]
		rows = append(rows, row{
			kind:    rowSeparator,
			header:  hunkHeader(hunk),
			line:    diff.Line{},
			tokens:  nil,
			hlIndex: 0,
			hlOld:   false,
			gutterW: metrics.gutterW,
		})
		for li := range hunk.Lines {
			line := hunk.Lines[li]
			var (
				idx     int
				fromOld bool
			)
			switch line.Kind {
			case diff.LineDeleted:
				idx, fromOld = oldIdx, true
				oldIdx++
			case diff.LineAdded:
				idx, fromOld = newIdx, false
				newIdx++
			case diff.LineContext:
				idx, fromOld = newIdx, false
				oldIdx++
				newIdx++
			default:
				idx, fromOld = newIdx, false
				newIdx++
			}
			rows = append(rows, row{
				kind:    rowLine,
				header:  "",
				line:    line,
				tokens:  nil,
				hlIndex: idx,
				hlOld:   fromOld,
				gutterW: metrics.gutterW,
			})
		}
	}

	return rows
}

// hunkHeader formats the unified-diff hunk marker plus its section heading.
func hunkHeader(hunk *diff.Hunk) string {
	marker := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
		hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines)
	if hunk.Section == "" {
		return marker
	}

	return marker + " " + hunk.Section
}

// flattenSplit turns a file into side-by-side rows: a separator row per hunk
// followed by paired old/new line rows from diff.SplitRows. Each cell records
// its index within the reconstructed old/new body so async highlight tokens can
// be attached by position, mirroring the unified flatten.
func flattenSplit(file *diff.File, metrics rowMetrics) []row {
	splits := diff.SplitRows(file)
	rows := make([]row, 0, len(splits))
	for si := range splits {
		split := &splits[si]
		if split.Separator {
			rows = append(rows, row{
				kind:    rowSeparator,
				header:  splitHunkHeader(split),
				line:    diff.Line{},
				tokens:  nil,
				hlIndex: 0,
				hlOld:   false,
				left:    splitCell{present: false, line: diff.Line{}, tokens: nil, hlIndex: -1},
				right:   splitCell{present: false, line: diff.Line{}, tokens: nil, hlIndex: -1},
				gutterW: metrics.gutterW,
			})

			continue
		}
		rows = append(rows, row{
			kind:    rowSplit,
			header:  "",
			line:    diff.Line{},
			tokens:  nil,
			hlIndex: 0,
			hlOld:   false,
			left:    splitCellFrom(split.Left, split.LeftIndex),
			right:   splitCellFrom(split.Right, split.RightIndex),
			gutterW: metrics.gutterW,
		})
	}

	return rows
}

// splitCellFrom builds a split cell from an optional line and its body index; a
// nil line yields an absent cell.
func splitCellFrom(line *diff.Line, hlIndex int) splitCell {
	if line == nil {
		return splitCell{present: false, line: diff.Line{}, tokens: nil, hlIndex: -1}
	}

	return splitCell{present: true, line: *line, tokens: nil, hlIndex: hlIndex}
}

// splitHunkHeader formats the hunk marker for a split separator row.
func splitHunkHeader(split *diff.SplitRow) string {
	marker := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
		split.OldStart, split.OldLines, split.NewStart, split.NewLines)
	if split.Section == "" {
		return marker
	}

	return marker + " " + split.Section
}

// computeMetrics derives the shared monospace layout measurements for a text
// size: glyph advance, row height, gutter width, and the content origin.
func computeMetrics(textSize float32) rowMetrics {
	advance := measureAdvance(textSize)
	height := lineHeight(textSize)
	padding := advance
	gutterW := float32(gutterDigits) * advance
	signW := advance
	contentX := gutterW*2 + signW + advance

	return rowMetrics{
		advance:  advance,
		height:   height,
		padding:  padding,
		gutterW:  gutterW,
		signW:    signW,
		contentX: contentX,
	}
}

// fileText reconstructs the old and new file bodies from the diff so the
// highlighter sees complete, lexable source rather than isolated lines.
func fileText(file *diff.File) (oldText, newText string) {
	var oldBuilder, newBuilder strings.Builder
	for hi := range file.Hunks {
		for li := range file.Hunks[hi].Lines {
			line := file.Hunks[hi].Lines[li]
			switch line.Kind {
			case diff.LineDeleted:
				appendLine(&oldBuilder, line.Content)
			case diff.LineAdded:
				appendLine(&newBuilder, line.Content)
			case diff.LineContext:
				appendLine(&oldBuilder, line.Content)
				appendLine(&newBuilder, line.Content)
			default:
				appendLine(&newBuilder, line.Content)
			}
		}
	}

	return oldBuilder.String(), newBuilder.String()
}

// appendLine writes a line and its newline to a builder.
func appendLine(builder *strings.Builder, content string) {
	builder.WriteString(content)
	builder.WriteByte('\n')
}
