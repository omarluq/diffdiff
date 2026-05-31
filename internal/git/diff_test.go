package git_test

import (
	"os"
	"path/filepath"
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

func TestWorkingDiffClean(t *testing.T) {
	t.Parallel()

	dir := initRepo(t, "a.txt", "stable\n")

	repo, err := ourgit.Open(dir)
	require.NoError(t, err)

	files, err := repo.WorkingDiff()
	require.NoError(t, err)
	assert.Empty(t, files)
}
