// Package di wires the application runtime dependency graph.
package di

import (
	"context"

	"github.com/samber/do/v2"
	"github.com/samber/oops"
)

// Container wraps the root injector used by the CLI runtime.
type Container struct {
	injector *do.RootScope
}

// NewContainer builds the root injector for the CLI runtime. configPath is the
// optional config file path; repoPath is the git repository to inspect.
func NewContainer(configPath, repoPath string) (*Container, error) {
	injector := do.New()
	do.ProvideNamedValue(injector, ConfigPathKey, configPath)
	do.ProvideNamedValue(injector, RepoPathKey, repoPath)
	RegisterServices(injector)

	if _, err := do.Invoke[*ConfigService](injector); err != nil {
		return nil, oops.
			In("di").
			Code("container_init").
			Wrapf(err, "initialize container")
	}

	return &Container{injector: injector}, nil
}

// ShutdownWithContext stops all registered services using the given context.
func (c *Container) ShutdownWithContext(ctx context.Context) *do.ShutdownReport {
	return c.injector.ShutdownWithContext(ctx)
}

// MustInvoke resolves a dependency and panics if it cannot be created.
func MustInvoke[T any](c *Container) T {
	return do.MustInvoke[T](c.injector)
}

// Invoke resolves a dependency, returning an error if it cannot be created.
func Invoke[T any](c *Container) (T, error) {
	return do.Invoke[T](c.injector)
}
