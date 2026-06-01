package di

import (
	"os"
	"path/filepath"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/omarluq/diffdiff/internal/recents"
)

// defaultRecentEntries bounds the recent-projects list shown in the picker.
const defaultRecentEntries = 5

// NewRecents builds the recent-projects store, falling back to a temp-dir store
// if the user config directory cannot be resolved so the picker still works.
func NewRecents(_ do.Injector) (*recents.Store, error) {
	store, err := recents.New(defaultRecentEntries)
	if err == nil {
		return store, nil
	}

	path := filepath.Join(os.TempDir(), "diffdiff-recent.json")
	fallback, err := recents.NewAt(path, defaultRecentEntries)
	if err != nil {
		return nil, oops.In("recents").Code("recents.fallback").Wrapf(err, "create fallback recents store at %s", path)
	}

	return fallback, nil
}
