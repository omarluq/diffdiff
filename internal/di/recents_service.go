package di

import (
	"os"
	"path/filepath"

	"github.com/samber/do/v2"

	"github.com/omarluq/diffdiff/internal/recents"
)

// defaultRecentEntries bounds the recent-projects list shown in the picker.
const defaultRecentEntries = 5

// NewRecents builds the recent-projects store, falling back to a temp-dir store
// if the user config directory cannot be resolved so the picker still works.
func NewRecents(_ do.Injector) (*recents.Store, error) {
	store, err := recents.New(defaultRecentEntries)
	if err != nil {
		return recents.NewAt(filepath.Join(os.TempDir(), "diffdiff-recent.json"), defaultRecentEntries)
	}

	return store, nil
}
