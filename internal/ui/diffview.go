package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/highlight"
	"github.com/omarluq/diffdiff/internal/theme"
)

// hbarThickness is the height reserved for the horizontal scrollbar strip under
// the diff; it is only laid out when a line overflows the viewport.
const hbarThickness float32 = 14

// gutterDigits reserves room for line numbers; six digits covers files past a
// hundred thousand lines without reflowing the gutter mid-file.
const gutterDigits = 6

// binaryNotice is shown in place of rows for files with no textual diff.
const binaryNotice = "Binary file not shown"

// loadingNotice is shown while a selected file's diff is still being built in
// the background.
const loadingNotice = "Loading diff…"

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

	list    *widget.List
	binary  *canvas.Text
	loading *canvas.Text
	holder  *fyne.Container

	// Horizontal scrolling: hScrollCols is the shared offset in glyph columns;
	// maxRunes is the widest line's rune count (sets the scroll range and whether
	// content overflows). hbar is a native horizontal scrollbar driven by a wide
	// transparent spacer; it is shown only when a line overflows the viewport.
	hScrollCols int
	maxRunes    int
	hbar        *container.Scroll
	hspacer     *canvas.Rectangle

	// split selects side-by-side layout over the unified (stacked) layout. file
	// and thm retain the current input so a layout toggle can re-flatten in place.
	split bool
	file  *diff.File
	thm   *theme.Theme

	// styleName is the chroma style name whose tokens are currently applied to
	// the rows. A palette-only restyle compares against it to decide whether the
	// (expensive) re-highlight is needed or a plain recolor suffices.
	styleName string

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
		loading:     nil,
		holder:      nil,
		hScrollCols: 0,
		maxRunes:    0,
		hbar:        nil,
		hspacer:     nil,
		split:       false,
		file:        nil,
		thm:         nil,
		styleName:   "",
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
		func() fyne.CanvasObject {
			dr := newDiffRow(v.metrics, v.palette)
			dr.onScroll = v.scrollContent

			return dr
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			dr, ok := obj.(*diffRow)
			if !ok || id < 0 || id >= len(v.rows) {
				return
			}
			dr.setRow(&v.rows[id], v.palette, v.metrics, v.hScrollCols)
		},
	)
	v.list.HideSeparators = true

	v.binary = canvas.NewText(binaryNotice, color.Gray{Y: 0x88})
	v.binary.Alignment = fyne.TextAlignCenter
	v.binary.Hide()

	v.loading = canvas.NewText(loadingNotice, color.Gray{Y: 0x88})
	v.loading.Alignment = fyne.TextAlignCenter
	v.loading.Hide()

	// A native horizontal scrollbar driven by a wide transparent spacer: dragging
	// it reports an offset, which onHScroll converts to a glyph-column offset that
	// every visible row re-windows to. Hidden until a line overflows.
	v.hspacer = canvas.NewRectangle(color.Transparent)
	v.hspacer.SetMinSize(fyne.NewSize(0, hbarThickness))
	v.hbar = container.NewHScroll(v.hspacer)
	v.hbar.OnScrolled = v.onHScroll
	v.hbar.Hide()

	center := container.NewStack(
		v.list,
		container.NewCenter(v.binary),
		container.NewCenter(v.loading),
	)
	// Float the scrollbar over the bottom of the content rather than reserving a
	// strip for it (which left a seam above a dead band). The filler is a plain
	// transparent rectangle — not Scrollable/Hoverable — so wheel and hover events
	// pass through to the list beneath it; only the thin bar at the bottom edge is
	// interactive.
	barOverlay := container.NewBorder(nil, v.hbar, nil, nil, canvas.NewRectangle(color.Transparent))
	v.holder = container.NewStack(center, barOverlay)
}

// scrollContent routes a wheel/trackpad scroll over the diff: vertical to the
// list, horizontal to the scrollbar (whose OnScrolled re-windows the rows). The
// row forwards here because, as the innermost Scrollable, it receives the event
// instead of the list, so the vertical part must be replayed onto the list.
func (v *DiffView) scrollContent(event *fyne.ScrollEvent) {
	if event.Scrolled.DY != 0 {
		v.list.ScrollToOffset(v.list.GetScrollOffset() - event.Scrolled.DY)
	}
	if event.Scrolled.DX != 0 && v.hbar.Visible() {
		v.hbar.Scrolled(&fyne.ScrollEvent{
			PointEvent: event.PointEvent,
			Scrolled:   fyne.Delta{DX: event.Scrolled.DX, DY: 0},
		})
	}
}

// Resize lays out the view and recomputes whether the horizontal scrollbar is
// needed for the new width (a wider viewport may remove the overflow).
func (v *DiffView) Resize(size fyne.Size) {
	v.BaseWidget.Resize(size)
	v.updateHBar()
}

// onHScroll maps the scrollbar's pixel offset to a glyph-column offset and, when
// it changes, re-windows every visible row by refreshing the list.
func (v *DiffView) onHScroll(pos fyne.Position) {
	if v.metrics.advance <= 0 {
		return
	}
	col := int(math.Round(float64(pos.X / v.metrics.advance)))
	col = max(0, min(col, v.maxRunes))
	if col == v.hScrollCols {
		return
	}
	v.hScrollCols = col
	v.list.Refresh()
}

// updateHBar sizes the scrollbar spacer to the widest line and shows the bar only
// when content overflows the viewport; otherwise it hides the bar and resets the
// horizontal offset so a narrower file never stays scrolled.
func (v *DiffView) updateHBar() {
	advance := v.metrics.advance
	viewport := v.Size().Width
	contentPx := v.metrics.contentX + float32(v.maxRunes)*advance

	if advance <= 0 || viewport <= 0 || contentPx <= viewport {
		if v.hScrollCols != 0 {
			v.hScrollCols = 0
			v.list.Refresh()
		}
		v.hbar.Offset = fyne.NewPos(0, 0)
		v.hbar.Hide()

		return
	}

	v.hspacer.SetMinSize(fyne.NewSize(contentPx, hbarThickness))
	v.hspacer.Refresh()
	v.hbar.Show()
	v.hbar.Refresh()
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
	v.styleName = pal.StyleName
	v.recomputeMetrics()
	v.applyNoticeColor()

	if file == nil {
		v.showRows(nil)

		return
	}
	if !file.Loaded() {
		v.showLoading()

		return
	}
	if file.Binary {
		v.showBinary()

		return
	}

	v.showRows(v.flattenFile(file))
	v.startHighlight(file, thm, v.generation)
}

// FileLoaded renders a file whose diff finished building in the background, but
// only while it is still the selected file (pointer identity) — a result for a
// file the user navigated away from is ignored. It is the swap-in half of the
// lazy load: SetFile shows a loading placeholder for an unloaded file, and this
// replaces it with the real diff once ready. Scrolling to top here is correct,
// since this is the first render of the file's content.
func (v *DiffView) FileLoaded(file *diff.File, thm *theme.Theme) {
	if file == nil || file != v.file {
		return
	}
	v.generation++
	v.thm = thm
	pal := thm.Palette()
	v.palette = paletteFrom(&pal)
	v.styleName = pal.StyleName
	v.recomputeMetrics()
	v.applyNoticeColor()

	if file.Binary {
		v.showBinary()

		return
	}

	v.showRows(v.flattenFile(file))
	v.startHighlight(file, thm, v.generation)
}

// applyNoticeColor tints the binary and loading placeholders to the muted
// palette color so they track the active theme.
func (v *DiffView) applyNoticeColor() {
	v.binary.Color = v.palette.muted
	v.loading.Color = v.palette.muted
}

// Restyle applies a new theme's palette in place without re-flattening or
// scrolling: row backgrounds, gutters, signs, and plain text recolor from the
// palette on the next refresh. Syntax tokens are re-derived only when the chroma
// style actually changed (a light/dark flip within the same style keeps them),
// avoiding the flicker and scroll jump of a full SetFile on every theme switch.
func (v *DiffView) Restyle(thm *theme.Theme) {
	v.thm = thm
	pal := thm.Palette()
	v.palette = paletteFrom(&pal)
	v.applyNoticeColor()

	styleChanged := pal.StyleName != v.styleName
	v.styleName = pal.StyleName

	v.list.Refresh()

	if styleChanged && v.file != nil && !v.file.Binary {
		v.generation++
		v.startHighlight(v.file, thm, v.generation)
	}
}

// Relayout re-measures the monospace metrics after a font change and repositions
// the current rows in place. The row model and its syntax tokens are font
// independent, so they are preserved (no re-flatten, no re-highlight) and the
// scroll position is kept — only cell geometry updates. Callers must invalidate
// the cached glyph metrics (invalidateMonoMetrics) before calling so the new
// font is measured.
func (v *DiffView) Relayout(thm *theme.Theme) {
	v.thm = thm
	pal := thm.Palette()
	v.palette = paletteFrom(&pal)
	v.applyNoticeColor()
	v.recomputeMetrics()
	v.list.Refresh()
}

// recomputeMetrics refreshes the cached monospace layout measurements for the
// diff text size. It is the single internal caller of computeMetrics so both the
// file-load and font-relayout paths share one measurement point.
func (v *DiffView) recomputeMetrics() {
	v.metrics = computeMetrics(diffTextSize)
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

// showRows swaps in a new row model and refreshes the list, hiding the notices.
// It resets the horizontal scroll to the left and recomputes the scrollbar for
// the new content's widest line.
func (v *DiffView) showRows(rows []row) {
	v.rows = rows
	v.maxRunes = maxLineRunes(rows)
	v.hScrollCols = 0
	v.hbar.Offset = fyne.NewPos(0, 0)
	v.binary.Hide()
	v.loading.Hide()
	v.list.Show()
	v.list.Refresh()
	v.list.ScrollToTop()
	v.updateHBar()
}

// maxLineRunes returns the widest line's rune count across the rows (both columns
// in split mode), which sets the horizontal scroll range and overflow check.
func maxLineRunes(rows []row) int {
	widest := 0
	for i := range rows {
		switch rows[i].kind {
		case rowLine:
			widest = max(widest, utf8.RuneCountInString(rows[i].line.Content))
		case rowSplit:
			if rows[i].left.present {
				widest = max(widest, utf8.RuneCountInString(rows[i].left.line.Content))
			}
			if rows[i].right.present {
				widest = max(widest, utf8.RuneCountInString(rows[i].right.line.Content))
			}
		case rowSeparator:
		}
	}

	return widest
}

// showBinary clears rows and reveals the centered binary-file notice.
func (v *DiffView) showBinary() {
	v.rows = nil
	v.maxRunes = 0
	v.list.Refresh()
	v.list.Hide()
	v.loading.Hide()
	v.binary.Show()
	v.binary.Refresh()
	v.updateHBar()
}

// showLoading clears rows and reveals the centered "loading" notice while a
// selected file's diff is still being built in the background.
func (v *DiffView) showLoading() {
	v.rows = nil
	v.maxRunes = 0
	v.list.Refresh()
	v.list.Hide()
	v.binary.Hide()
	v.loading.Show()
	v.loading.Refresh()
	v.updateHBar()
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
	return formatHunkMarker(hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines, hunk.Section)
}

// formatHunkMarker renders the "@@ -a,b +c,d @@" marker plus an optional section
// heading, shared by the unified and split separator rows.
func formatHunkMarker(oldStart, oldLines, newStart, newLines int, section string) string {
	marker := fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldLines, newStart, newLines)
	if section == "" {
		return marker
	}

	return marker + " " + section
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
	return formatHunkMarker(split.OldStart, split.OldLines, split.NewStart, split.NewLines, split.Section)
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
