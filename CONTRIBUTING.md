# Contributing to DoulaCloud

This file is an index. It holds no rule of its own: every standard a change is held to lives in one of the documents below, and this list only says where to look. A reviewer, human or agent, reads these before reviewing a change.

## The standards

- [`CLAUDE.md`](CLAUDE.md): the project's standing instructions, the cross-cutting expectations every feature carries (audit trail, accessibility, performance, security, layout, time), American English, and the git flow.
- [`docs/api-design.md`](docs/api-design.md): the Go BFF's HTTP endpoint standards.
- [`docs/testing.md`](docs/testing.md): the coverage gate, the test infrastructure, and migrations.
- [The Voice section of `docs/design/brief.md`](docs/design/brief.md#voice): the rule for every line a person reads on screen.
- [The Laws of UX section of `docs/design/brief.md`](docs/design/brief.md#the-laws-of-ux): what each law demands of a DoulaCloud screen, and where the laws conflict.
- [`docs/design/govuk-alignment.md`](docs/design/govuk-alignment.md): the GOV.UK pattern-by-pattern table for any screen that asks a person for something or reports a failure ([ADR-0021](docs/adr/0021-govuk-is-the-reference-for-service-patterns.md)).
- [ADR-0024](docs/adr/0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md): layout is intrinsic, and 320px is a conformance commitment.
- [ADR-0025](docs/adr/0025-layout-is-verified-across-the-continuum.md): layout is verified across the continuum.
- [`docs/agents/experience-review.md`](docs/agents/experience-review.md): the Experience reviewer's brief, which checks the rendered page against the Laws of UX and the GOV.UK table.

## Where the rest lives

The domain language is in [`GLOSSARY.md`](GLOSSARY.md), and the recorded decisions are in [`docs/adr/`](docs/adr/). How issues, triage and worktrees work is in [`docs/agents/`](docs/agents/).
