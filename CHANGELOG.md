# Changelog

All notable changes to EnvDoctor are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- The CI matrix is configured but has never executed: the repository has no remote yet.
- `goreleaser check` fails without a git remote; only the snapshot build was verified.

## [0.0.0] - 2026-11-13

- Initial skeleton. No functionality.
