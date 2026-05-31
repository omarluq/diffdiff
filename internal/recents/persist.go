package recents

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/samber/oops"
)

// dirPerm is the permission applied to the directory holding the backing file.
const dirPerm = 0o750

// defaultPath returns the default backing file location under the user's
// configuration directory.
func defaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", oops.In("recents").Code("config_dir").Wrapf(err, "resolve user config dir")
	}

	return filepath.Join(base, "diffdiff", "recent.json"), nil
}

// load reads the backing file at path. A missing file or invalid JSON yields an
// empty slice rather than an error so a corrupt store self-heals on next write.
func load(path string) []string {
	data, err := os.ReadFile(path) //nolint:gosec // path is the store's own config file.
	if err != nil {
		return []string{}
	}

	var entries []string
	if err := json.Unmarshal(data, &entries); err != nil {
		return []string{}
	}

	return entries
}

// persist writes entries to path atomically: it marshals to JSON, ensures the
// parent directory exists, writes a temp file, then renames it into place.
func persist(path string, entries []string) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return oops.In("recents").Code("marshal").Wrapf(err, "marshal entries")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return oops.In("recents").Code("mkdir").Wrapf(err, "create store dir")
	}

	return writeAtomic(path, dir, data)
}

// writeAtomic writes data to a temp file in dir and renames it onto path,
// removing the temp file if any step fails.
func writeAtomic(path, dir string, data []byte) error {
	tmp, err := os.CreateTemp(dir, "recent-*.json")
	if err != nil {
		return oops.In("recents").Code("create_temp").Wrapf(err, "create temp file")
	}

	tmpName := tmp.Name()

	if err := writeAndClose(tmp, data); err != nil {
		return discardTemp(tmpName, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return discardTemp(tmpName, oops.In("recents").Code("rename").Wrapf(err, "rename temp file"))
	}

	return nil
}

// writeAndClose writes data to tmp and closes it, returning the first error
// encountered while still closing the file when the write fails.
func writeAndClose(tmp *os.File, data []byte) error {
	if _, err := tmp.Write(data); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return oops.In("recents").Code("write").Wrapf(err, "write temp file (close failed: %v)", closeErr)
		}

		return oops.In("recents").Code("write").Wrapf(err, "write temp file")
	}

	if err := tmp.Close(); err != nil {
		return oops.In("recents").Code("close").Wrapf(err, "close temp file")
	}

	return nil
}

// discardTemp removes the temp file named name, returning cause annotated with
// any removal failure.
func discardTemp(name string, cause error) error {
	if rmErr := os.Remove(name); rmErr != nil {
		return oops.In("recents").Code("cleanup").Wrapf(cause, "remove temp file failed: %v", rmErr)
	}

	return cause
}
