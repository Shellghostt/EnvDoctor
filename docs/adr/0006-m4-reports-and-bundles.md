# ADR 0006 — M4 report formats and shareable diagnostic evidence

- Status: proposed; documentation only
- Date: 2026-11-17
- Required by: [M4 requirements](../phases/M4.md), ENV-024–ENV-026
- Would extend: ADRs 0004/0005 with a separate bundle export/offline-input contract

## Context

M3 plain/JSON reports and fixture-directory diagnosis are implemented. Report JSON intentionally retains
Git identity; home/credential-URL redaction is not a general sharing scrubber. The PLAN requires zip
bundles below 2 MB without raw environment values, tokens or usernames, yet does not specify a format,
decompression limit or how removed evidence affects pure rules. A saved report is not evidence of the
receiving machine, and bundled suggestions are untrusted strings rather than commands to run.

## Recommended design, pending acceptance

1. Use one renderer selection path over the existing report. Propose `--format text|markdown|json`, with
   current `--json` retained as an alias and an explicit plain choice for terminal use. JSON/Markdown
   never enter Bubble Tea. Default non-TTY output stays plain and stable; dumb terminals/NO_COLOR use
   plain fallback. Conflicting explicit formats exit 4 before collection.
2. Add a separate versioned allowlisted export model and redaction policy. Export only bounded diagnostic
   facts with identifying/free-text fields sanitized or omitted. Generate every report member from that
   export. Preserve omissions as explicit evidence limitations; dependent rules cannot treat redaction
   as proof that a setting is missing. Existing ordinary JSON/fixture formats remain compatible.
3. Bundle v1 is a zip with manifest, snapshot, JSON/Markdown reports and normalized tool versions; optional
   diagnostics are the tool's own allowlisted structured notices. No arbitrary log files are read.
   The manifest records capture/tool/schema/redaction versions and member sizes/digests. It is integrity
   metadata, not a machine attestation or authentication mechanism.
4. Publish only complete archives below 2,000,000 compressed bytes, at most eight entries and 8 MiB total
   decompressed data. Bound each member and JSON parser separately. Reject overflow rather than truncating
   evidence or relaxing the budget. Read expected members directly without filesystem extraction.
5. `--from-bundle` evaluates exported captured evidence under the current registry, records capture and
   evaluator versions and exposes omissions. It executes no collectors/commands/network work and cannot
   trust a saved verdict. Live-evidence flags and fixture input are mutually exclusive with bundle input.
6. Preserve diagnosis exits 0/1/2/3/4. Propose writer exit 0 for a successfully published archive, storing
   the capture's own exit code inside it; usage is 4 and archive/tool/I/O failures are 3. Valid offline
   diagnosis can still exit 3 for captured degradation or missing required export evidence.

## Choices that must be resolved before acceptance

- Field-by-field export schema for all twelve rules, including Git identities, proxy endpoints, runtime
  and dependency paths, errors and rule prerequisites. Decide where opaque local replacements preserve
  comparison and where omission requires explicit inconclusive evaluation.
- Report/source metadata and schema-version compatibility, capture-local replacement stability, JSON
  duplicate/depth/token/string limits and exact manifest/member schemas. Do not alter fixture v1 silently.
- Per-member limits and supported zip methods/features; bounded directory preflight, size/integrity
  validation, archive errors and output replacement behavior on Windows and POSIX.
- Final format/plain flags, TTY stream detection, NO_COLOR precedence, cancellation semantics and the
  writer-vs-diagnosis exit distinction. Document current JSON behavior before adding new formats.
- Pinned Bubble Tea/terminal dependencies, Markdown escaping, native terminal test method, and whether
  optional structured diagnostics can meet the same export policy. Unsupported log export stays absent.

## Alternatives considered

- Zip the current report: retains Git identity and unreviewed free text; does not meet bundle policy.
- Remove fields without omission metadata: can turn a real configuration into a false missing-setting
  finding or a falsely healthy result when dependent evidence disappears.
- Extract a supplied zip before diagnosis: introduces path/symlink hazards with no need for extraction.
- Run captured verification commands: lets untrusted bundles execute on the maintainer's machine and
  confuses captured evidence with a new observation. Pure offline evaluation is sufficient.

## Consequences and validation

ENV-024 explicitly includes Markdown. ENV-025 must implement a strict writer and reader plus an evidence
sharing policy before ENV-026 can reuse them. Size/redaction/abuse/cross-OS fixtures, terminal cleanup,
existing JSON/golden/probe regressions and native CI gates are required by the phase document.
This record remains a proposal and does not authorize edits to accepted ADRs or claim implementation.
