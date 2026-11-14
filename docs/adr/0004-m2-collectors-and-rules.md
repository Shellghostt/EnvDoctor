# ADR 0004 — M2 collection, rule evidence and offline diagnosis

- Status: accepted for the M2 feature branch
- Date: 2026-11-14
- Supersedes: ADR 0002 decision 5 only for quoted paths and embedded error paths

## Decisions

1. Runtime discovery reads absolute PATH entries, file/symlink metadata, known version-manager paths,
   Windows PE version resources and `pyvenv.cfg`. It never executes an interpreter or shim discovered
   on PATH. Unknown versions stay empty. A numeric directory alone is insufficient evidence of a version.
   Python and python3 have independent active resolutions. Broken symlinks cannot shadow a usable later
   candidate; venv invocation paths remain distinct from their shared system interpreter target.
2. Expected Python comes from `VIRTUAL_ENV`; expected Node comes from `NVM_BIN` or `NVM_SYMLINK`.
   Manager labels cover observable pyenv/nvm/fnm/volta/venv paths. This is metadata-based discovery,
   not execution of version-manager commands or proof that every shim selects a working runtime.
3. A port is a conflict only when explicitly requested with `--port`. Collectors read Linux `/proc`,
   macOS lsof field output, or Windows `GetExtendedTcpTable` and query-only process handles. Ownership
   may be absent when the OS denies access. Missing inspection tools cause degradation, not a clean result.
4. Git and Docker CLIs are invoked only from known absolute installation locations with fixed arguments,
   bounded output and a collector deadline. PATH is not used to find them. Git reads effective identity
   and autocrlf configuration; an absent key is distinct from an invalid config. Git/Docker environment
   overrides are removed from child processes so the probe cannot select an injected repository or remote
   Docker context. Git identity remains visible in JSON and should be reviewed before sharing reports.
5. ENV006 belongs to M2 even though the full Docker inspector backlog is M3. M2 therefore implements the
   minimum local-default engine probe needed by that rule. It distinguishes a CLI not found in known
   locations from a present CLI whose local engine is unreachable. Remote contexts, version-skew rules
   and the broader Docker/dependency inspection remain M3. A failed/timed-out probe is degraded.
6. ENV001–ENV006 are pure snapshot functions with stable IDs, deterministic evidence and manual fix
   suggestions. Every rule has positive, negative and edge fixtures for all three operating systems.
   ENV005 verification includes the relevant requested ports. Import/AST checks prohibit rule I/O.
7. `--fixture-dir` loads one bounded, versioned `snapshot.json` and invokes no collectors. Unknown fields,
   additional JSON documents, absent capture timestamps and unsupported versions are rejected. Captured
   collector errors still yield notices and exit 3. Platform facts can be absent after a failed collector;
   the snapshot schema now permits that documented degraded state.
8. Home redaction also covers quoted raw PATH entries and home paths embedded in error messages. Both
   snapshot errors and timing notices are redacted before output. This is home-path redaction, not a
   general secret/identity scrubber; the later bundle feature must establish its own export policy.
   Unexpected PATH metadata failures (including access denial) degrade the collector; they are not
   evidence that a directory is missing. OS errors for malformed paths, overly long paths or a
   non-directory ancestor are broken-entry evidence and preserve the raw entry for ENV003.
9. The M1 runner also bounds its wait when a collector ignores cancellation. Late results use buffered
   channels and are not merged. A blocked kernel filesystem call cannot be forcibly canceled, but cannot
   hold up the CLI after the deadline. No diagnostic command changes the machine or applies a fix.

## Consequences

Six rules ship with an actionable plain-text report and a versioned JSON report. A healthy report exits 0,
warnings exit 1, errors exit 2, incomplete inspection exits 3, and invalid usage exits 4. Rule/component
filters still filter findings rather than collectors (ADR 0002 decision 4). The full test suite runs in
the three-OS CI matrix; local cross-compilation is separate from execution on an actual macOS host.
