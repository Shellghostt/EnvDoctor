# Golden files

Fixtures are named by rule, OS and case, for example `ENV007-windows-positive.json`. Each JSON file
contains a synthetic collector snapshot and exact expected findings. Pure rules are tested against
these files rather than live machine state; integration tests also run every case through the built CLI,
checking findings, JSON schemas and exit codes.

There are 172 cases: all 54 original M2 fixtures remain unchanged, with 118 additional cases covering
ENV007–ENV012 and expanded ENV006 failure states. Every rule has positive, negative and edge cases
for Windows, Linux and macOS. Additional cases exercise API overrides/malformed bounds, explicit
dependency absence, unsupported Windows arm64 OpenSSL, conflicting lockfile families, unknown runner
versions, explicit Git policy, canonical venv paths and degraded proxy probes.

Expected findings are reviewed evidence. Do not regenerate expectations from production rule output to
make a failing test pass. A missing rule or a rule without fixtures fails the harness. Fixtures remain
bounded, versioned data; fixture diagnosis rejects live-evidence options and invokes no collectors.

Real machine output is never committed here without redaction (usernames, hostnames, tokens).
