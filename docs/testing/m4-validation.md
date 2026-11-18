# EnvDoctor M4 validation — 2026-11-19

ENV-024–ENV-026 are implemented and locally verified on `feat/m4-implementation`.
Implementation commit: `c532dc33bd16271fb19298cf600a5dd6b05cceb9`.
The branch includes published preparation `2be420a`, based on merged M3 `6cf3581`.
[ADR 0006](../adr/0006-m4-reports-and-bundles.md) was accepted and committed at `32da338`
before dependent implementation. Accepted ADRs and all 172 original golden fixtures are unchanged.

**Publication and native Windows/Ubuntu/macOS M4 GitHub CI remain pending.** This record covers local
implementation, verification and limits. It does not claim a release, main merge or native macOS run.
Recorded dates use Asia/Calcutta; console captures also contain the corresponding 2026-11-18 UTC time.

## Environment and canonical gates

Windows 11 amd64, AMD Ryzen 5 5600H, Go 1.26.11, GNU Make 4.4.1 and golangci-lint 2.15.0.
Every local Go command pins `GOTOOLCHAIN=go1.26.11`. Native race uses the existing MSYS2 UCRT64
x64 GCC/G++ beneath `%LOCALAPPDATA%/Programs/MSYS2-Go-x64/msys64/ucrt64/bin`.
[Dependency review](m4-dependencies.md) records exact selected modules and license evidence.

| Check | Executed result |
|---|---|
| `make test` | Passed all packages, including 172 original built-CLI goldens, M4 formats/CLI, sharing, schemas, archive abuse and atomic-output cases |
| Native Windows `make race` | Passed all packages with the x64 compiler; no compiler-limitation skip |
| `make lint` | Passed gofmt, vet and golangci-lint: **0 issues** |
| `make build` | Passed; Windows amd64, CGO disabled, 5,479,936 bytes |
| `make check-docs` | Passed; all twelve generated rule pages remain unchanged |
| Linux `make race` | Passed in `golang:1.26.11`, using a read-only source mount; built-CLI integration package: 22.465 s |
| Linux docs/static build | `make check-docs` and CGO-disabled static build passed; build metadata identifies linux/amd64 and implementation `c532dc3` |
| Four remaining static cross-builds | darwin/amd64, darwin/arm64, windows/arm64 and linux/arm64 passed with CGO disabled |
| `govulncheck ./...` | **No vulnerabilities found** |
| `go mod verify` | All modules verified |
| Bounded `FuzzM4BundleReader` run | Passed: 15 s fuzz budget, two workers, 3,158 executions, one new interesting input; no failure corpus |
| Documentation and local review | `make check-docs`, local Markdown file links, all 40 named M4 test/fuzz/benchmark functions and `git diff --check` passed; golden/registry/ADR files preserved |
| Native three-OS M4 GitHub CI | **Pending publication**; earlier M3 CI is recorded separately in [M3 validation](m3-validation.md) |

Static build sizes were 5,173,410 bytes for linux/amd64; 4,915,362 for linux/arm64;
5,245,904 for darwin/amd64; 4,989,714 for darwin/arm64; and 5,106,688 for windows/arm64.
These compile checks cover all six targets and stay below the 25 MB target. They are not signed release
or native macOS execution evidence. GoReleaser packaging was not rerun for M4.

## Acceptance mapping

| M4 criterion | Named executed evidence |
|---|---|
| TTY, redirects, `TERM=dumb`, empty/present `NO_COLOR`, explicit plain/format precedence and uncontaminated structured output | `TestM4FormatPolicy`, `TestM4CLIFormatAndTerminalMatrix`, `TestM4CLIInvalidFormatsBeforeCollection`, `TestM4CLIStructuredOutputErrors`, `TestM4EveryFormatPropagatesShortWrites` |
| Narrow/resize/scroll, interruption, output failure and terminal/timer cleanup | `TestM4ViewerNarrowResizeScrollAndInterrupt`, `TestM4InteractiveQuitCancellationAndOutputCleanup`, `TestM4ResizePollerStopsAndJoinsOnCancellation`, `TestM4RepeatedViewerLifecycleReleasesResources`; native console probe below |
| Markdown severity, order, limitations, errors, omissions and hostile content | Reviewed `internal/report/testdata/markdown-m4.golden`; `TestM4MarkdownGoldenAndEscaping`, `TestM4MarkdownBoundsAndBidiControls`, `TestM4TextControlsAndMarkdownWriteFailure` |
| Exclusive compressed-size bound, streaming/member/parse limits and explicit failure | `TestM4ArchiveExactCompressedBoundary`, `TestM4ArchiveBufferCannotBypassSizeGuard`, `TestM4JSONIndependentWorkBounds`, `TestM4DeclaredAndHiddenDecompressionBombAreBounded` |
| Sharing privacy across every member and diagnostics; unsupported fields/values remain unavailable | `TestM4BundleSharingAllowlistAndPresence`, `TestM4UnsupportedSharingValuesAreInconclusive`, source-model coverage guard, offline/error privacy and structured-log CLI cases |
| Corrupt, duplicate, traversal, symlink, unsupported and malformed archives | `TestM4ArchiveHeaderAbuse`, `TestM4ArchiveNamesAndTypes`, `TestM4BundleStrictJSONAndMetadata`, `TestM4DuplicateEntryBelowEntryLimit`, `TestM4StoreDeflateAndCompressedTrailingGarbage`, bounded fuzz run |
| Windows/Linux/macOS evidence survives another receiving OS | `TestM4All172GoldenCapturesRoundTripAcrossOS`, `TestM4EnvironmentAliasUsesCapturedOSAndExactPaths`, executed on Windows and Linux; actual Windows bundle read in network-disabled Linux below |
| Offline collector/home/environment bypass, provenance, flags, exits and saved-verdict independence | `TestM4CLIOfflineEvidenceAndProvenance`, `TestM4CLIOfflineAllExitCodes`, `TestM4CLIConflictsAndReadFailuresAreOffline`, `TestM4SavedVerdictsAreReferenceOnly` |
| Schema compatibility and required structure validation | `TestM4BundleMembersMatchCommittedSchemas`, `TestM4OptionalEmptyAliasesMatchSchemaAndReader`, original diagnose/snapshot schema and built-CLI tests |
| Atomic output failure, cancellation, locked destinations and private staging | `TestM4PublicationEveryStagePreservesDestination`, `TestM4AtomicPublicationAndCancellation`, `TestM4SymlinkDestinationAndInputAreNotFollowed`, `TestM4WindowsAtomicReplaceLockedDestination`, `TestM4WindowsPublicationHasPrivateProtectedACL`, `TestM4CLIBundleStatusFailurePreservesDestination` |
| M1–M3 collectors, rules, JSON, registry, goldens and opt-in probe contracts | Full canonical test/race suites; original 172 goldens and twelve rule pages have no diff |
| Pinned/audited terminal dependencies and local gates | [Dependency review](m4-dependencies.md), canonical gate table above; native M4 GitHub CI remains unchecked |
| Real README screenshot | [m4-terminal.jpg](../images/m4-terminal.jpg), from actual CLI Windows PTY output using committed synthetic identities/paths |
| Measured lifecycle/time/memory limits and honest remaining work | Resource measurements, native evidence and pending-CI/release limits below |

The runtime reader uses exact typed JSON grammar preflight and semantic validation. Schema tests compile
the committed schemas with network loading forbidden. Ordinary report-v1 properties/values are unchanged;
the only compatible addition is optional bundle-source metadata, absent from ordinary live/fixture runs.
Empty optional aliases are accepted by both reader and schema; raw paths and aliases outside 1–4,096 fail.

## Independent archive and evidence bounds

| Resource | Enforced contract |
|---|---|
| Complete compressed ZIP, including directory | Fewer than 2,000,000 bytes; 1,999,999 accepted and 2,000,000 rejected |
| Total streamed uncompressed member bytes | At most 8 MiB |
| Entries | At most eight; five literal required names and optional `diagnostics.json` |
| Manifest / versions | 64 KiB each |
| Snapshot / Markdown | 2 MiB each |
| JSON report / diagnostics | 3 MiB / 512 KiB |
| JSON grammar/work | One document; exact tags; required fields; no unknown/duplicate/case aliases; depth 32, 200,000 tokens |
| JSON scalar/array work | Strings at most 4,096 UTF-8 bytes; arrays at most 4,096 elements; numeric tokens at most 32 bytes |
| PATH / runtimes | 1,024 / 128 |
| Listeners / requested ports | 1,024 each |
| Dependencies / proxies / errors / omissions | 32 / 3 / 32 / 256 |

The reader checks the entire central/local layout before streaming. It accepts regular Store/Deflate
entries, verifies CRC/SHA-256 and cross-member metadata, and rejects unsupported versions/flags, encryption,
ZIP64/disks, extra/comments, overlaps, prefixes/trailing bytes, duplicate/nonliteral names and symlinks.
Deflate byte reading detects trailing compressed garbage. No member is extracted or executed.
Digests detect corruption, not authorship or authentic machine state.

Publication protects the empty owned stage before payload writes, syncs/closes/revalidates it, then
performs same-directory replacement. Injected protect/write/short-write/sync/close/read/validation/rename
and cancellation failures preserve the previous destination and remove the owned temporary file.
A status-stream failure is detected before publication. Atomic namespace replacement assumes a supporting
local filesystem; network filesystem behavior and power-loss durability are not promised.

## Native terminal evidence

The committed manual harness [tests/terminal/main_windows.go](../../tests/terminal/main_windows.go)
calls the production CLI with real console handles. It can inject one transient frame failure and
exercise two owned viewport sizes. [m4-native-terminal.json](m4-native-terminal.json) records the results.

| Scenario | Result | Console modes/cursor after exit |
|---|---|---|
| `q` | Diagnosis exit 2 and plain report | Restored |
| Ctrl+C | Exit 3 and plain report | Restored |
| Injected frame-write failure | Exit 3; injection confirmed; plain fallback attempted | Restored |
| Resize 40×18 → 96×28, then `q` | Both native viewport changes applied; report rewrapped; exit 2 | Restored |

Before/after input mode was 503, output mode 7, and the cursor was visible. Bubble Tea v1.3.10 has no
Windows resize listener, so the adapter owns a 200 ms window-size poller and cancels/joins it on every
return. Model tests also cover 1×1, 2×2, Unicode widths, scroll boundaries and oversized dimensions.

The README image displays actual live Windows PTY ANSI output through a temporary xterm.js 6.0.0
preview. It is not a physical desktop screenshot. The preview server and PTY were stopped after capture;
neither that server nor Node is a product runtime dependency. Native restoration/resize evidence is
independent of the browser display.

## Real Windows capture and offline cross-OS smoke

The final built Windows CLI published a 4,524-byte bundle with structured diagnostics. Its original
diagnosis exit was 1 for existing PATH warnings. Windows and network-disabled Linux offline diagnosis
both returned 3 with identical findings/source metadata: one retained finding and ten explicit omissions.
The captured shell-name value was outside the sharing enum and became a fixed unavailable collector
notice. That is intentional degradation; removing unsupported evidence does not establish health.

The Linux receiving process preserved Windows capture OS/time, current evaluator provenance and opaque
aliases. `--network none` blocked remote network work; the CLI used no collectors. Automated offline
tests also inject panic collectors/home/environment accessors and cover every diagnosis exit and flag
conflict. The saved JSON verdict/Markdown tampering tests confirm evaluation uses current builtin rules.

## Lifecycle, allocations and local performance

With `GOMAXPROCS=4`, 100 synthetic bundle build/read/cancel cycles took **303.2657 ms**; the archive was
7,342 bytes. Goroutines stayed **2 → 2**; retained heap after GC changed **1,797,600 → 887,320 bytes**.
Fifty synthetic viewer quit/cancel cycles took **533.4757 ms**, with goroutines **2 → 2** and retained
heap **304,424 → 285,560 bytes**. The resize-poller cancellation/join test also passed.

Twenty-five hidden decompression bombs were rejected in **1.0419 ms**, allocating **2,140,424 bytes
total**. A 100-iteration bundle build/read benchmark reported **3,002,830 ns/op**, **1,535,852 B/op**
and **11,419 allocs/op**. The sampled retained profile contained runtime and test-schema initialization
allocations; it is a sampled workload result, not proof that every possible input is leak-free.

Ten read-only full Windows diagnoses on `c532dc3` completed in milliseconds:
561.36, 236.06, 240.98, 245.00, 247.62, 247.96, 249.09, 248.41, 242.53 and 245.23.
All returned exit 1 with nine collectors and twelve rules. Windows `GetProcessMemoryInfo` counters,
including successful final reads after exit, reported peak working sets from **13,213,696 to 14,782,464
bytes**. These local warm runs stayed below the 3 s/100 MB targets. They are not cold-host, native macOS
or universal process-memory results. Machine PATH/configuration was not changed.

Profiles, actual local captures, binaries and raw command logs stay beneath ignored `bin/`. Shared
fixtures use synthetic data. The existing Docker data/workloads were preserved; owned Linux test/smoke
containers used `--rm` and left no running M4 containers.

## Reproduce

```powershell
$env:GOTOOLCHAIN='go1.26.11'
$raceCompilerDir=Join-Path $env:LOCALAPPDATA 'Programs/MSYS2-Go-x64/msys64/ucrt64/bin'
$env:CC=Join-Path $raceCompilerDir 'gcc.exe'
$env:CXX=Join-Path $raceCompilerDir 'g++.exe'
$env:PATH="$raceCompilerDir;$env:PATH"
$env:CGO_ENABLED='1'
make test
make race
make lint
make build
make check-docs
govulncheck ./...
go mod verify
git diff --check
```

The Linux race/docs/static gates ran in `golang:1.26.11`, with `GOTOOLCHAIN=local`, `GOMAXPROCS=4`, a
read-only repository mount at `/src`, and an owned ignored output directory mounted at `/qaout`:

```sh
make race
make check-docs
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X main.version=m4-validation' -o /qaout/envdoctor ./cmd/envdoctor
go version -m /qaout/envdoctor
```

The resulting binary was run in a second `--network none` container against the actual Windows bundle.
For bounded fuzzing and allocation measurements:

```powershell
$env:GOTOOLCHAIN='go1.26.11'
$env:GOMAXPROCS='4'
go test ./internal/bundle -run '^$' -fuzz FuzzM4BundleReader -fuzztime=15s -parallel=2
go test -count=1 -run 'TestM4RepeatedBundleLifecycleReleasesResources|TestM4DeclaredAndHiddenDecompressionBombAreBounded' -v ./internal/bundle
go test -count=1 -run 'TestM4RepeatedViewerLifecycleReleasesResources|TestM4ResizePollerStopsAndJoinsOnCancellation' -v ./internal/report
go test -run '^$' -bench '^BenchmarkM4BundleBuildRead$' -benchtime=100x -benchmem -memprofile bin/m4-bundle-mem.pprof -o bin/m4-bundle-profile.test.exe ./internal/bundle
go tool pprof -top -inuse_space bin/m4-bundle-mem.pprof
```

Native console reproduction: build `./tests/terminal` into `bin/m4-terminal-probe.exe`, run it with
`--record`, optional `--resize`/`--fail-frame`, then `diagnose --fixture-dir docs/testing/fixtures/m4-viewer`.
Use a capable console with `TERM=xterm-256color` and `NO_COLOR` absent. The harness only changes its
owned viewport and records modes/cursor; production diagnosis has no testing-only flags.

## Remaining acceptance work

Push the M4 feature branch after publication approval and run native Windows/Ubuntu/macOS GitHub CI,
including lint/security. Keep that checkbox pending until the final branch passes. Native macOS
interactive observation, cold/clean-host performance, packaging and signed releases are not claimed.
Rules ENV013–ENV018 and fix/apply/undo remain later milestones.
