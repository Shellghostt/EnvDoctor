# ADR 0005 — M3 evidence, bounded network opt-in and rule metadata

- Status: accepted for the M3 feature branch
- Date: 2026-11-15
- Supersedes: ADR 0004 decisions 4–6 and 8 only for the extensions below;
  ADR 0002's six-rule registry limit. Existing rule IDs, offline defaults and exit codes remain.

## Decisions

1. Docker still probes the **default local** Unix socket or Windows engine pipe, with a trusted absolute
   CLI, fixed arguments and a sanitized environment. Selected context metadata is read locally from the
   standard home-directory Docker configuration and the context's SHA-256 metadata directory. Context
   and host overrides are recorded as data; they never select the probe endpoint. Alternate Docker
   configuration directories are unsupported. A TCP/SSH context cannot cause a remote connection.
2. ENV007 uses API numbers, not Docker product versions. The daemon advertises its maximum `ApiVersion`
   and, when available, `MinAPIVersion`. The client advertises its API version and optional minimum.
   A client maximum below the server minimum, or an explicitly known client minimum above the server
   maximum, proves disjoint ranges. An explicit `DOCKER_API_VERSION` outside the known daemon range
   also proves incompatibility. A newer client maximum alone does not: Docker supports negotiation.
   Unknown/malformed bounds remain inconclusive. A successful negotiated API inside the daemon's
   range is compatible. Sources: Docker Engine API version matrix and API negotiation documentation
   (<https://docs.docker.com/reference/api/engine/>), and the CLI's `docker version` JSON fields.
3. Local Docker states distinguish CLI absent, available, unavailable, permission denied, unsupported
   API and unknown metadata. Only a completed, recognized local connection failure supports ENV006;
   denial, unknown subprocess failures and malformed metadata degrade inspection. Older M2 snapshots
   without the new status retain their established ENV006 interpretation.
4. The initial dependency catalog is deliberately explicit. The project marker
   `envdoctor.requirements.json` is bounded to 1 MiB, strictly decoded, versioned at 1, and declares
   `requirements` with `kind`, `name` and optional `path`. Supported tools are `cmake`, `make`, `gcc`
   and `pkg-config`: inspect executable file/symlink metadata on absolute PATH entries, never run them.
   A complete, successful search can establish that the required executable is absent from PATH;
   denied metadata or relative PATH ambiguity is unknown. Supported library `openssl` requires an
   explicit installation prefix (`path`, relative to the project or absolute). Inspect `include/openssl/ssl.h`
   plus both SSL and crypto development libraries: Linux `.so`/`.a` in `lib` or `lib64`, macOS `.dylib`/`.a`
   in `lib`, Windows `.lib` in `lib` or `lib/VC/x64/MD` (amd64 only). Missing files establish absence
   **at that declared prefix**, not absence everywhere on the machine. Unsupported names/layouts/
   architectures are unknown. Presence is metadata evidence, not proof a compiler can link the SDK.
   Source files, language extensions and undeclared transitive dependencies do not imply requirements.
5. In the working directory, bounded `package.json` metadata supplies an exact `packageManager`
   declaration (npm/pnpm/yarn/bun with an exact numeric version; optional integrity suffix). Observed
   lockfiles are `package-lock.json`, `npm-shrinkwrap.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lock`
   and `bun.lockb`. `--package-manager <name>[@<version>]` explicitly selects a runner; an installed
   npm executable alone does not. ENV009 distinguishes conflicting lockfile families, declaration/
   runner disagreement, version disagreement when both versions are known, and selected manager/
   lockfile disagreement. Missing/unsupported declarations and unknown versions are inconclusive.
6. Git records effective identity, whether it was checked, and the config origin from fixed
   `git config --null --show-origin --get` invocations. Missing keys remain distinct from failures.
   The requirements file may explicitly declare `gitAutocrlf` (`true`, `false`, `input`). Only a
   conflict with that policy supports autocrlf advice. The OS alone does not, and an `eol` attribute
   overriding autocrlf is not itself a conflict. Git identity remains visible by the M2 contract;
   origin paths are redacted. Invalid config is degraded.
7. ENV011 requires an absolute `VIRTUAL_ENV`, readable `pyvenv.cfg`, a known canonical environment
   directory and a selected interpreter with known file resolution. Compare invocation environment
   metadata, preserving the distinction between a venv symlink and its shared system interpreter.
   Normalize separators, Windows case and resolved environment-directory symlinks. Known shims and
   unresolved metadata remain inconclusive; never execute an interpreter or shim.
8. `--probe-network` is the only proxy reachability opt-in and is incompatible with `--fixture-dir`.
   Without it, collect HTTP_PROXY, HTTPS_PROXY and ALL_PROXY (uppercase takes precedence over lowercase)
   as redacted endpoint data, with reachability `untested`. Accept HTTP/HTTPS/SOCKS5/SOCKS5H URLs with
   a host and valid port; default ports are 80/443/1080. Strip userinfo, path, query and fragment.
   NO_PROXY is represented only by whether it is configured. With opt-in, probe at most three distinct
   configured host/port pairs, with a one-second total deadline within the collector's two-second
   deadline. Each endpoint gets at most one DNS lookup and one TCP dial to one resolved address.
   No credentials, HTTP requests, TLS handshakes, external targets or scanning. Close each connection.
   Refusal/no-route is unreachable evidence. DNS failures, denial, timeout, cancellation and invalid
   URLs are unknown/degraded, not proof that the proxy is down. Duplicate endpoints share a result.
9. Optional snapshot fields extend schema v1 without making old fields newly required. Fixtures remain
   strict, bounded data and never invoke collectors. A collector can return typed, certain partial
   evidence with explicit `Part.Errors`; that evidence is retained with degraded notices/timings, and
   dependent rules are skipped. An ordinary returned error still discards the part, as in M2.
   Redact credential-bearing URLs and home paths in evidence, errors and CLI notices even when home
   discovery fails. New fields do not authorize disclosing arbitrary project/environment contents.
10. One immutable builtin registry owns each rule's ID, primary component, collector prerequisites,
    severity, title, evidence description, explanation, manual steps, verification, limitations and
    documentation link, along with its pure evaluator. Evaluation, `explain`, `rules --json` and
    generated `docs/rules/ENV*.md` use that registry. Registry JSON has `schemaVersion: 1` and an
    ordered `rules` array, documented by its own schema. Metadata commands run no collectors.
    Unknown IDs/invalid arguments exit 4; output failures exit 3. `go run ./cmd/rulegen` generates
    pages and `go run ./cmd/rulegen --check` detects drift, also enforced by CI/tests.

## Consequences

ENV007–ENV012 join the existing six rules without changing their ordering, filtering, fixture or
exit-code contracts. Unsupported evidence limits coverage rather than generating speculative findings.
The dependency manifest is project evidence, not executable configuration or an automatic repair plan.
M4 reporting/bundles, ENV013–ENV018, fixes and packaging remain later scope. Cross-platform fixtures,
real read-only inspection, cancellation/socket tests, registry/schema checks and canonical Go gates
must pass before readiness documentation can claim completion. Local cross-compilation is not native
three-OS CI execution.
