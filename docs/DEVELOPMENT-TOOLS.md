# Development tools and memory validation

Initial audit: 2026-11-14; M3 verification refresh: 2026-11-15.
These are development aids, not product features or substitutes for tests.
Read [PLAN.md](PLAN.md), the [M4 requirements](phases/M4.md), implemented [M3 contract](phases/M3.md)
and accepted [ADRs](adr/) first. M4 ADR 0006 is accepted; implementation evidence is still pending.

## CodeGraph

CodeGraph 1.6.2 is installed. The existing index was refreshed from merged M2 during the initial audit
and from final M3 source on 2026-11-15: 75 files, 819 nodes and 1,804 edges; status reported up to date.
Check freshness when switching branches or pulling changes:

```bash
codegraph status
codegraph sync
```

On 2026-11-17, `codegraph status` reported the existing index up to date against merged M3 source.
No new index or generated source was created for M4 preparation.

For indexed code, use the CodeGraph MCP exploration tool with this repository's absolute
`projectPath` and the symbol/flow you need. Read its returned source and callers before edits;
use direct source/config searches for areas the graph does not capture. Unresolved references remain,
so the graph does not prove complete dependency coverage, correctness or test coverage.
Builds and CI do not require the local index. Generated databases/transient files stay ignored through
`.codegraph/.gitignore`; do not commit them. Initialize a new index only when intentionally enabling it.

## MemGuard and runtime profiling

MemGuard is JavaScript/TypeScript static resource analysis. It does not analyze Go/Python, inspect a
running process, or prove there are no leaks. Pin its version, check the reported file count and
diagnostics, and inspect resource findings rather than suppressing them to get a green result.

Use Go allocation/heap profiles and process RSS around repeated diagnosis, cancellation and fixture runs. Check that subprocesses, sockets and goroutines finish after deadlines. This is a Go CLI, so MemGuard and Java HPROF analysis do not cover its current code.

For runtime changes, reproduce the suspected workload, record a baseline, repeat the same lifecycle,
then compare live/retained memory after cleanup. Capture allocation rate, heap/RSS, goroutines or
active handles as applicable, and latency/drop metrics. Go benchmarks can collect `-benchmem` and
`-memprofile` for a specific package/workload; inspect profiles with `go tool pprof`. Node supports
heap snapshots/inspector; Python supports `tracemalloc`. Choose the native tool for the process.
Record versions, duration, inputs, commands and limitations in a validation report.
Profile only when the change or suspected fault justifies it; a documentation edit needs link/diff checks.

[M3 validation](testing/m3-validation.md) records the actual proxy lifecycle/heap benchmark, retained
profile, goroutine counts and local diagnosis timings. Those are bounded Go workload measurements;
MemGuard/MAT were not treated as Go runtime gates or evidence of universal leak freedom.

## M4 implementation validation

Use Go heap/RSS/goroutine measurements for repeated archive read/write, rejection, cancellation and
terminal resize/exit. Bound decompression and report buffers independently; use synthetic captures and
measure cleanup after failures. Terminal dependencies remain Go code, so JS static tools do not replace
these checks. M4 resource/renderer evidence is planned, not measured by this documentation pass.

## Memory Analyzer

The locally installed analyzer is Eclipse MAT 1.17.0, configured with Java 21 and a 4 GiB analyzer heap.
It reads Java/HPROF heap dumps. This repository has no Java runtime workload; MAT is therefore not an
appropriate gate for it. MAT's own heap setting is not this project's memory budget.
Heap dumps/profiles can contain secrets and user data; keep them local/ignored and use synthetic inputs.

## Handoff between editors or assistants

State the branch and commit, read the phase/ADRs, refresh the graph after source changes, and provide
the exact failing command, output and minimal synthetic fixture. Keep implementation on a feature
branch. A skipped integration test must be reported as skipped, with its missing dependency; it is not
a passing check. Preserve local Docker volumes and use disposable databases for migration tests.
