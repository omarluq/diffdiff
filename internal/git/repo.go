// Package git reads diffs from a local repository using pure-Go go-git.
//
// go-git does not emit line-level patches for the working tree (Status only
// reports per-file change codes), so this package reads the old (HEAD blob) and
// new (working file) content itself and hands both to the diff engine. go-git's
// role is repository state and blob access; the diff model is computed by
// internal/diff.
package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/samber/oops"
)

// Repository is an opened local git repository rooted at a working tree.
type Repository struct {
	repo *git.Repository
	root string
}

// Open opens the repository containing path, walking up to find the .git
// directory so any subdirectory of a repo works.
func Open(path string) (*Repository, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: false,
	})
	if err != nil {
		return nil, oops.
			In("git").
			Code("open_repo").
			With("path", path).
			Wrapf(err, "open repository")
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, oops.
			In("git").
			Code("worktree").
			Wrapf(err, "resolve worktree")
	}

	return &Repository{repo: repo, root: wt.Filesystem.Root()}, nil
}

// Root returns the absolute path to the repository working tree.
func (r *Repository) Root() string {
	return r.root
}
