# Experience review: the third axis of `/code-review`

The Standards and Spec axes read the diff. A law of UX cannot be checked on a diff: Fitts's Law, Peak-End, Selective Attention and the 320px commitment of [ADR-0024](../adr/0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md) are facts about a rendered page. This axis is the reviewer that the standing instruction under [The Laws of UX](../design/brief.md#the-laws-of-ux) names.

## When it runs

Only when the diff touches one of these paths:

- `app/src/routes`
- `site/src/routes`
- `app/src/lib/components`

Otherwise `/code-review` skips it with a one-line note.

## What the reviewer does

1. Read [The Laws of UX](../design/brief.md#the-laws-of-ux) and [the Voice section](../design/brief.md#voice) of the brief, and [`docs/design/govuk-alignment.md`](../design/govuk-alignment.md).
2. From the diff, list every screen the change reaches: each changed route, and each route that renders a changed component.
3. Start the app (`bun run dev` at the repo root, per `docs/testing.md`; `site/` runs on its own `vite dev`). Use a real Practice's data where a fixture exists.
4. Drive every listed screen with playwriter, at **320px** wide and at **desktop width** (1280px). Do the task the screen exists for, to its end. Read the rendered page, not the diff.

## What it reports

For each screen:

1. **Which law the screen spends and which GOV.UK pattern it follows.** Name the law from the brief's tables and the row from the GOV.UK table.
2. **Every departure without a recorded reason** from the Laws table or the GOV.UK table: what the page does, which row it breaks, and at which width.
3. **Whether the task ends in a confirmation** that says what happened and what is next (Peak-End). A task that ends on a blank page, or with no word of what happened, is a finding.

A departure that the ticket, an ADR or the GOV.UK table records a reason for is not a finding ([ADR-0021](../adr/0021-govuk-is-the-reference-for-service-patterns.md)). Name it and the place the reason is recorded, and move on.

The report is **under 400 words**, the same limit as the other two axes. A finding names the screen, the width, and the law or the GOV.UK row. Skip what tooling already enforces: the copy specs, axe, the keyboard walk, and the continuum sweep of [ADR-0025](../adr/0025-layout-is-verified-across-the-continuum.md).

## What it does not do

It adds no rule: every obligation it checks is already in the brief or the GOV.UK table. It does not replace looking at the page while building.
