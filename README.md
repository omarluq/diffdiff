<p align="center">
  <img src="assets/imgs/mascot.png" alt="diffdiff" width="280">
</p>
<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-%3E%3D1.26-00ADD8?style=flat&labelColor=24292e&logo=go&logoColor=white" alt="Go"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue?style=flat&labelColor=24292e&logo=opensourceinitiative&logoColor=white" alt="MIT"></a>
  <a href="https://github.com/omarluq/diffdiff/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/omarluq/diffdiff/ci.yml?style=flat&labelColor=24292e&label=Tests&logo=github&logoColor=white" alt="Tests"></a>

  <p align="center"> A fast, themeable desktop app for browsing your Git working-tree diff </p>

</p>

## Run

```bash
mise install                   # pinned Go + Task toolchain
mise exec -- task run           # build ./bin/diffdiff and launch it in the current repo
```

`diffdiff [path]` opens the repository at `path` (default: the current directory); switch repositories anytime from the folder button in the toolbar.

## License

MIT
