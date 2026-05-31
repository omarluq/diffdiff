package recents_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/diffdiff/internal/recents"
)

// repoAlpha is reused across cases to keep literal paths consistent.
const repoAlpha = "/repos/alpha"

func TestAddPrependsAndDeduplicates(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recent.json")
	store, err := recents.NewAt(path, 5)
	require.NoError(t, err)

	require.NoError(t, store.Add(repoAlpha))
	require.NoError(t, store.Add("/repos/beta"))
	require.NoError(t, store.Add("/repos/gamma"))

	assert.Equal(t, []string{"/repos/gamma", "/repos/beta", repoAlpha}, store.List())

	require.NoError(t, store.Add(repoAlpha))

	got := store.List()
	assert.Equal(t, []string{repoAlpha, "/repos/gamma", "/repos/beta"}, got)
	assert.Len(t, got, 3, "re-adding an existing path must not grow the list")
}

func TestAddEnforcesCap(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recent.json")
	store, err := recents.NewAt(path, 3)
	require.NoError(t, err)

	for _, repo := range []string{"/r/1", "/r/2", "/r/3", "/r/4", "/r/5"} {
		require.NoError(t, store.Add(repo))
	}

	got := store.List()
	require.Len(t, got, 3)
	assert.Equal(t, []string{"/r/5", "/r/4", "/r/3"}, got)
}

func TestNonPositiveMaxDefaultsToFive(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recent.json")
	store, err := recents.NewAt(path, 0)
	require.NoError(t, err)

	for i := range 7 {
		require.NoError(t, store.Add(fmt.Sprintf("/r/%d", i)))
	}

	assert.Len(t, store.List(), 5)
}

func TestPersistenceRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recent.json")

	first, err := recents.NewAt(path, 5)
	require.NoError(t, err)
	require.NoError(t, first.Add("/repos/one"))
	require.NoError(t, first.Add("/repos/two"))
	require.NoError(t, first.Add("/repos/three"))

	second, err := recents.NewAt(path, 5)
	require.NoError(t, err)

	assert.Equal(t, []string{"/repos/three", "/repos/two", "/repos/one"}, second.List())
}

func TestNewAtMissingFileIsEmptyNoError(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	store, err := recents.NewAt(path, 5)
	require.NoError(t, err)
	assert.Empty(t, store.List())
}

func TestListReturnsCopy(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recent.json")
	store, err := recents.NewAt(path, 5)
	require.NoError(t, err)
	require.NoError(t, store.Add(repoAlpha))

	got := store.List()
	require.Len(t, got, 1)
	got[0] = "MUTATED"

	assert.Equal(t, []string{repoAlpha}, store.List(), "mutating the returned slice must not affect the store")
}

func TestRelativePathStoredAbsolute(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recent.json")
	store, err := recents.NewAt(path, 5)
	require.NoError(t, err)

	require.NoError(t, store.Add("./some/rel/dir"))

	got := store.List()
	require.Len(t, got, 1)
	assert.True(t, filepath.IsAbs(got[0]), "stored path must be absolute, got %q", got[0])
	assert.Equal(t, got[0], filepath.Clean(got[0]), "stored path must be cleaned")
}

func TestConcurrentAddStaysConsistentAndCapped(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recent.json")
	store, err := recents.NewAt(path, 5)
	require.NoError(t, err)

	const workers = 16

	errs := make([]error, workers)

	var group sync.WaitGroup

	group.Add(workers)

	for i := range workers {
		go func(n int) {
			defer group.Done()

			for j := range 20 {
				if addErr := store.Add(fmt.Sprintf("/r/%d/%d", n, j%10)); addErr != nil {
					errs[n] = addErr
				}
			}
		}(i)
	}

	group.Wait()

	for _, addErr := range errs {
		require.NoError(t, addErr)
	}

	got := store.List()
	assert.LessOrEqual(t, len(got), 5, "list must remain capped under concurrent writes")
	assert.Len(t, uniq(got), len(got), "list must contain no duplicates")
}

// uniq returns the distinct elements of in, preserving order.
func uniq(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))

	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}

		out = append(out, v)
	}

	return out
}
