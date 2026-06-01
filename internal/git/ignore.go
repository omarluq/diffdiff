package git

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// supplementalMatcher covers only the ignore sources go-git's Status does NOT
// apply: .git/info/exclude and the user/system global excludes (including the
// XDG default ~/.config/git/ignore). The repository's own .gitignore is already
// enforced by Status, so the default path needs only this — and crucially
// avoids the recursive .gitignore scan that would descend into huge ignored
// trees like node_modules.
func (r *Repository) supplementalMatcher() gitignore.Matcher {
	patterns := parsePatternFile(filepath.Join(r.root, ".git", "info", "exclude"))
	patterns = append(patterns, globalIgnorePatterns()...)

	return gitignore.NewMatcher(patterns)
}

// globalIgnorePatterns gathers user- and system-level ignore patterns from every
// standard location so files ignored only globally (editor/tool dotfiles) are
// excluded just as git would exclude them.
func globalIgnorePatterns() []gitignore.Pattern {
	root := osfs.New("/")

	var patterns []gitignore.Pattern
	if global, err := gitignore.LoadGlobalPatterns(root); err == nil {
		patterns = append(patterns, global...)
	}
	if system, err := gitignore.LoadSystemPatterns(root); err == nil {
		patterns = append(patterns, system...)
	}
	for _, path := range defaultGlobalIgnoreFiles() {
		patterns = append(patterns, parsePatternFile(path)...)
	}

	return patterns
}

// defaultGlobalIgnoreFiles returns the global ignore files git reads when
// core.excludesFile is unset: $XDG_CONFIG_HOME/git/ignore (default
// ~/.config/git/ignore) and ~/.gitignore.
func defaultGlobalIgnoreFiles() []string {
	files := make([]string, 0, 2)

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		files = append(files, filepath.Join(xdg, "git", "ignore"))
	} else if home, err := os.UserHomeDir(); err == nil {
		files = append(files, filepath.Join(home, ".config", "git", "ignore"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files, filepath.Join(home, ".gitignore"))
	}

	return files
}

// parsePatternFile reads a gitignore-format file into root-domain patterns. A
// missing or unreadable file yields no patterns.
func parsePatternFile(path string) []gitignore.Pattern {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is a fixed git ignore-file location, not user input
	if err != nil {
		return nil
	}

	var patterns []gitignore.Pattern
	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(trimmed, nil))
	}

	return patterns
}
