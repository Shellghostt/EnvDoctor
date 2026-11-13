# Changelog

All notable changes to EnvDoctor are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The repository tracks CodeGraph's index ignore rules and documents it as an optional local tool
  for code navigation. Nothing in the build or in CI depends on it.

### Changed

- The repository is published at https://github.com/Shellghostt/EnvDoctor and the CI workflow has
  executed. The first run on 2026-11-13 passed the Windows, Ubuntu and macOS matrix, the lint job and
  the vulnerability scan.
- The Go floor moves from 1.23 to 1.26. `go.mod` declares `go 1.26.0` and `toolchain go1.26.11`, so
  CI installs the patched toolchain and not the exact patch named by the `go` directive. Go 1.23 is
  past its support window and receives no security fixes. See ADR 0003.
- The linter is golangci-lint v2.15.0, pinned by version, run by `golangci/golangci-lint-action@v9`.
  The four findings it produced were fixed in the code rather than excluded.

### Fixed

- Four writes whose error return was discarded now say so explicitly with `_, _ =`.

### Planned

- M2: runtime, port and Git collectors, plus the first diagnostic rules with golden fixtures.

## [0.1.0] - 2026-11-13

### Added

- `envdoctor diagnose` with `--json`, `--component`, `--rule` and `--verbose`.
- `envdoctor version` and `--version`.
- Collector interface, parallel runner and a 2 s per-collector deadline. A failed collector is
  recorded in the snapshot, printed in the report and makes the run exit 3 without aborting the
  other collectors.
- Platform collector: operating system, version, architecture and elevation on Windows, Linux and
  macOS. An unavailable version is reported as empty rather than guessed.
- Shell collector: shell name, and path when the shell exposes `SHELL`.
- PATH collector: one entry per element with existence, directory, position and duplicate flags.
  Existence comes from file metadata; nothing on PATH is executed.
- Home-directory redaction to `~` in every exported string, case-insensitive and
  separator-insensitive, applied before the snapshot is written anywhere.
- Text report and a versioned JSON document.
- Exit codes: `0` clean, `1` warning, `2` error or critical, `3` tool failure (including a
  degraded collector), `4` invalid usage.
- Committed JSON schemas in `docs/schema/` for the snapshot and the diagnose document, enforced by
  a test that validates real CLI output.
- Golden-fixture harness (`internal/diagnostic/fixture.go`, `tests/golden/`): a fixture for a rule
  that is not registered fails, and a registered rule without a fixture fails.
- `.goreleaser.yaml` covering six targets, verified with `goreleaser build --snapshot --clean`.
- CI workflow with a Windows/Ubuntu/macOS test matrix, gofmt, `go vet`, `go test -race`, build and
  lint jobs.

### Known limitations

- Zero diagnostic rules are registered, so `diagnose` never reports a finding and exit codes 1 and
  2 are unreachable in this release. The report states the rule count instead of implying a clean
  bill of health.
- No runtime, port, Git, Docker or dependency collectors.
- No bundles, no fixes, no Bubble Tea renderer.
- The CI matrix was configured but had not executed when this tag was created: the repository had no
  remote yet. It has since run on GitHub; see [Unreleased].
- `goreleaser check` fails without a git remote; only the snapshot build was verified.

## [0.0.0] - 2026-11-13

- Initial skeleton. No functionality.
