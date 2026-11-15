# EnvDoctor M3 validation — 2026-11-15

M3 is complete on `feat/m3-implementation`, based on merged M2
`c7e2c84`. [ADR 0005](../adr/0005-m3-evidence-and-rule-registry.md) was accepted and committed
as `3e22674` before implementation. Implementation commit: `d185e5b`.
This record covers the final M3 source, not a release or merge.
The feature branch was pushed on 2026-11-15. [CI 43723839978](https://github.com/Shellghostt/EnvDoctor/actions/runs/43723839978)
passed for `fbf79dd`: native Windows, Ubuntu and macOS race/test/build/documentation checks, lint and
the vulnerability scan. The run completed at 14:21:34 UTC with all five jobs successful.

## Environment and gates

Windows 11 amd64 on an AMD Ryzen 5 5600H, Go 1.26.11, GNU Make 4.4.1, golangci-lint 2.15.0,
and govulncheck 1.9.0. The vulnerability database reported an update of 2026-11-11.
The host's default Go is newer; every local Go gate pins `GOTOOLCHAIN=go1.26.11`.
Native race tests use the existing MSYS2 UCRT64 GCC 16.2 toolchain targeting
`x86_64-w64-mingw32`. Go's CC/CXX point to its absolute GCC/G++ paths beneath
`%LOCALAPPDATA%/Programs/MSYS2-Go-x64/msys64/ucrt64/bin`.

| Check | Final result |
|---|---|
| `make docs` | Passed; all twelve generated page hashes unchanged on regeneration |
| `make check-docs` | Passed; registry and committed/generated pages agree |
| `make test` | Passed, including 172 exact fixtures and built-CLI schema/exit checks |
| Native Windows `make race` | Passed, all packages |
| `make lint` | Passed: gofmt, vet and golangci-lint; zero issues |
| `make build` | Passed; Windows amd64 binary, `CGO_ENABLED=0`, 4,379,136 bytes |
| Linux `go test -race -count=1 -timeout=120s ./...` | Passed on the final source in `golang:1.26.11`; built-CLI integration package took 10.491 s |
| Linux static build | Passed; build metadata confirms linux/amd64 and `CGO_ENABLED=0` |
| macOS static cross-builds | Passed for amd64 and arm64; 4,140,384 and 3,929,250 bytes, respectively |
| `govulncheck ./...` | Passed on final source: no vulnerabilities found |
| `git diff --check` | Passed |
| CodeGraph refresh/status | Current: 75 files, 819 nodes, 1,804 edges; local index is ignored |
| M3 three-OS GitHub CI | Passed for `fbf79dd`: native Windows/Ubuntu/macOS race, tests, builds and docs checks, plus lint/security; [run 43723839978](https://github.com/Shellghostt/EnvDoctor/actions/runs/43723839978) |

`make build` disables CGO only for that target, preserving the static binary contract without disabling
the race detector for `make race`. Cross-compilation confirms compilation; the CI matrix supplies
native macOS execution evidence.
The original 54 M2 fixture JSON files are unchanged; 118 additional cases bring the total to 172.

## Acceptance evidence

| Requirement | Tests and evidence |
|---|---|
| ENV007–ENV012 positive/negative/edge cases on applicable OSes | `TestRegistryPagesAndFixturesCoverEveryBuiltin`, all `tests/golden/ENV007`–`ENV012` cases and `TestBuiltCLIWithAllGoldenFixtures`; Windows arm64 OpenSSL is explicitly inconclusive |
| Preserve all 54 M2 golden cases and pure rule evaluation | Existing fixtures, `TestBuiltinRulesOnlyImportPureDependencies`, `TestM3RuleEvaluationIsDeterministicAndDoesNotMutateSnapshot`, `TestUnknownResolutionDoesNotCreateM2Findings` |
| Distinct Docker absent/down/denied/remote/API states | `TestDockerLocalProbeKeepsRemoteContextAsMetadata`, `TestDockerHostOverrideAndInspectionStates`; expanded ENV006 fixtures; ENV007 compatible, incompatible, override, malformed and remote-context cases |
| Explicit, reliable dependency evidence | `TestDependenciesRequireExplicitManifestAndReliableAbsence`, `TestOpenSSLDeclaredPrefixNeedsHeaderAndBothLibraries`, `TestProjectParsingIsBoundedStrictAndNeverRunsScripts`, ENV008 missing/present/denied/no-declaration cases |
| Package-manager and virtualenv decisions avoid guessing | ENV009 declaration/runner/version/conflicting-family cases; ENV011 case normalization/shared-system-target/unknown-environment cases; `TestUnrecognizedPythonTextShimIsInconclusive`, `TestNativeExecutableDoesNotTreatTextShimsAsResolvedInterpreters` |
| Effective Git identity, origins and explicit line-ending policy | `TestGitEffectiveValuesIncludeConfigOrigins`, `TestGitBooleanAliasesUsePortableTypedConfigQuery`, ENV010 fixtures; live Git 2.53.0.windows.2 inspection |
| Offline normal/fixture/metadata commands | `TestNormalDiagnosisDoesNotConnectToRemoteOverrides`, `TestMetadataCommandsAreOfflineAndMatchRegistryAndPages`, `TestProxyCollectionIsOfflineRedactedAndBounded`, existing fixture collector-bypass tests |
| Bounded opt-in networking and resource cleanup | `TestProxyFailuresStayUnknownAndCancellationStopsFurtherDials`, `TestProxyDNSIsBoundedByCancellation`, `TestProxyProbeSendsNoApplicationBytesAndClosesConnection`, `TestRepeatedProxyLifecycleReleasesResources`; end-to-end loopback smoke below |
| Redaction, schema compatibility and stable output/exit codes | `TestNewEvidenceAndNoticesAreRedactedEvenWithoutHomeDiscovery`, `TestRedactionCoversHomePathsInsideURLsAndCredentialURLPaths`, `TestProxyValidationAndEnvironmentPrecedence`, `TestMetadataWriteErrorsUseToolFailureExit`, built-CLI schema and fixture checks |
| Registry/explain/JSON/page consistency and immutability | `TestMetadataCommandsAreOfflineAndMatchRegistryAndPages`, `TestMetadataCopiesCannotMutateRulesOrDocumentation`, page hash comparison and `make check-docs` |
| Degraded partial evidence cannot trigger dependent rules | `TestFailedCollectorPrerequisitesDisableEveryM3Rule`, partial-result collector/CLI cases and fixture error/exit-code checks |

The built CLI validates the version-1 snapshot/report contract for all fixtures. Metadata tests validate
the new version-1 registry schema, and project tests validate the version-1 requirements schema and strict
collector parsing. Findings remain ordered, snapshot-only and manual; no diagnosis applies a fix.

## Read-only Windows smoke

Ten consecutive full diagnoses completed without degraded collectors, with nine collectors and twelve
registered rules. Durations in milliseconds were 390.93, 186.28, 196.89, 186.22, 191.68, 189.57,
194.06, 192.39, 204.12 and 193.47. Exit 1 reflected existing PATH warnings. These are local observations,
not clean-host or three-platform M6 performance results. Machine PATH was not changed.

Docker Desktop returned CLI 29.9.2, client/server API 1.56 and server minimum API 1.40.
The selected context was `desktop-linux`; the default local probe was available.
An owned TCP listener on port 46852 was identified as `pwsh.exe`, PID 25232, producing exit 2 for ENV005.
That disposable listener was stopped.

An end-to-end `--probe-network` test pointed HTTP_PROXY, HTTPS_PROXY and ALL_PROXY at one disposable
loopback listener. Exactly one connection was accepted and zero application bytes arrived. Credential
text was redacted and the CLI exited 0. The listener was closed and process environment restored.

A disposable project declared a required `cmake` and an OpenSSL prefix beneath that project. With PATH
temporarily set to an absolute empty directory in the smoke process, ENV008 returned exit 2 and one finding
with both missing requirements. A `package.json` declaring `pnpm@9.0.0`, a `pnpm-lock.yaml`, and an explicitly
selected `npm@10.0.0` runner produced ENV009 and exit 1. A harmless install hook that would write a sentinel
never executed. The process PATH was restored, and the four known smoke files and two empty directories
were removed without recursive cleanup.

## Lifecycle and allocation evidence

The bounded Windows loopback lifecycle workload ran 100 proxy cycles in 31.77 ms. Goroutines changed
from 3 to 2; retained heap after GC changed from 377,944 to 340,936 bytes. A 1,000-cycle benchmark reported
320,839 ns/op, 2,408 B/op and 39 allocs/op. M3 rule evaluation reported 2,582 ns/op, 1,945 B/op and
16 allocs/op. A sampled retained heap profile contained runtime/testing allocations and no sampled retained
socket allocations. Profiles and compiled test binaries stay local beneath ignored `bin/`.

These results cover repeated creation/closure in this workload. They do not establish universal absence
of leaks or a process RSS budget. Native macOS allocation benchmarks/profiles and M6 clean-host budgets
were not recorded. MemGuard and Eclipse MAT do not analyze this Go runtime and were not used as gates.

## Corrections found during verification

- The inherited subprocess output buffer embedded `bytes.Buffer`, exposing its `ReadFrom` method and
  allowing `io.Copy` to bypass the bounded `Write`. A named private buffer closes that bypass; stdout
  and stderr limit regressions pass.
- Protected WindowsApps/MSIX paths and application aliases remain unknown metadata rather than being
  opened as ordinary interpreters or described as missing. M2 rules ignore explicitly unknown resolution
  in new snapshots while preserving the original snapshots' behavior.
- Git autocrlf inspection uses portable raw queries and the longstanding `--type=bool` query for boolean
  aliases. Literal `input` is handled directly; inspection does not depend on newer `bool-or-str` support.

## Reproduce

Run native gates with an amd64 MinGW-w64-compatible compiler configured for Go race tests:

```powershell
$env:GOTOOLCHAIN='go1.26.11'
make docs
make check-docs
make test
make race
make lint
make build
govulncheck ./...
git diff --check
```

The final Linux run used a cached Go image and a read-only repository mount, with no persistent volumes:

```powershell
docker run --rm --name envdoctor-m3-race `
  --mount 'type=bind,source=D:\Dev Repo\EnvDoctor,target=/src,readonly' `
  -w /src -e GOTOOLCHAIN=local -e CGO_ENABLED=1 golang:1.26.11 `
  sh -c 'go test -race -count=1 -timeout=120s ./... && CGO_ENABLED=0 go build -trimpath -o /tmp/envdoctor ./cmd/envdoctor && go version -m /tmp/envdoctor'
```

The container removed itself on completion. Existing Docker workloads and data were preserved.
For optional local profiling:

```powershell
$env:GOTOOLCHAIN='go1.26.11'
go test -run '^$' -bench '^BenchmarkProxyLifecycle$' -benchtime=1000x -benchmem -memprofile bin/m3-socket-mem.pprof -o bin/m3-system-profile.test.exe ./internal/system
go tool pprof -top -inuse_space bin/m3-socket-mem.pprof
go test -run '^$' -bench '^BenchmarkM3RuleEvaluation$' -benchtime=1000x -benchmem ./internal/diagnostic
```

## Evidence limits

The initial catalog checks `cmake`, `make`, `gcc` and `pkg-config` on absolute PATH entries, and OpenSSL
development files at a declared prefix. Relative PATH entries, denied metadata, unsupported libraries
and unsupported Windows architectures remain unknown. Presence does not prove an SDK can link.
ENV009 requires observed supported lockfiles and explicit manager evidence, or conflicting lockfile
families. Installed npm alone and unknown manager versions cannot prove a mismatch.

Docker probes its default local pipe/socket, regardless of selected context or host overrides. Custom
Docker config directories and remote/rootless endpoints are not probed. API incompatibility requires
known bounds; differing Docker product versions alone do not establish it. Python discovery never runs
interpreters or shims. Effective Git identity remains visible in JSON under the M2 sharing contract.

Proxy reachability is off by default. Opt-in allows at most three distinct configured endpoints, one DNS
lookup and one TCP dial per endpoint, with a one-second total deadline. Refusal/no-route is evidence;
DNS failures, denial, timeout and cancellation are unknown/degraded. A successful TCP connection does not
prove proxy authentication or forwarding works. M4 bundles, ENV013–ENV018, automated fixes and releases
remain later work.
