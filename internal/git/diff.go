package git

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/samber/oops"

	"github.com/omarluq/diffdiff/internal/diff"
)

// binarySniffLen is how many leading bytes we scan for a NUL byte when deciding
// whether a working-tree file is binary.
const binarySniffLen = 8000

// ignoredStatus is the synthetic git status assigned to ignored files surfaced
// by the showIgnored option; git's own Status never reports them.
var ignoredStatus = &git.FileStatus{Staging: git.Untracked, Worktree: git.Untracked, Extra: ""}

// WorkingDiff returns the diff of the working tree against HEAD (equivalent to
// `git diff HEAD`): every staged and unstaged change plus untracked files.
// Gitignored files are excluded unless showIgnored is true, in which case the
// worktree is scanned for ignored, untracked files which are appended as
// additions. Files are returned sorted by path.
func (r *Repository) WorkingDiff(showIgnored bool) ([]*diff.File, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return nil, oops.In("git").Code("worktree").Wrapf(err, "resolve worktree")
	}

	status, err := wt.Status()
	if err != nil {
		return nil, oops.In("git").Code("status").Wrapf(err, "compute status")
	}

	head, err := r.headCommit()
	if err != nil {
		return nil, err
	}

	if showIgnored {
		return r.workingDiffWithIgnored(wt, head, status)
	}

	entries := selectEntries(status, r.supplementalMatcher(), false)

	return r.buildFiles(head, entries)
}

// workingDiffWithIgnored builds the diff including ignored files, using the full
// matcher (repository .gitignore plus global/system excludes) to both keep
// ignored entries and scan the worktree for ignored files Status omits.
func (r *Repository) workingDiffWithIgnored(
	wt *git.Worktree, head *object.Commit, status git.Status,
) ([]*diff.File, error) {
	matcher, err := r.fullMatcher(wt)
	if err != nil {
		return nil, err
	}

	entries := selectEntries(status, matcher, true)
	if err := r.addIgnored(wt, matcher, entries); err != nil {
		return nil, err
	}

	return r.buildFiles(head, entries)
}

// selectEntries chooses the status entries to render: every tracked change, plus
// untracked files unless they are ignored and showIgnored is false. go-git's
// Status only filters repository .gitignore, so untracked files are re-checked
// against the full matcher (which also covers global and system excludes).
func selectEntries(status git.Status, matcher gitignore.Matcher, showIgnored bool) map[string]*git.FileStatus {
	entries := make(map[string]*git.FileStatus, len(status))
	for path, fs := range status {
		if fs.Staging == git.Unmodified && fs.Worktree == git.Unmodified {
			continue
		}
		if !showIgnored && fs.Worktree == git.Untracked && matcher.Match(strings.Split(path, "/"), false) {
			continue
		}
		entries[path] = fs
	}

	return entries
}

// buildFiles materializes the diff for each entry, sorted by path, dropping any
// that resolve to no renderable change.
func (r *Repository) buildFiles(head *object.Commit, entries map[string]*git.FileStatus) ([]*diff.File, error) {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	files := make([]*diff.File, 0, len(paths))
	for _, path := range paths {
		file, err := r.buildFile(head, path, entries[path])
		if err != nil {
			return nil, err
		}
		if file != nil {
			files = append(files, file)
		}
	}

	return files, nil
}

// addIgnored scans the worktree for ignored, untracked files (those git's own
// Status omits) and records them in entries as additions. Tracked files and
// paths already in entries are left untouched; the .git directory is skipped.
func (r *Repository) addIgnored(wt *git.Worktree, matcher gitignore.Matcher, entries map[string]*git.FileStatus) error {
	tracked, err := r.trackedSet()
	if err != nil {
		return err
	}

	scan := &ignoreScan{matcher: matcher, tracked: tracked, entries: entries}
	if err := util.Walk(wt.Filesystem, "/", scan.visit); err != nil {
		return oops.In("git").Code("walk_ignored").Wrapf(err, "scan ignored files")
	}

	return nil
}

// ignoreScan carries the state for a worktree walk that records ignored,
// untracked files.
type ignoreScan struct {
	matcher gitignore.Matcher
	tracked map[string]bool
	entries map[string]*git.FileStatus
}

// visit is a filepath.WalkFunc that records a path when it is an ignored,
// untracked file. Unreadable entries are skipped so one error cannot abort the
// whole scan.
func (s *ignoreScan) visit(path string, info os.FileInfo, err error) error {
	if err != nil {
		return nil //nolint:nilerr // skip an unreadable entry; do not abort the scan
	}

	rel := strings.Trim(filepath.ToSlash(path), "/")
	if rel == "" {
		return nil
	}

	components := strings.Split(rel, "/")
	if info.IsDir() {
		// Never descend into .git, and prune ignored directory trees entirely
		// (like `git status --ignored`): surfacing every file inside, say,
		// node_modules or a build cache would be useless and slow.
		if filepath.Base(rel) == ".git" || s.matcher.Match(components, true) {
			return filepath.SkipDir
		}

		return nil
	}

	if _, seen := s.entries[rel]; seen || s.tracked[rel] {
		return nil
	}
	if s.matcher.Match(components, false) {
		s.entries[rel] = ignoredStatus
	}

	return nil
}

// trackedSet returns the set of index-tracked paths so the ignored-file scan can
// skip files git already follows.
func (r *Repository) trackedSet() (map[string]bool, error) {
	idx, err := r.repo.Storer.Index()
	if err != nil {
		return nil, oops.In("git").Code("index").Wrapf(err, "read index")
	}

	tracked := make(map[string]bool, len(idx.Entries))
	for _, entry := range idx.Entries {
		tracked[entry.Name] = true
	}

	return tracked, nil
}

// headCommit returns the commit at HEAD, or nil if the repository has no
// commits yet (an unborn HEAD), in which case everything is treated as added.
func (r *Repository) headCommit() (*object.Commit, error) {
	ref, err := r.repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, nil //nolint:nilnil // nil commit signals an unborn HEAD
		}
		return nil, oops.In("git").Code("head").Wrapf(err, "resolve HEAD")
	}

	commit, err := r.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, oops.In("git").Code("head_commit").Wrapf(err, "load HEAD commit")
	}

	return commit, nil
}

// buildFile assembles the diff for a single changed path. It returns nil when
// the path turns out to have no renderable change (e.g. a metadata-only delta).
func (r *Repository) buildFile(head *object.Commit, path string, fs *git.FileStatus) (*diff.File, error) {
	oldPath := resolveOldPath(fs, path)

	oldContent, oldBinary, oldExists, err := readHeadFile(head, oldPath)
	if err != nil {
		return nil, err
	}

	newContent, newBinary, newExists, err := r.readWorkFile(path)
	if err != nil {
		return nil, err
	}

	file := &diff.File{
		Path:     strings.Clone(path),
		OldPath:  strings.Clone(oldPath),
		Status:   classify(fs, oldExists, newExists, oldPath != path),
		Language: "",
		Binary:   oldBinary || newBinary,
		Hunks:    nil,
		Added:    0,
		Deleted:  0,
	}

	if file.Binary {
		return file, nil
	}

	hunks, added, deleted, err := diff.BuildHunks(oldContent, newContent)
	if err != nil {
		return nil, oops.In("git").Code("build_hunks").With("path", path).Wrapf(err, "diff file")
	}

	// A tracked file flagged as modified but with identical content (e.g. a
	// staged change reverted in the worktree) yields no hunks — drop it.
	if len(hunks) == 0 && file.Status == diff.StatusModified {
		return nil, nil //nolint:nilnil // nil file signals "no renderable change"
	}

	file.Hunks = hunks
	file.Added = added
	file.Deleted = deleted

	return file, nil
}

// resolveOldPath returns the path a file had on the old side of the diff,
// following a rename's recorded source when git reports one.
func resolveOldPath(fs *git.FileStatus, path string) string {
	if fs.Extra == "" {
		return path
	}

	if fs.Staging == git.Renamed || fs.Worktree == git.Renamed {
		return fs.Extra
	}

	return path
}

// classify maps git status plus content presence to a diff status.
func classify(fs *git.FileStatus, oldExists, newExists, renamed bool) diff.Status {
	switch {
	case fs.Worktree == git.Untracked:
		return diff.StatusUntracked
	case !oldExists && newExists:
		return diff.StatusAdded
	case oldExists && !newExists:
		return diff.StatusDeleted
	case renamed:
		return diff.StatusRenamed
	default:
		return diff.StatusModified
	}
}

// readHeadFile reads a blob from the HEAD commit. It reports existence and
// whether the blob is binary; binary blobs return empty content.
func readHeadFile(head *object.Commit, path string) (content string, binary, exists bool, err error) {
	if head == nil {
		return "", false, false, nil
	}

	f, err := head.File(path)
	if err != nil {
		if errors.Is(err, object.ErrFileNotFound) || errors.Is(err, object.ErrDirectoryNotFound) {
			return "", false, false, nil
		}
		return "", false, false, oops.In("git").Code("head_file").With("path", path).Wrapf(err, "read HEAD blob")
	}

	binary, err = f.IsBinary()
	if err != nil {
		return "", false, false, oops.In("git").Code("is_binary").With("path", path).Wrapf(err, "detect binary")
	}
	if binary {
		return "", true, true, nil
	}

	content, err = f.Contents()
	if err != nil {
		return "", false, false, oops.In("git").Code("contents").With("path", path).Wrapf(err, "read blob contents")
	}

	return content, false, true, nil
}

// readWorkFile reads a file from the working tree. It reports existence and
// whether the bytes look binary; binary files return empty content.
func (r *Repository) readWorkFile(path string) (content string, binary, exists bool, err error) {
	data, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(path)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, false, nil
		}
		return "", false, false, oops.In("git").Code("read_work").With("path", path).Wrapf(err, "read working file")
	}

	if isBinary(data) {
		return "", true, true, nil
	}

	return string(data), false, true, nil
}

// isBinary reports whether data looks binary by sniffing for a NUL byte in the
// leading bytes — the same cheap heuristic git itself uses.
func isBinary(data []byte) bool {
	if len(data) > binarySniffLen {
		data = data[:binarySniffLen]
	}
	return bytes.IndexByte(data, 0) >= 0
}
