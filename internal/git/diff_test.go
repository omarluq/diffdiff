package git_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/diff"
	ourgit "github.com/omarluq/diffdiff/internal/git"
)

// initRepo creates a temporary repository with a single committed file and
// returns the repository root.
func initRepo(t *testing.T, name, content string) string {
	t.Helper()

	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	_, err = wt.Add(name)
	require.NoError(t, err)

	_, err = wt.Commit("init", &gogit.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@example.com", When: time.Unix(0, 0)},
	})
	require.NoError(t, err)

	return dir
}

func TestWorkingDiffModified(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "a.txt", "one\ntwo\nthree\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n2\nthree\n"), 0o600))

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	files, err := repo.WorkingDiff()
	require.NoError(t, err)
	require.Len(t, files, 1)

	f := files[0]
	assert.Equal(t, "a.txt", f.Path)
	assert.Equal(t, diff.StatusModified, f.Status)
	assert.Equal(t, 1, f.Added)
	assert.Equal(t, 1, f.Deleted)
	assert.NotEmpty(t, f.Hunks)
}

func TestWorkingDiffUntracked(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "a.txt", "kept\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("brand\nnew\n"), 0o600))

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	files, err := repo.WorkingDiff()
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "new.txt", files[0].Path)
	assert.Equal(t, diff.StatusUntracked, files[0].Status)
	assert.Equal(t, 2, files[0].Added)
}

func TestWorkingDiffDeleted(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "gone.txt", "a\nb\nc\n")
	require.NoError(t, os.Remove(filepath.Join(dir, "gone.txt")))

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	files, err := repo.WorkingDiff()
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, diff.StatusDeleted, files[0].Status)
	assert.Equal(t, 3, files[0].Deleted)
}

// TestWorkingDiffExcludesIgnored confirms a repository .gitignore'd file never
// appears in the diff; only the new .gitignore itself is a visible change.
func TestWorkingDiffExcludesIgnored(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "keep.go", "package a\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "debug.log"), []byte("noise\n"), 0o600))

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	files, err := repo.WorkingDiff()
	require.NoError(t, err)
	for _, f := range files {
		assert.NotEqual(t, "debug.log", f.Path, "ignored file must never appear")
	}
}

// TestWorkingDiffRespectsGlobalIgnore guards the bug where files ignored only
// via the global excludes (e.g. ~/.config/git/ignore) leaked in as untracked,
// because go-git's Status honors just the repository .gitignore.
func TestWorkingDiffRespectsGlobalIgnore(t *testing.T) {
	xdg := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(xdg, "git"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(xdg, "git", "ignore"), []byte("*.secret\n"), 0o600))
	t.Setenv("XDG_CONFIG_HOME", xdg)

	dir := initRepo(t, "keep.go", "package a\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "creds.secret"), []byte("token\n"), 0o600))

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	files, err := repo.WorkingDiff()
	require.NoError(t, err)
	for _, f := range files {
		assert.NotEqual(t, "creds.secret", f.Path, "globally-ignored file must never appear")
	}
}

func TestDetails(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "a.txt", "hi\n")

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	details := repo.Details()
	assert.NotEmpty(t, details.Branch, "a committed repo reports its branch")
	assert.Len(t, details.Head, 7, "head is a short hash")
	assert.Equal(t, "init", details.Subject)
}

func TestWorkingDiffClean(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "a.txt", "stable\n")

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	files, err := repo.WorkingDiff()
	require.NoError(t, err)
	assert.Empty(t, files)
}

// TestChangedFilesAreUnloaded verifies the cheap scan returns the changed file
// with its path and status but no diff content, so the UI can paint the list
// before any blob is read or diffed.
func TestChangedFilesAreUnloaded(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "a.txt", "one\ntwo\nthree\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n2\nthree\n"), 0o600))

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	working, files, err := repo.ChangedFiles()
	require.NoError(t, err)
	require.NotNil(t, working)
	require.Len(t, files, 1)

	f := files[0]
	assert.Equal(t, "a.txt", f.Path)
	assert.False(t, f.Loaded(), "the cheap scan yields an unloaded file")
	assert.Empty(t, f.Hunks, "no hunks are built before load")
	assert.Zero(t, f.Added)
	assert.Zero(t, f.Deleted)
}

// TestLoadFileMaterializesAndIsIdempotent verifies LoadFile builds the hunks and
// counts in place and that a second call does not rebuild or change them.
func TestLoadFileMaterializesAndIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "a.txt", "one\ntwo\nthree\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n2\nthree\n"), 0o600))

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	working, files, err := repo.ChangedFiles()
	require.NoError(t, err)
	require.Len(t, files, 1)
	f := files[0]

	require.NoError(t, working.LoadFile(f))
	assert.True(t, f.Loaded())
	assert.Equal(t, diff.StatusModified, f.Status)
	assert.Equal(t, 1, f.Added)
	assert.Equal(t, 1, f.Deleted)
	require.NotEmpty(t, f.Hunks)

	hunks, added, deleted := len(f.Hunks), f.Added, f.Deleted
	require.NoError(t, working.LoadFile(f), "a second load is a no-op")
	assert.Len(t, f.Hunks, hunks)
	assert.Equal(t, added, f.Added)
	assert.Equal(t, deleted, f.Deleted)
}

// TestLoadFileConcurrent loads every file from many goroutines at once; with the
// race detector enabled this guards that concurrent (and duplicate) loads do not
// race on a file's fields.
func TestLoadFileConcurrent(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "a.txt", "one\ntwo\nthree\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n2\nthree\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("brand\nnew\n"), 0o600))

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	working, files, err := repo.ChangedFiles()
	require.NoError(t, err)
	require.NotEmpty(t, files)

	var group sync.WaitGroup
	for _, f := range files {
		for range 8 {
			group.Add(1)
			go func(file *diff.File) {
				defer group.Done()
				assert.NoError(t, working.LoadFile(file))
			}(f)
		}
	}
	group.Wait()

	for _, f := range files {
		assert.True(t, f.Loaded(), "every file is loaded after the concurrent sweep")
	}
}
