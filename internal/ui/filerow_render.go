package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"github.com/omarluq/diffdiff/internal/diff"
	"github.com/omarluq/diffdiff/internal/icons"
)

// fileRowRenderer lays out a file-list row left to right: status glyph, adds,
// dels, then the path drawn one canvas.Text per rune so directory, basename,
// and fuzzy-matched characters can each take their own color and weight. The
// per-rune objects are rebuilt on each set since the path varies.
type fileRowRenderer struct {
	row      *fileRow
	icon     *canvas.Image
	glyph    *canvas.Text
	adds     *canvas.Text
	dels     *canvas.Text
	segments []*canvas.Text
	height   float32
}

// Destroy has nothing to release.
func (r *fileRowRenderer) Destroy() {}

// MinSize reports one text line's height; width is supplied by the list.
func (r *fileRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, r.height)
}

// Layout is a no-op: positions are absolute and assigned in Refresh.
func (r *fileRowRenderer) Layout(_ fyne.Size) {}

// Refresh repaints the row for the current entry, rebuilding the chrome colors
// and the per-rune path objects.
func (r *fileRowRenderer) Refresh() {
	if !r.row.hasData || r.row.entry.file == nil {
		return
	}
	file := r.row.entry.file
	pal := r.row.palette
	advance := r.row.advance

	r.layoutIcon(file.Path)
	cursor := r.layoutChrome(file, pal, advance)
	r.layoutPath(file.Path, pal, advance, cursor)
	canvas.Refresh(r.row)
}

// layoutIcon places the Material file-type icon as a square at the row's left
// edge, sized to the line height, picking the light variant on light themes.
func (r *fileRowRenderer) layoutIcon(filePath string) {
	r.icon.Resource = icons.For(filePath, r.row.palette.dark)
	r.icon.Resize(fyne.NewSize(r.height, r.height))
	r.icon.Move(fyne.NewPos(0, 0))
	r.icon.Refresh()
}

// layoutChrome positions the status glyph and the +adds/-dels counts, returning
// the x cursor (in pixels) where the path should begin.
func (r *fileRowRenderer) layoutChrome(file *diff.File, pal palette, advance float32) float32 {
	adds, dels := countsLabels(file)

	// lead reserves a square at the row's left edge for the file-type icon.
	lead := r.height

	r.glyph.Text = statusGlyph(file.Status)
	r.glyph.Color = statusColor(pal, file.Status)
	r.glyph.TextSize = fileRowTextSize
	r.glyph.Move(fyne.NewPos(lead+advance, 0))

	addsX := lead + advance*(1+glyphGap+1)
	r.adds.Text = adds
	r.adds.Color = pal.addEmph
	r.adds.Move(fyne.NewPos(addsX, 0))

	delsX := addsX + float32(len(adds)+glyphGap)*advance
	r.dels.Text = dels
	r.dels.Color = pal.delEmph
	r.dels.Move(fyne.NewPos(delsX, 0))

	return delsX + float32(len(dels)+glyphGap*2)*advance
}

// layoutPath draws the path as runs of canvas.Text — one per maximal span of
// runes sharing a color and weight: the directory is muted, the basename
// foreground, and fuzzy-matched runes accented and bold. Grouping runs (instead
// of one object per rune) keeps the object count low and sidesteps a Fyne GL
// quirk where a lone "_" text object renders blank. A column counter drives
// positioning so multi-byte runes still advance one monospace cell each.
func (r *fileRowRenderer) layoutPath(filePath string, pal palette, advance, startX float32) {
	r.segments = r.segments[:0]
	dirLen := strings.LastIndexByte(filePath, '/') + 1

	// In basename-only mode (a nested-tree leaf) render just the base name;
	// offsetBase keeps fuzzy-match lookups aligned to the full-path offsets the
	// matcher recorded.
	render, offsetBase := filePath, 0
	if r.row.basenameOnly {
		render, offsetBase = filePath[dirLen:], dirLen
	}

	var (
		buf      strings.Builder
		runColor color.NRGBA
		runBold  bool
		runStart int
		haveRun  bool
	)
	col := 0

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		txt := r.segmentText(buf.String(), runColor, runBold)
		txt.Move(fyne.NewPos(startX+float32(runStart)*advance, 0))
		r.segments = append(r.segments, txt)
		buf.Reset()
	}

	for byteOffset, rch := range render {
		full := offsetBase + byteOffset
		emphasized := r.row.entry.matched[full]
		c := runeColor(pal, full >= dirLen, emphasized)
		if !haveRun || c != runColor || emphasized != runBold {
			flush()
			runColor, runBold, runStart, haveRun = c, emphasized, col, true
		}
		buf.WriteRune(rch)
		col++
	}
	flush()
}

// segmentText builds a monospace text run in the given color and weight.
func (r *fileRowRenderer) segmentText(content string, textColor color.NRGBA, bold bool) *canvas.Text {
	txt := canvas.NewText(content, textColor)
	txt.TextSize = fileRowTextSize
	style := monoStyle()
	style.Bold = bold
	txt.TextStyle = style

	return txt
}

// runeColor chooses a path rune's color: accent for a fuzzy match, foreground
// for the basename, muted for the leading directory.
func runeColor(pal palette, inBasename, matched bool) color.NRGBA {
	if matched {
		return pal.accent
	}
	if inBasename {
		return pal.foreground
	}

	return pal.muted
}

// Objects returns the row's drawables in paint order.
func (r *fileRowRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, 4+len(r.segments))
	objs = append(objs, r.icon, r.glyph, r.adds, r.dels)
	for _, txt := range r.segments {
		objs = append(objs, txt)
	}

	return objs
}
