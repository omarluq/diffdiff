// Package recents maintains a persistent, bounded most-recently-used list of
// project repository paths.
package recents

import (
	"path/filepath"
	"sync"

	"github.com/samber/oops"
)

// defaultMaxEntries is the fallback cap applied when a non-positive maximum is
// requested.
const defaultMaxEntries = 5

// Store is a concurrency-safe, bounded most-recently-used list of absolute
// repository paths backed by a JSON file on disk.
type Store struct {
	path    string
	max     int
	mu      sync.Mutex
	entries []string
}

// New creates a Store backed by <user-config-dir>/diffdiff/recent.json. A
// non-positive maxEntries defaults to five. A missing or corrupt backing file
// yields an empty store without error.
func New(maxEntries int) (*Store, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}

	return NewAt(path, maxEntries)
}

// NewAt creates a Store backed by the given path. A non-positive maxEntries
// defaults to five. A missing or corrupt backing file yields an empty store
// without error.
func NewAt(path string, maxEntries int) (*Store, error) {
	maxValue := maxEntries
	if maxValue <= 0 {
		maxValue = defaultMaxEntries
	}

	loaded := load(path)
	if len(loaded) > maxValue {
		loaded = loaded[:maxValue]
	}

	return &Store{
		path:    path,
		max:     maxValue,
		mu:      sync.Mutex{},
		entries: loaded,
	}, nil
}

// List returns the stored paths most-recent first, bounded by the configured
// maximum, as a copy safe for the caller to mutate.
func (s *Store) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]string, len(s.entries))
	copy(out, s.entries)

	return out
}

// Add records path as the most recent entry. The path is resolved to an
// absolute, cleaned form; any existing equal entry moves to the front. The
// list is capped at the configured maximum and persisted to disk.
func (s *Store) Add(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	abs = filepath.Clean(abs)

	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]string, 0, len(s.entries)+1)
	filtered = append(filtered, abs)

	for _, entry := range s.entries {
		if entry != abs {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) > s.max {
		filtered = filtered[:s.max]
	}

	s.entries = filtered

	if err := persist(s.path, s.entries); err != nil {
		return oops.In("recents").Code("persist").Wrapf(err, "persist recent paths")
	}

	return nil
}
