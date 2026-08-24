# Changelog

All notable changes to `nacelle-tui` are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow semver —
while on `v0`, a breaking change bumps the minor.

## [Unreleased]

### Fixed

- The gofmt pass in `scripts/check.sh` handed git's file list to `xargs` as
  whitespace-delimited text. Git passes spaces in filenames through unquoted, so a path
  with a space was split and half of it formatted; the list is null-delimited now (`-z`
  feeding `xargs -0`), which also stops GNU `xargs` running gofmt once on an empty list
  with stdin attached.
- The status line's waiting phrase is bucketed from when this run began rather than from
  the wall clock, so every wait opens on the first phrase instead of wherever the epoch
  happened to be.

## [0.2.1] — 2026-08-24

### Changed

- Homebrew installs from a cask rather than a formula. `brew install
  FacileStudio/tap/nacelle` is unchanged; what moves is the tap file, from
  `Formula/nacelle.rb` to `Casks/nacelle.rb`. GoReleaser soft-deprecated `brews` in v2.10 and
  hard-deprecated it in v2.16, and release CI runs 2.17.1, so the previous config would have
  failed its own check on the next tag. A cask download carries the quarantine attribute
  where a formula's did not, so a `postflight` hook strips it — without that, the first run
  of the unsigned binary dies on "the developer cannot be verified".
- The core SDK is pinned at v0.4.0, up from v0.3.1. The only change between the two is the
  split that created this repository, so nothing here had to adapt.

## [0.2.0] — 2026-08-24

### Added

- `ctrl+t` expands a turn's reasoning, and keeps showing it in full until pressed again. It
  costs the prompt its `transpose-character-backward` binding.

### Changed

- A tool call reads as `read_file(view.go) · 12ms` instead of the tool name followed by its
  raw JSON input. The argument is picked by key (`path`, `file_path`, `file`, `command`,
  `pattern`, `query`, `url`, `name`), falling back to the sole key of a one-argument call.
- A tool that succeeds is one line carrying its duration, not a call line plus a
  `done in 12ms` line under it. Failures keep both lines. The call line is held until the
  result arrives, which the status line already covers by naming the running tool; a run that
  ends with calls in flight still prints them, without a duration.
- Reasoning collapses to `· thought for 4.2s`, with the `ctrl+t` hint shown once per session.

### Fixed

- The thinking duration measured the whole turn. A turn is committed at its end, which is
  after the answer has streamed and after any tool the model called has run, so 0.6s of
  reasoning followed by a 0.6s answer printed `thought for 1.2s`. The clock now stops at the
  first answer delta or tool call.
- `/clear` left the retained reasoning behind, so `ctrl+t` reprinted the thinking from the
  session that was just cleared.
- A run that ended mid-tool said the held call line above the sentence that announced it.
- The build binary `nacelle-tui` was tracked rather than ignored; plain `go build` writes it
  and `.gitignore` only listed the old `nacelle` name. Untracked here, though the objects
  already in history stay there.

### Security

- Tool-call input carrying a repeated key is refused rather than summarised.
  `encoding/json` silently keeps the last value, so `{"command":"ls","command":"rm -rf /"}`
  could be shown as `run_command(ls)` over a call that ran something else. With
  `-approve-tools` on, such a call is denied before any prompt is drawn and before the
  session allow-list is consulted, since allow-for-session is permission for a tool granted
  against a legible call, not a standing waiver on whatever input arrives afterwards.

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

[Unreleased]: https://github.com/FacileStudio/nacelle-tui/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.2.1
[0.2.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.2.0
[0.1.0]: https://github.com/FacileStudio/nacelle-tui/releases/tag/v0.1.0
