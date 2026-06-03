// Package diff defines the rendering-oriented diff data model — files, hunks,
// and lines — built from old/new content with a Myers diff (go-udiff).
//
// The model deliberately mirrors the structure Pierre describes in
// "On rendering diffs": parsed line content plus hunk metadata, with substrings
// detached from their source blob (via strings.Clone) so a rendered diff never
// pins the original file contents in memory.
package diff

import "github.com/samber/lo"

// LineKind classifies a single diff line.
type LineKind uint8

const (
	// LineContext is an unchanged line shown for surrounding context.
	LineContext LineKind = iota
	// LineAdded exists only in the new version of the file.
	LineAdded
	// LineDeleted exists only in the old version of the file.
	LineDeleted
)

// Status is the kind of change applied to a file in a diff.
type Status uint8

const (
	// StatusModified means the file exists on both sides with content changes.
	StatusModified Status = iota
	// StatusAdded means the file is new.
	StatusAdded
	// StatusDeleted means the file was removed.
	StatusDeleted
	// StatusRenamed means the file moved path (possibly with content changes).
	StatusRenamed
	// StatusUntracked means the file is new and not yet tracked by git.
	StatusUntracked
)

// LoadState tracks whether a file's diff content has been built. The file list
// can be shown from a cheap status-only scan (Unloaded) and each file's hunks and
// counts filled in lazily (Loaded) without blocking the first paint.
type LoadState uint8

const (
	// StateUnloaded means only the path and status are known; Hunks and the
	// Added/Deleted counts are not yet computed.
	StateUnloaded LoadState = iota
	// StateLoaded means the diff content has been built: Hunks and counts are
	// authoritative (including a legitimately empty diff).
	StateLoaded
)

// String renders a short status label.
func (s Status) String() string {
	switch s {
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusUntracked:
		return "untracked"
	case StatusModified:
		return "modified"
	default:
		return "modified"
	}
}

// Segment is a contiguous run of text within a line. Intraline marks the run as
// part of a within-line (word/character-level) change relative to its
// counterpart on the other side of the diff; it is an overlay independent of
// syntax color, which is computed separately by the highlighter.
type Segment struct {
	Text      string
	Intraline bool
}

// Line is a single rendered diff line.
//
// OldNum/NewNum are 1-based line numbers in the old and new files; the number
// that does not apply to this line's kind is 0. Segments is nil when the whole
// line is unchanged at the sub-line level; when populated it partitions Content
// into intra-line change runs.
type Line struct {
	Kind     LineKind
	OldNum   int
	NewNum   int
	Content  string
	Segments []Segment
}

// Hunk is a contiguous block of changed lines plus surrounding context.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	// Section is the function/context heading git places after the "@@" marker
	// (e.g. the enclosing function signature); empty when unavailable.
	Section string
	Lines   []Line
}

// File is the complete diff for a single file.
type File struct {
	// Path is the new path, or the old path for deletions.
	Path string
	// OldPath is set for renames; otherwise it equals Path.
	OldPath string
	Status  Status
	// Language is the resolved highlighter language token, or "" if unknown.
	Language string
	Binary   bool
	Hunks    []Hunk
	Added    int
	Deleted  int
	// State reports whether Hunks and the counts have been built yet. A file from
	// the cheap working-tree scan starts Unloaded and becomes Loaded once its diff
	// is materialized on demand.
	State LoadState
}

// IsRename reports whether the file moved path.
func (f *File) IsRename() bool {
	return f.OldPath != "" && f.OldPath != f.Path
}

// Loaded reports whether the file's diff content (hunks and counts) has been
// built. While false the file carries only its path and a provisional status.
func (f *File) Loaded() bool {
	return f.State == StateLoaded
}

// HasCounts reports whether the Added/Deleted counts are authoritative, which is
// exactly when the file has been loaded. The file list uses this to show a
// placeholder until the counts stream in.
func (f *File) HasCounts() bool {
	return f.Loaded()
}

// TotalLines returns the number of rendered diff lines across all hunks. It is
// the basis for virtualized layout height estimation.
func (f *File) TotalLines() int {
	return lo.SumBy(f.Hunks, func(h Hunk) int { return len(h.Lines) })
}
