# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`diffdiff` is a Go CLI built on a strict, batteries-included toolchain. The runtime wires a `samber/do` dependency-injection container that loads Viper-based config and a zerolog/slog logger, exposed through a Cobra command tree styled by Fang.

- **CLI**: Fang v2 (`charm.land/fang/v2`) wrapping Cobra
- **Config**: Viper (defaults → env → optional YAML file), returned as `mo.Result[*Config]`
- **DI**: samber/do v2 behind a `di.Container` facade
- **Functional/monads/errors**: samber `lo`, `mo`, `oops`
- **Logging**: zerolog with a slog bridge (`slog-zerolog`)
- **Testing**: stretchr/testify
- Requires Go 1.26+ (mise pins 1.26.3).

## Commands

All workflow runs through Task; `mise exec --` ensures the pinned toolchain. Tasks set repo-local `GOCACHE`/`GOMODCACHE`/`GOTMPDIR` (in `.gocache`, `.gomodcache`, `.tmp`) via the `ensure-cache` dep, so prefer Task over bare `go`.

```bash
mise exec -- task build          # Build to ./bin/diffdiff with version ldflags
mise exec -- task run            # Build and run
mise exec -- task test           # go test -v -race ./...
mise exec -- task test-short     # adds -short
mise exec -- task test-coverage  # coverage.out + coverage.html
mise exec -- task lint           # golangci-lint run
mise exec -- task fmt            # golangci-lint run --fix
mise exec -- task ci             # fmt → lint → test → build
```

Run a single test (set the cache env so it matches Task, or just run after a `task build` once):

```bash
GOCACHE="$PWD/.gocache" go test -race -run '^TestRootCommand$' ./cmd/diffdiff/
```

`task --list` shows all tasks. There is **no** `dev`/live-reload task.

## Architecture

The dependency graph is built once at startup and resolved lazily:

```
main.run() → fang.Execute(ctx, newRootCmd(), WithVersion(vinfo.String()))
                          │
            cmd/diffdiff/*.go  (cobra commands; --config sets package var cfgFile)
                          │
   di.NewContainer(cfgFile) → do.New() injector
                          │   ProvideNamedValue(ConfigPathKey, path)
                          │   RegisterServices: ConfigService, LoggerService
                          │
   ConfigService ── config.Load(path).Get() ──► *config.Config (validated)
   LoggerService ── reads ConfigService ──────► zerolog + slog (slog.SetDefault)
```

Key seams:

- **`internal/di/container.go`** — the public facade. Use `di.NewContainer(configPath)`, `di.MustInvoke[T](c)`, and `c.ShutdownWithContext(ctx)`. Do **not** pass the raw `do.Injector` around; services receive it internally via their `New*` constructors.
- **Config path injection**: the CLI's `--config` value is stored as a *named* injector value (`di.ConfigPathKey = "config.path"`) and read by `ConfigService` via `do.MustInvokeNamed[string]`. This is how a Cobra flag reaches a DI-constructed service without globals leaking into `internal/`.
- **Config resolution** (`internal/config/loader.go`): `Load()` layers defaults → env (`DIFFDIFF_` prefix, `.`→`_`) → optional file (`./config.yaml` or `$HOME/.config/diffdiff/config.yaml`), unmarshals, then runs `Config.Validate()`. A missing *default* file is not an error; a missing *explicit* `--config` file is. Returns `mo.Result[*Config]` — call `.Get()` to unwrap.
- **Validation is strict** (`config.go`): `app.env ∈ {development,test,production}`, `logging.level ∈ {debug,info,warn,error}`, `logging.format ∈ {pretty,json}`. Adding a config field means adding its default in `setDefaults`, a struct field with `mapstructure`/`yaml`/`json` tags, and a `Validate()` branch.
- **`cmd/diffdiff/`** currently exposes `config` (`show`/`validate`) and `version`. The `config show` command demonstrates the intended `lo` style (`lo.Map`, `lo.SliceToMap`, `lo.MaxBy`).
- **`internal/vinfo`** — `Version`/`Commit`/`BuildDate` are set via `-ldflags` in `task build`; falls back to `debug.ReadBuildInfo()` for `go install` builds.

## Code Style (enforced — 50+ linters, no test exclusions)

- **`exhaustruct`** is enabled for all `^github.com/omarluq/diffdiff/.*` structs: every struct literal in this module must initialize **all** fields. This is the most common surprise when adding code.
- **`errcheck` with `check-blank: true`**: never discard errors, including `_ =`. Handle every `fmt.Fprintf`/`fmt.Fprintln` return (see `printLine` in `cmd/diffdiff/config.go` for the pattern).
- Wrap errors with `oops.In("domain").Code("code").Wrapf(err, "msg")`.
- Use `mo.Option`/`mo.Result` for fallible/optional values; `lo` for collection transforms.
- Internal-only test helpers are exported through `export_test.go` (same-package test access).

## Git Hooks (Lefthook)

Installed via `task hooks-install`. Commits and pushes are gated:

- **pre-commit**: `golangci-lint run --fix` (auto-stages fixes) + `task test-short`
- **pre-push**: `task test` + `task build`
- **commit-msg**: **Conventional Commits required** — `type(scope?): subject` where type ∈ `feat|fix|docs|style|refactor|perf|test|chore|ci|build`. Non-conforming messages are rejected.

## Adding Code

- **New command**: create `cmd/diffdiff/yourcmd.go` exporting `newYourCmd() *cobra.Command`; register it in `root.go` via `cmd.AddCommand(...)`.
- **New service**: create `internal/yourservice/`, add `do.Provide(injector, NewYourService)` in `internal/di/register.go`, resolve with `di.MustInvoke[*YourService](container)`. Service constructors take `do.Injector` and pull their dependencies with `do.MustInvoke`.

## Notes

- `AGENTS.md` mirrors this guidance for other AI harnesses — keep the two roughly in sync when changing workflow or conventions.
- This repo started from a Go template (`README.md` describes a `task init` rename flow). That init task/command is **not present** in the current `Taskfile.yml`; if repurposing, the module path `github.com/omarluq/diffdiff`, the `diffdiff` binary name, the `DIFFDIFF_` env prefix (`internal/config/loader.go`), the `exhaustruct` include pattern (`.golangci.yml`), and `cmd/diffdiff/` must be changed by hand.
