# Golden files

One JSON fixture per rule, named after the rule id: `ENV004.json`. A fixture contains a captured
collector snapshot and the exact expected diagnosis. Rules are tested against these files rather than
against live machine state.

Real machine output is never committed here without redaction (usernames, hostnames, tokens).
