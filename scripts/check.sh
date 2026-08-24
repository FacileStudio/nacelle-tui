#!/usr/bin/env sh
#
# The repository quality gate. Reports, never rewrites (except --format).
#
#   sh scripts/check.sh           gofmt + vet + test + lint
#   sh scripts/check.sh --no-lint skip the lint pass
#   sh scripts/check.sh --format  rewrite Go sources in place
#
# One module, so there is nothing to walk: every step runs from the root and
# the script is a third the length of the one it inherited, most of which
# existed to reconcile a nested module with its parent across a workspace.
# The lint pass is skipped rather than fatal when golangci-lint is missing,
# because CI runs it either way and a contributor without the tool should
# still be able to check their work.

set -eu

mode="all"
case "${1:-}" in
--no-lint) mode="nolint" ;;
--format) mode="format" ;;
"") ;;
*)
  echo "usage: $0 [--no-lint|--format]" >&2
  exit 2
  ;;
esac

root="$(git rev-parse --show-toplevel)"
cd "$root"

# Resolve the toolchain from GOROOT when it is set. mise exports GOROOT for
# the version this repo pins while leaving an unrelated `go` earlier on PATH,
# and a go binary driving a different GOROOT fails with
# `compile: version "X" does not match go tool version "Y"`.
if [ -n "${GOROOT:-}" ] && [ -x "$GOROOT/bin/go" ]; then GO="$GOROOT/bin/go"; else GO=go; fi
if [ -n "${GOROOT:-}" ] && [ -x "$GOROOT/bin/gofmt" ]; then GOFMT="$GOROOT/bin/gofmt"; else GOFMT=gofmt; fi

if ! command -v "$GO" >/dev/null 2>&1; then
  echo "check: no usable go ('$GO')" >&2
  exit 1
fi

if [ "$mode" = "format" ]; then
  git ls-files -co --exclude-standard -z -- '*.go' | xargs -0 "$GOFMT" -w
  echo "==> formatted"
  exit 0
fi

status=0

# gofmt is pointed at this repository's own sources rather than at the tree,
# because the tree is not only this repository. A git worktree checked out
# below the root — which is where agent tooling puts one, under .claude/ — is
# walked by `gofmt -l .` like any other directory, so somebody else's
# half-written file failed this gate and the pre-push hook with it, naming a
# path the person pushing had never opened.
#
# Tracked and untracked-but-not-ignored, so a source file written a minute ago
# is still checked and anything .gitignore already excludes is not. That is the
# same set every other tool here means by "this repository's files".
#
# The handoff is null-delimited (-z feeding xargs -0), not whitespace-delimited:
# git passes spaces through unquoted, so xargs would split a filename at its
# space and gofmt would format half a path. It also means an empty list runs
# gofmt on nothing rather than once with stdin attached, which GNU xargs would
# do where the BSD one on macOS does not.
echo "==> gofmt"
unformatted="$(git ls-files -co --exclude-standard -z -- '*.go' | xargs -0 "$GOFMT" -l)"
if [ -n "$unformatted" ]; then
  echo "gofmt: the following files are not formatted (run 'sh scripts/check.sh --format'):"
  echo "$unformatted"
  status=1
fi

echo "==> go build"
"$GO" build ./... || status=1

echo "==> go vet"
"$GO" vet ./... || status=1

# -race, not as a nicety: bubbletea updates arrive from its own goroutines,
# which is exactly the shape the detector exists for.
echo "==> go test"
"$GO" test -race ./... || status=1

if [ "$mode" != "nolint" ]; then
  echo "==> golangci-lint"
  if golangci-lint version >/dev/null 2>&1; then
    golangci-lint run ./... || status=1
  else
    echo "check: no usable 'golangci-lint', skipping the lint pass (CI still runs it)" >&2
  fi
fi

if [ "$status" -ne 0 ]; then
  echo "check failed"
  exit "$status"
fi

echo "==> ok"
