package git

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/samber/oops"
	"golang.org/x/sync/singleflight"

	"github.com/omarluq/diffdiff/internal/diff"
)

// binarySniffLen is how many leading bytes we scan for a NUL byte when deciding
// whether a working-tree file is binary.
const binarySniffLen = 8000

// WorkingSet is a lazily-materialized view of the working-tree changes against
// HEAD. ChangedFiles produces the file list cheaply (paths + provisional status,
// no blob reads or diffing); LoadFile then builds one file's hunks and counts on
// demand. LoadFile is safe to call concurrently from multiple goroutines.
type WorkingSet struct {
	repo    *Repository
	head    mo.Option[*object.Commit]
	entries map[string]*git.FileStatus
	flight  singleflight.Group
	mu      sync.Mutex
	loaded  map[string]bool
}

// ChangedFiles scans the working tree against HEAD and returns the changed files
// with only their path and a provisional status populated — Hunks nil, counts
// zero, State Unloaded. It performs no blob reads or diffing, so it is cheap
// enough to run before the first paint; call WorkingSet.LoadFile to build a
// file's diff on demand. Gitignored files are always excluded (including those
// ignored only via .git/info/exclude or the global/system excludes, which
// go-git's Status does not filter on its own). Files are sorted by path.
func (r *Repository) ChangedFiles() (*WorkingSet, []*diff.File, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return nil, nil, oops.In("git").Code("worktree").Wrapf(err, "resolve worktree")
	}

	status, err := wt.Status()
	if err != nil {
		return nil, nil, oops.In("git").Code("status").Wrapf(err, "compute status")
	}

	head, err := r.headCommit()
	if err != nil {
		return nil, nil, err
	}

	entries := selectEntries(status, r.supplementalMatcher())
	working := &WorkingSet{
		repo:    r,
		head:    head,
		entries: entries,
		flight:  singleflight.Group{},
		mu:      sync.Mutex{},
		loaded:  make(map[string]bool, len(entries)),
	}

	return working, listFiles(entries), nil
}

// listFiles builds the sorted, status-only file list for the cheap scan. Each
// file is Unloaded: its path and a provisional (blob-free) status are known, but
// its hunks and counts are not yet computed.
func listFiles(entries map[string]*git.FileStatus) []*diff.File {
	paths := lo.Keys(entries)
	sort.Strings(paths)

	files := make([]*diff.File, 0, len(paths))
	for _, path := range paths {
		fs := entries[path]
		files = append(files, &diff.File{
			Path:     strings.Clone(path),
			OldPath:  strings.Clone(resolveOldPath(fs, path)),
			Status:   classifyFromStatus(fs),
			Language: "",
			Binary:   false,
			Hunks:    nil,
			Added:    0,
			Deleted:  0,
			State:    diff.StateUnloaded,
		})
	}

	return files
}

// WorkingDiff returns the diff of the working tree against HEAD (equivalent to
// `git diff HEAD`): every staged and unstaged change plus untracked files, each
// fully materialized. It is the eager convenience wrapper over ChangedFiles +
// LoadFile; the GUI uses the lazy pair directly. A modified file that resolves to
// no textual change is dropped, matching `git diff`. Files are sorted by path.
func (r *Repository) WorkingDiff() ([]*diff.File, error) {
	working, files, err := r.ChangedFiles()
	if err != nil {
		return nil, err
	}

	out := make([]*diff.File, 0, len(files))
	for _, file := range files {
		if err := working.LoadFile(file); err != nil {
			return nil, err
		}
		if len(file.Hunks) == 0 && file.Status == diff.StatusModified && !file.Binary {
			continue // a staged change reverted in the worktree: no renderable diff
		}
		out = append(out, file)
	}

	return out, nil
}

// selectEntries chooses the status entries to render: every tracked change, plus
// untracked files that are not ignored. go-git's Status only filters the
// repository .gitignore, so untracked files are re-checked against matcher
// (which also covers .git/info/exclude and the global/system excludes).
func selectEntries(status git.Status, matcher gitignore.Matcher) map[string]*git.FileStatus {
	entries := make(map[string]*git.FileStatus, len(status))
	for path, fs := range status {
		if fs.Staging == git.Unmodified && fs.Worktree == git.Unmodified {
			continue
		}
		if fs.Worktree == git.Untracked && matcher.Match(strings.Split(path, "/"), false) {
			continue
		}
		entries[path] = fs
	}

	return entries
}

// LoadFile builds the diff content for a single file — hunks, counts, the real
// binary flag, and the blob-corrected status — mutating the file in place and
// marking it loaded. It is idempotent and safe to call concurrently: concurrent
// calls for the same file are de-duplicated (singleflight) so the file's fields
// are written by exactly one goroutine, and an already-loaded file returns
// immediately. Callers must publish the file to the UI only after LoadFile
// returns, so the writes are visible without further synchronization.
func (ws *WorkingSet) LoadFile(file *diff.File) error {
	ws.mu.Lock()
	done, present := ws.loaded[file.Path], ws.entries[file.Path]
	ws.mu.Unlock()
	if done || present == nil {
		return nil // already built, or not part of this working set
	}

	// The shared value is unused (callers want only the error); return a non-nil
	// sentinel so the flight closure never returns (nil, nil).
	sentinel := struct{}{}
	_, err, _ := ws.flight.Do(file.Path, func() (any, error) {
		ws.mu.Lock()
		already := ws.loaded[file.Path]
		ws.mu.Unlock()
		if already {
			return sentinel, nil
		}

		if buildErr := ws.repo.materialize(ws.head, file, present); buildErr != nil {
			return sentinel, buildErr
		}

		ws.mu.Lock()
		ws.loaded[file.Path] = true
		ws.mu.Unlock()

		return sentinel, nil
	})

	return err
}

// classifyFromStatus derives a diff status from git status codes alone, with no
// blob reads, for the cheap scan. It is a best guess that materialize later
// corrects with blob-existence information (see classify).
func classifyFromStatus(fs *git.FileStatus) diff.Status {
	switch {
	case fs.Worktree == git.Untracked:
		return diff.StatusUntracked
	case fs.Staging == git.Added:
		return diff.StatusAdded
	case fs.Staging == git.Deleted || fs.Worktree == git.Deleted:
		return diff.StatusDeleted
	case fs.Staging == git.Renamed || fs.Worktree == git.Renamed:
		return diff.StatusRenamed
	default:
		return diff.StatusModified
	}
}

// headCommit returns the commit at HEAD, or None if the repository has no
// commits yet (an unborn HEAD), in which case everything is treated as added.
func (r *Repository) headCommit() (mo.Option[*object.Commit], error) {
	ref, err := r.repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return mo.None[*object.Commit](), nil // unborn HEAD: no commit yet
		}
		return mo.None[*object.Commit](), oops.In("git").Code("head").Wrapf(err, "resolve HEAD")
	}

	commit, err := r.repo.CommitObject(ref.Hash())
	if err != nil {
		return mo.None[*object.Commit](), oops.In("git").Code("head_commit").Wrapf(err, "load HEAD commit")
	}

	return mo.Some(commit), nil
}

// materialize fills in a file's diff content in place: it reads the old (HEAD)
// and new (working) blobs, sets the real binary flag and the blob-corrected
// status, builds the hunks and line counts, and marks the file loaded. A binary
// file is recorded with no hunks. Unlike the old eager path it never drops a
// "no renderable change" file — the row already exists, so it stays as a loaded
// zero-count entry (WorkingDiff applies the drop for its eager callers).
func (r *Repository) materialize(head mo.Option[*object.Commit], file *diff.File, fs *git.FileStatus) error {
	oldPath := resolveOldPath(fs, file.Path)

	oldContent, oldBinary, oldExists, err := readHeadFile(head, oldPath)
	if err != nil {
		return err
	}

	newContent, newBinary, newExists, err := r.readWorkFile(file.Path)
	if err != nil {
		return err
	}

	file.Status = classify(fs, oldExists, newExists, oldPath != file.Path)
	file.Binary = oldBinary || newBinary
	if file.Binary {
		file.Hunks = nil
		file.Added = 0
		file.Deleted = 0
		file.State = diff.StateLoaded

		return nil
	}

	hunks, added, deleted, err := diff.BuildHunks(oldContent, newContent)
	if err != nil {
		return oops.In("git").Code("build_hunks").With("path", file.Path).Wrapf(err, "diff file")
	}

	file.Hunks = hunks
	file.Added = added
	file.Deleted = deleted
	file.State = diff.StateLoaded

	return nil
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
// whether the blob is binary; binary blobs return empty content. A None head
// (unborn HEAD) means every path is absent on the old side.
func readHeadFile(head mo.Option[*object.Commit], path string) (content string, binary, exists bool, err error) {
	commit, ok := head.Get()
	if !ok {
		return "", false, false, nil
	}

	f, err := commit.File(path)
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
