# EnvDoctor M1 and M2 validation — 2026-11-14

Branch: `feat/m2-implementation`. M2 is complete locally and unreleased. No main-branch changes,
pushes or release publication are part of this work.

## Environment

Windows host (AMD Ryzen 5 5600H), Go 1.26.11, Docker Desktop Linux containers with Go 1.26.11,
golangci-lint 2.15.0 and GoReleaser. Linux race tests use the cached `golang:1.26.11` image.
Docker is a test tool/optional inspected component; the CLI itself remains one static Go binary.

## M1 regressions

The original platform, shell, PATH, rendering, schema, flags and exit-code tests pass. Extra regressions
bound the runner when a collector ignores cancellation, permit absent platform facts in a degraded JSON
snapshot, and preserve home redaction in quoted entries and error/timing notices. PATH metadata access
failures now degrade collection rather than inventing evidence that a directory is missing. Malformed
Windows paths and paths traversing a regular file remain broken-entry evidence, with the raw entry preserved.

## M2 acceptance

| Requirement | Evidence |
|---|---|
| Python and Node conflict fixtures for all three OSes | 54 exact golden cases: six rules × Windows/Linux/macOS × positive/negative/edge |
| Raw broken PATH entries preserved | Collector quoting/raw-entry tests, ENV003 fixtures and home-redaction tests |
| Requested port identifies its owner where permitted | Real owned TCP listener tests on Windows and Linux; final Windows CLI smoke run identified `pwsh.exe` PID 7200 on TCP 26743 |
| All six rules covered by CLI and CI tests | Built binary exercised against all 54 fixtures, checking exact findings, schema and exit code; full suite wired into the three-OS CI matrix |
| Rules perform no direct OS/environment/process I/O | Import/AST purity test; deterministic snapshot-to-finding comparisons |

Collector tests cover nonexecution of discovered PATH binaries, unrelated numeric-directory versions,
broken symlinks, venv/system symlink separation, missing versus invalid Git config, per-user Docker locations,
local Docker metadata states, sanitized Git/Docker context overrides, lsof parsing/failure distinction and
process deadlines. Offline tests reject trailing JSON and preserve captured collector errors as exit 3.

The committed `tests/demo/snapshot.json` is synthetic. It produces all six findings and exits 2.
A focused ENV005 demo includes `--port 3000` in its verification command. It is not a captured host diagnosis.

## Checks and measurements

- Native `go test -count=1 -timeout=120s ./...`, `go vet ./...`, and `go build ./...`: passed.
- Linux `go test -race -count=1 -timeout=120s ./...`: passed, including real listener and built-CLI tests.
- golangci-lint 2.15.0: zero issues. `govulncheck ./...` on Go 1.26.11: no vulnerabilities found.
- `goreleaser check` and `goreleaser build --snapshot --clean`: passed for all six targets.
- Live Windows JSON diagnosis detected six registered rules, the owned TCP socket, and the reachable
  per-user Docker Desktop CLI/engine. It did not change the listener or any environment configuration.
- Three sequential native diagnosis invocations took 208.78, 189.69 and 188.96 ms with a running Docker
  engine. These are local warm observations, not a clean-host or all-platform M6 performance claim.
- Ten additional sequential live listener checks passed without degraded collectors (184.54–229.65 ms).
  One earlier live attempt returned exit 3; the final thirteen warm runs completed without degradation.
- Snapshot binaries ranged from 3,276,962 to 3,693,056 bytes across the six targets.

The dependency `golang.org/x/sys` moved from v0.30.0 to v0.48.0, removing the vulnerability found during
the audit. Source formatting and staged diffs are checked before the feature-branch commit.

## Reproduce

```powershell
$env:GOTOOLCHAIN='go1.26.11'
go test -count=1 -timeout=120s ./...
go vet ./...
go build ./...
goreleaser check
goreleaser build --snapshot --clean
```

When a native Windows C toolchain cannot run the race detector:

```powershell
docker run --rm -v "${PWD}:/src:ro" -v m2-go-mod:/go/pkg/mod -v m2-go-build:/root/.cache/go-build -w /src -e GOTOOLCHAIN=go1.26.11 golang:1.26.11 go test -race -count=1 -timeout=120s ./...
```

CI runs that full suite on Windows, Ubuntu and macOS. Feature/fix branches now trigger it. The updated
branch has not been pushed during this session, so actual updated macOS execution and native Windows race
execution remain pending CI. Cross-compiling macOS binaries is not a substitute for executing them there.

## Limits

Runtime discovery is based on metadata. Unknown versions stay unknown; opaque shims, custom installation
locations and some Unix system binaries cannot be fully resolved without executing them. Expected targets
use `VIRTUAL_ENV`, `NVM_BIN` and `NVM_SYMLINK`. Ports are checked only when requested; owner information
can be unavailable due to OS permissions. Docker probes only the default local socket, not remote/rootless
or custom contexts. Git identity appears in JSON and should be reviewed before sharing.

Rules ENV007–ENV018, dependency inspection, broader Docker rules, generated explanation docs, bundles,
automatic remediation, interactive output and signed releases remain later milestones.
