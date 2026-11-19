# Architecture Decision Records

Each file records one decision that was expensive to make and would be expensive to reverse.

| ADR | Title | Status |
|---|---|---|
| [0001](0001-initial-decisions.md) | Initial technology and scope decisions | accepted |
| [0002](0002-m1-implementation-decisions.md) | M1 implementation decisions | accepted, partly superseded by 0003 and 0005 |
| [0003](0003-go-126-floor-and-ci-contract.md) | Go 1.26 floor and CI contract | accepted |
| [0004](0004-m2-collectors-and-rules.md) | M2 collectors and rules | accepted, scoped extensions in 0005 |
| [0005](0005-m3-evidence-and-rule-registry.md) | M3 evidence, bounded network opt-in and rule metadata | accepted; merged M3 |
| [0006](0006-m4-reports-and-bundles.md) | M4 report formats and shareable diagnostic evidence | accepted; implementation in progress |

New ADRs are numbered sequentially and never edited after acceptance. A changed mind gets a new ADR
that supersedes the old one.
