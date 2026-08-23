# Changelog

All notable changes to `nacelle-tui` are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow semver —
while on `v0`, a breaking change bumps the minor.

## [Unreleased]

## [0.1.0] — 2026-08-23

### Added

- First tagged release of the terminal client, extracted from
  [FacileStudio/nacelle](https://github.com/FacileStudio/nacelle) where it lived as `tui/`.
  The module is now `github.com/FacileStudio/nacelle-tui` and pins core v0.3.1.
- `-version` flag printing exactly `nacelle <semver>`, stamped by goreleaser's ldflags into
  `main.version`.
- Distribution per CLI-STANDARD §5/§9: GoReleaser archives for darwin/linux amd64+arm64,
  Homebrew formula in `FacileStudio/tap`, an `install.sh` shim, and a `nacelle` entry in the
  `facile` catalog.

[Unreleased]: https://github.com/FacileStudio/nacelle-tui/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.1.0
