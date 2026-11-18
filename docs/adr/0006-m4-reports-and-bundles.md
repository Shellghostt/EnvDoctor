# ADR 0006 — M4 report formats and shareable diagnostic evidence

- Status: accepted
- Proposed: 2026-11-17; accepted: 2026-11-19
- Required by: [M4 requirements](../phases/M4.md), ENV-024–ENV-026
- Extends: ADRs 0004/0005 with a separate bundle export/offline-input contract

## Context

M3 plain/JSON reports and fixture-directory diagnosis are implemented. Ordinary JSON intentionally
retains effective Git identity. Home/credential-URL redaction is not a sharing policy. Bundles must be
smaller than 2 MB, remove identifying/raw data and preserve the meaning of absent diagnostic evidence.
Saved reports are reference data, not verdicts or instructions to execute on another machine.

## Decision

Implement ENV-024–ENV-026 below. Preserve collectors, twelve builtin rules, registry pages, fixture-v1
input, ordinary JSON and diagnosis exits. No command in this phase applies a fix.

### Formats and terminal lifecycle

- `diagnose --format text|markdown|json` selects an explicit format. `--json` remains a JSON alias.
  `--plain` forces text, as does explicit text. `--json --format json` is compatible; conflicting
  formats, plain with JSON/Markdown and unknown formats exit 4 before collection.
- Without an explicit format/plain choice, use Bubble Tea only when stdin, stdout **and stderr** are
  terminals, TERM is not `dumb`, and NO_COLOR is not set. Missing TERM on Windows does not preclude a
  terminal. Pipes, redirects and non-file test streams fall back to stable plain output. JSON/Markdown
  bypass the viewer. Verbose collector timings remain on stderr.
- Pin `github.com/charmbracelet/bubbletea v1.3.10` and `golang.org/x/term v0.48.0`; commit exact
  transitive module versions/checksums and audit/review licenses before completion.
- The viewer scrolls the completed report, handles narrow dimensions/resize and uses an alternate screen.
  Quit returns the diagnosis exit; interruption/render failure returns 3. Every exit restores terminal
  state and prints the plain report. Output errors cancel the viewer and propagate as failures.
  No viewer action executes advice, collects evidence or changes a finding.
- Text makes terminal controls visible. Markdown emits repository-owned structure and escapes dynamic
  punctuation/controls, including HTML, links, fences and commands/paths. Include severity/ID, evidence,
  limitations, manual advice/verification, degraded collectors and skipped prerequisites from the same
  report/registry. Missing evidence is not displayed as healthy.

### Separate sharing snapshot

Bundle snapshots have a separate version-1 typed allowlist. Convert approved facts into existing pure
engine input; generate JSON/Markdown/versions from that evidence, never from saved finding text.

| Captured field | Sharing representation and rule effect |
|---|---|
| Timestamp/platform | Original UTC capture time; known OS/architecture, elevation and bounded numeric version metadata. Unknown platform stays absent/degraded. |
| Shell | Known shell-name enum; path removed. Unknown names are unavailable with an omission. |
| PATH | Order/existence/directory/duplicate facts; raw locations become capture-local opaque path tokens. |
| Runtimes | Python/Node names, numeric versions, known manager/resolution enums and active/expected/missing facts. Exact path aliases preserve comparisons; separate normalized environment aliases preserve ENV011 equality under the captured OS. |
| Requested ports/listeners | Port/protocol/state facts; PID/process labels omitted. Ownership detail is unavailable while ENV005 can still detect an occupied requested port. |
| Docker | Installed/local probe/API-range/override facts and numeric versions. Selected context names/endpoint URLs omitted; they never select a connection target. |
| Git | Version, identity-checked and nonempty-name/email **presence booleans**, autocrlf policy and opaque origin markers. Reconstruct fixed presence markers for ENV010; redaction is not an unset identity. |
| Dependencies | Supported tool/library names, present/missing/unknown status, opaque paths/project/requirements markers and allowed Git policy. Keep supported package-manager names/exact versions and known lockfile names. |
| Python environment | Metadata-known and capture-local canonical environment tokens; no receiving-host canonicalization or lookup. |
| Proxies | Known variable/scheme/port/probe facts; original host/URL become capture-local `.invalid` aliases. No userinfo, hostname, path, query or fragment. Offline evaluation never resolves/dials aliases. |
| Failures/timings | Known collector identifier, bounded milliseconds and a fixed unavailable notice. Original errors/stdout/stderr/free-text logs omitted. |
| Report/tool metadata | Fixed tool name and normalized version metadata. Captured findings/advice come from the builtin registry after export, not the original report's free text. |

Aliases are deterministic in field traversal order and local to a capture; they contain no hash or raw
path/identity. They preserve only the documented equality and are not filesystem locations. Omissions
use fixed field/reason identifiers. Removing required evidence marks that collector unavailable and
skips dependent rules under existing prerequisites. Removing non-prerequisite labels stays visible
without fabricating an error. Unknown/future export fields fail closed; unsupported values are omitted
or rejected rather than copied through.

No raw environment values, tokens, usernames/emails, hostname, home paths, process arguments, project
contents or arbitrary logs are exported. This is a current-field allowlist, not universal secret
detection. Numeric versions/ports, platform and diagnostic facts still describe the captured setup.

### Bundle v1 and independent bounds

Required ZIP members: `manifest.json`, `snapshot.json`, `report.json`, `report.md`, `versions.json`.
Optional `diagnostics.json`: `bundle --include-logs` means only allowlisted structured collector
statuses/timings, never shell history, files or command output. Every member uses the sharing policy.

The manifest identifies bundle/snapshot/report/redaction/tool versions, original capture time and
diagnosis exit, omissions, and every other member's exact size/SHA-256. It excludes its own digest.
Digests detect corruption; they do not authenticate a machine/author. The exported report exposes the
original exit as reference metadata and its recomputed exit, including omitted-prerequisite degradation.

| Limit | Contract |
|---|---|
| Compressed archive, including directory | Fewer than **2,000,000 bytes** |
| Total decompressed members | At most **8 MiB** |
| Entries | At most eight; exactly five required names plus optional diagnostics |
| Manifest / versions | At most 64 KiB each |
| Snapshot / Markdown | At most 2 MiB each |
| JSON report / diagnostics | At most 3 MiB / 512 KiB |
| JSON work | One document; no duplicate/unknown fields; depth ≤32, tokens ≤200,000, strings ≤4,096 UTF-8 bytes; arrays ≤4,096 elements |
| Evidence collections | PATH ≤1,024; runtimes ≤128; listeners/requested ports ≤1,024; dependencies ≤32; proxies ≤3; errors ≤32; omissions ≤256 |

Preflight file/directory limits before constructing an unbounded ZIP/JSON graph; enforce actual
streamed decompressed sizes. Accept only Store/Deflate regular entries. Reject encryption,
ZIP64/multi-disk/unsupported flags, extra/comment records, duplicate/unknown or nonliteral names,
directories/symlinks, absolute/drive/UNC/backslash/traversal paths, inconsistent local/central headers,
overlaps, size/digest/CRC mismatches and trailing data. Never extract. Validate required structures and
cross-member capture metadata before evaluation.

An oversized capture fails explicitly without truncation, partial publication or relaxed limits.
Stage a private temporary file beside the requested destination, finish/sync/close and validate it,
then replace the named regular destination by same-directory rename (Windows replacement API).
Never follow a destination symlink. Failure/cancellation removes only the owned temporary file and
keeps the previous destination. Atomic namespace replacement assumes a supporting local filesystem;
network-filesystem semantics and power-loss durability are not guarantees of this format.

### Commands, provenance and exits

- `bundle --output FILE` defaults to `envdoctor-bundle.zip`; `-o` is an alias. Existing capture controls
  `--fixture-dir`, `--port`, `--probe-network`, `--package-manager` and optional `--include-logs` apply.
  It captures the complete builtin report; format/rule filters do not select archive members.
  Successful publication exits 0 and prints bounded status/byte count, even for captured warnings/errors/
  degradation. Capture/archive/cancellation/output/I/O failures exit 3; usage exits 4.
- `diagnose --from-bundle FILE` validates and evaluates sharing evidence using the current builtin registry.
  It runs no collectors, PATH/Git/Docker/project commands, home/path inspection or network work. Preserve
  capture time/platform, label capture-tool/current-evaluator versions, and expose omissions. Saved
  verdicts/suggestions/Markdown are reference members and cannot influence evaluation.
- Bundle input conflicts with fixture input and requested-port/network-probe/package-manager live flags.
  Format/component/rule/verbose filters remain available. Fixture behavior, including its existing
  requested-port selection, stays unchanged. Invalid combinations fail before I/O.
- Malformed/unsupported/bounded-out/corrupt input and read failures exit 3; usage exits 4. Valid offline
  evaluation preserves 0/1/2/3, including 3 for unavailable required evidence. Bundle errors have fixed
  reason text and do not interpolate untrusted fields/member names/captured errors.
- Ordinary JSON remains schema v1 with existing fields/values and Git identity. Optional bundle-source
  metadata is absent from ordinary runs and documented as a compatible report-schema addition.

## Alternatives considered

- Zip ordinary JSON: retains identity/unreviewed free text and fails the sharing contract.
- Remove paths/identities without presence/equality/omissions: causes false missing-setting or healthy results.
- Extract ZIP or run captured commands: unnecessary filesystem/execution authority; pure evaluation suffices.
- Reuse saved verdicts: loses evaluator provenance and turns untrusted instructions into findings.

## Consequences and validation

Renderer/Markdown cases precede export. Add reviewed redaction/archive-abuse cases before writer/reader
and offline CLI. Verify all original 172 cases, strict schemas, deterministic cross-OS evidence, privacy
across every member/error, exact size boundaries and rejection cleanup. Measure repeated archive/viewer
lifecycles using synthetic captures. Record native terminal/Windows/Ubuntu/macOS CI, dependency/licenses
and a real interactive screenshot separately from cross-builds/model-only tests. Check acceptance boxes
only after named evidence exists.
