# ADR 0001 - Initial technology and scope decisions

- Status: accepted
- Date: 2026-11-13
- Deciders: Aditya

## Context

EnvDoctor is a greenfield open-source project. Before any code exists, the expensive mistakes to avoid are
scope creep and undocumented stack choices that a future contributor will try to reverse.

## Decisions

1. **Go plus Cobra, shipped as one static binary** - the tool often runs on a machine where the toolchain is already broken. A Node or Python CLI cannot diagnose a broken runtime in its own ecosystem.
2. **Collectors touch the OS, rules never do** - a rule is a pure function over a snapshot. That is the only way to test three platforms from one CI runner.
3. **No administrative privileges required** - diagnosis must work for a normal user account. Escalation, if ever needed, is a separate explicit action.
4. **Fixes are opt-in, previewed, and never silent** - the tool reports what it would change and requires --yes. No rule may edit PATH, registry, or shell profiles as a side effect.
5. **Never shell out to a tool that might not exist** - binary presence is checked first; OS APIs are preferred where they are stable. A missing tool is a finding, not a crash.
6. **MIT license** - maximum adoption matters more than patent protection for a CLI utility.

## Consequences

- The project ships one narrow capability that actually works rather than a broad surface of stubs.
- Contributors have a reference point: changing any decision above requires a superseding ADR, not a
  silent pull request.
- Reversing a decision later is allowed but must be written down with the reason and the migration cost.

## Follow-up

- ADR 0002 is created at M1, recording the concrete schema and interface decisions made while building.
