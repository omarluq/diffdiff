package di

import (
	"github.com/samber/do/v2"

	"github.com/omarluq/diffdiff/internal/git"
)

// RepoPathKey stores the path to the git repository to inspect in the injector.
const RepoPathKey = "repo.path"

// NewRepository opens the git repository located at the injector's configured
// repository path.
func NewRepository(injector do.Injector) (*git.Repository, error) {
	path := do.MustInvokeNamed[string](injector, RepoPathKey)

	return git.Open(path)
}
