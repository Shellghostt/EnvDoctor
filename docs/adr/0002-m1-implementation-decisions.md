# ADR 0002 - M1 implementation decisions

- Status: accepted
- Date: 2026-11-13
- Deciders: Aditya

## Context

M1 delivered the CLI skeleton, the collector pipeline and the first two
collectors. Several behaviours were underspecified, and one acceptance
criterion (exit code on collector failure) needed an interpretation against
PLAN section 6 ("a timeout is degraded, not a crash").

## Decisions

1. **Go 1.23 is kept.** Unlike EventRelay, every dependency here declares a
   minimum at or below 1.23: Cobra 1.10.2, `golang.org/x/sys` v0.30.0 (pinned;
   newer releases require a newer toolchain) and `jsonschema/v5` (test only,
   Go 1.18). There was no reason to raise the floor for a utility that people
   may build on an older machine.
2. **A collector returns a `Part`, not a `Snapshot`.** `Part` has one field per
   collector and the runner merges parts after every goroutine has finished.
   This keeps parallel collection race-free without a mutex, makes "which
   collector owns which field" a compile-time fact, and makes a failed
   collector unable to contribute partial data.
3. **Collector failure exits 3, and the report is still printed.** PLAN
   section 6 says a failure is degraded rather than fatal; the M1 acceptance
   criterion says exit 3 on collector failure. Both are honoured: the snapshot
   records the error, the text report lists the skipped collector, and the
   exit code is 3 because the diagnosis is incomplete.
4. **Rules filter findings, not collectors.** `--component` and `--rule` narrow
   what is evaluated; collection still covers the whole snapshot, so a future
   `--component docker` costs the same as a full run. Optimising that is a
   later change, not a behaviour change.
5. **Redaction replaces the home directory with `~` in every exported string.**
   The comparison is case-insensitive and accepts either path separator. A
   partial segment match is not rewritten, so `C:\Users\ann` does not touch
   `C:\Users\anna`. Existence is decided before redaction and survives it.
6. **Empty PATH elements are dropped.** On Unix an empty element means "the
   current directory", which cannot be expressed as a directory path. Recording
   it as existing would be false; recording it as missing would be misleading.
   It is therefore absent, and a rule may address it in a later milestone with
   its own evidence.
7. **The JSON document is versioned separately from the snapshot schema.**
   `docs/schema/snapshot-v1.schema.json` describes machine facts;
   `docs/schema/diagnose-v1.schema.json` describes the wrapper (`findings`,
   `summary`, `exitCode`, `rules`, `collectors`). Both are enforced by a test
   that validates real CLI output, not a hand-written example.
8. **The golden-fixture harness exists before the first rule.** M1 ships the
   loader, the evaluator and a test that scans `tests/golden`. It fails if a
   fixture names a rule that is not registered, and it fails if a registered
   rule has no fixture. The first rule added will therefore arrive with its
   fixture or not at all.
9. **`goreleaser check` needs a git remote; `goreleaser build --snapshot` does
   not.** The repository has no remote yet, so configuration validation is
   limited to snapshot builds, which cross-compile all six targets. That is
   what M1 verified.

## Consequences

- `diagnose` on a healthy machine exits 0 and prints zero findings because this
  build registers zero rules. It is honest about that: the report states the
  rule count.
- A collector that hangs is bounded at 2 s and cannot block the others.
- JSON output is safe to attach to a bug report without leaking the username.
- The fix engine, bundles and Bubble Tea renderer remain unimplemented; no
  command pretends otherwise.
