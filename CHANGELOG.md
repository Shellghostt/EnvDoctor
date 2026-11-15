# Changelog

All notable changes to EnvDoctor are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- M3 rules ENV007–ENV012: known Docker API incompatibility, explicitly required dependency absence,
  package-manager/lockfile disagreements, effective Git identity/explicit autocrlf policy,
  canonical virtualenv/interpreter mismatch and opt-in proxy refusal/no-route evidence.
- Local Docker context/API metadata, strict version-1 `envdoctor.requirements.json`, supported
  tool/OpenSSL metadata checks and explicit `--package-manager` runner evidence.
- Redacted proxy configuration with bounded `--probe-network` DNS/TCP checks, offline by default.
- Offline `explain`, `rules --json`, a shared rule registry, generated pages for all twelve rules,
  `make docs`/`make check-docs` and CI documentation drift enforcement.
- 118 additional golden cases, preserving the original 54 for 172 total; collector, schema, purity,
  CLI, cancellation and socket lifecycle regressions.
- Accepted [ADR 0005](docs/adr/0005-m3-evidence-and-rule-registry.md),
  [M3 acceptance requirements](docs/phases/M3.md), [validation evidence](docs/testing/m3-validation.md)
  and [development-tool audit](docs/DEVELOPMENT-TOOLS.md). M3 is locally verified; branch CI is pending.

- M2 runtime, requested TCP-port and Git collectors, plus the local Docker probe needed by ENV006.
- Rules ENV001–ENV006 with deterministic evidence, manual suggestions and rule documentation.
- 54 three-OS golden cases and built-CLI checks for JSON, findings and exit codes.
- `--port` and bounded offline `--fixture-dir` diagnosis, with a committed synthetic demo.
- Runtime, Git and Docker facts in the plain-text report.
- The repository tracks CodeGraph's index ignore rules and documents it as an optional local tool
  for code navigation. Nothing in the build or in CI depends on it.

### Changed

- Optional M3 snapshot evidence extends schema v1 compatibly; project and rule-registry JSON have
  separate version-1 schemas. Explicit typed partial collector evidence retains degraded notices
  and disables dependent rules.
- `make build` disables CGO only for static builds and uses `.exe` on Windows; race tests retain CGO.
- Home/credential-URL redaction covers new evidence, paths within URLs, and collector/CLI errors.
- `golang.org/x/sys` updated to v0.48.0 to remove the vulnerable v0.30.0 dependency.
- Feature and fix branches trigger the three-OS CI matrix. Snapshot schema permits absent platform facts
  after a failed collector; offline errors retain degraded notices and exit code 3.
- The repository is published at https://github.com/Shellghostt/EnvDoctor and the CI workflow has
  executed. The first run on 2026-11-13 passed the Windows, Ubuntu and macOS matrix, the lint job and
  the vulnerability scan.
- The Go floor moves from 1.23 to 1.26. `go.mod` declares `go 1.26.0` and `toolchain go1.26.11`, so
  CI installs the patched toolchain and not the exact patch named by the `go` directive. Go 1.23 is
  past its support window and receives no security fixes. See ADR 0003.
- The linter is golangci-lint v2.15.0, pinned by version, run by `golangci/golangci-lint-action@v9`.
  The four findings it produced were fixed in the code rather than excluded.

### Fixed

- Bounded subprocess output no longer exposes `bytes.Buffer.ReadFrom`, which could bypass the
  writer's limit for stdout/stderr.
- Protected WindowsApps/MSIX paths and application aliases remain unresolved metadata; new
  unknown-resolution snapshots do not cause speculative M2 runtime conflicts.
- Git autocrlf boolean aliases use portable typed config queries; `input` is handled directly.
- M1 runner deadline now bounds a collector that ignores cancellation.
- Home redaction includes quoted PATH entries and paths embedded in collector/timing errors.
- Broken symlinks do not shadow usable runtimes; Python venvs stay distinct from their system symlink
  targets; unrelated numeric directories do not become runtime versions.
- Per-user Windows Docker locations are recognized. Invalid Git config and missing macOS tools cause
  degraded inspection instead of silent clean results. Git/Docker context overrides are removed.
- Port verification commands include the requested ports. Offline input rejects extra JSON documents.
- PATH permission/inspection failures are degraded rather than converted to missing-directory evidence;
  malformed paths and non-directory ancestors still produce broken-entry warnings with raw text preserved.
- Four writes whose error return was discarded now say so explicitly with `_, _ =`.

### Planned

- M4 reports and bundles; later rules ENV013–ENV018, confirmed fixes and signed releases.

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
