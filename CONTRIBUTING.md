# Contributing to EnvDoctor

Thanks for taking the time. This document is deliberately short and specific.

## Read first

1. [docs/PLAN.md](docs/PLAN.md) - the authoritative specification. It explains scope, architecture,
   milestones and what is explicitly out of scope.
2. [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - how the pieces fit together.
3. [docs/adr/](docs/adr/) - decisions that have already been argued about. Do not relitigate them
   without a new ADR.
4. [docs/phases/M4.md](docs/phases/M4.md) - report/bundle contracts, dependency order and acceptance gates;
   preserve [M3 evidence contracts](docs/phases/M3.md) and accepted ADRs 0005/0006.
5. [docs/DEVELOPMENT-TOOLS.md](docs/DEVELOPMENT-TOOLS.md) - CodeGraph freshness, static-analysis coverage
   and profiling appropriate to this repository's runtime.

## Ground rules

- Be decent. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
- Security issues never go in a public issue. See [SECURITY.md](SECURITY.md).
- One logical change per pull request. If the description needs "and also", split it.
- Do not invent benchmarks, screenshots or feature claims. If it is not measured, it is not stated.

## Development setup

```bash
git clone https://github.com/shellghostt/envdoctor.git
cd EnvDoctor
make bootstrap   # install tooling and dependencies
make test        # unit tests
make lint        # formatting and static analysis
```

Pin `GOTOOLCHAIN=go1.26.11` for the recorded gates. `go test -count=1 -timeout=120s ./...` also builds
the CLI and runs all 172 golden cases through it, including the unchanged 54 M2 cases.
Each new rule needs positive, negative and edge snapshots for Windows, Linux and macOS. Expected
findings are reviewed evidence, not output regenerated from the implementation. See
[M3 validation](docs/testing/m3-validation.md) for native Windows and Linux race commands and limits.
[M4 validation](docs/testing/m4-validation.md) records terminal, sharing, archive, offline and resource
cases. M4 local gates pass; native three-OS GitHub CI remains pending publication.
Windows race tests need a 64-bit MinGW-w64-compatible GCC configured in Go's CC/CXX; ordinary static
builds use `make build`, which disables CGO only for that target.

## Rule metadata and documentation

`internal/diagnostic/registry.go` owns each builtin rule's metadata and pure evaluator. Findings,
`explain`, `rules --json` and generated `docs/rules/ENV*.md` pages share that registry.
Update the registry, add reviewed fixtures, then run:

```bash
make docs
make check-docs
make test
make race
make lint
```

Do not edit generated rule pages directly. Tests and CI reject drift. Optional evidence additions must
preserve snapshot-v1 compatibility; project and registry contracts have their own committed schemas.
Unknown or failed evidence cannot become a speculative finding, and no metadata command runs collectors.

## Code navigation

[CodeGraph](https://github.com/colbymchenry/codegraph) builds a local symbol index of this
repository. It is optional: neither the build nor CI depends on it.

```bash
codegraph init --yes .   # build the index once
codegraph status          # whether the index is current
codegraph sync            # refresh it after a change
```

The index lives in `.codegraph/`. Git ignores its database and temporary files; the
`.codegraph/.gitignore` that enforces that is tracked.

## Commit messages

Conventional Commits, imperative mood, no trailing period:

```
feat(ingest): verify HMAC signature before persisting
fix(retry): honour Retry-After when it exceeds the base delay
docs(plan): record M2 acceptance criteria as met
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`.

## Pull request process

1. Open an issue first for anything larger than a typo. It saves everyone time.
2. Branch from `main`: `feat/short-slug`, `fix/short-slug`.
3. Add tests. A pull request that reduces coverage needs a reason in the description.
4. Run `make lint` and `make test` locally before pushing.
5. Fill in the pull request template completely, including how you verified the change.
6. CI must be green. Reviewers are looking for correctness first, style second.

## Definition of done

A change is done when:

- it is implemented, not stubbed;
- it is covered by a test that fails without it;
- `docs/PLAN.md` status fields and `README.md` roadmap reflect the new reality;
- any user-visible behaviour is documented where a user would look for it.

## Good first issues

Issues labelled `good first issue` are scoped to be completable in one sitting, with the relevant files
listed in the issue body. Claim one by commenting, then open a draft pull request early.
