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
3. Start the app with `bun run dev` at the repo root, with `DOCKER_HOST` and `TESTCONTAINERS_RYUK_DISABLED=true` exported in the same shell (`docs/testing.md`). In a worktree every port shifts by `.port-offset` × 100 (`app/e2e/ports.ts`), so read the URL from the Vite log. Use a real Practice's data where a fixture exists.
4. Drive every listed screen with playwriter, at **320px** wide and at **desktop width** (1280px). Do the task the screen exists for, to its end. Read the rendered page, not the diff.

## What it reports

For each screen:

1. **Which law the screen spends and which GOV.UK pattern it follows.** Name the law from the brief's tables and the row from the GOV.UK table.
2. **Every departure without a recorded reason** from the Laws table or the GOV.UK table: what the page does, which row it breaks, and at which width.
3. **Whether the task ends in a confirmation** that says what happened and what is next (Peak-End). A task that ends on a blank page, or with no word of what happened, is a finding.
4. **The audit checks** ([#2001](https://github.com/markgoho/doula-cloud/issues/2001), 2026-10-26). For each screen, whichever of these holds is a finding:
   - a raw `<select>`, `<fieldset>`, `<label>`, `<h1>`-`<h6>`, `<ul>` of fields or a `<dl>` where the library has `Select`, `CheckboxGroup`, `LabeledField`, `Heading`, `StackedForm` or `SummaryList` (the lint rule catches most; the review catches the rest);
   - more than one primary button on the screen (one per section on a record page), a destructive button beside a benign one, or a destructive action repeated per row;
   - a list or section with nothing in it that is not `EmptyState` with its one action, or a totals block drawn over nothing;
   - a key/value list that is not `SummaryList`, or whose rows do not share a column, or a status written as a bare sentence where a row or a `Badge` would do;
   - a short answer in a full-width box (the width rule, [#1216](https://github.com/markgoho/doula-cloud/issues/1216));
   - an edit form rendered open on a record page where no one asked for it, or a form inside a table cell;
   - a task that ends without a status `Notice` (the confirmation check above), a bad link that lands on a box with no way on, or an entry screen with no line of context under its heading;
   - a heading at a size that is not the atom's, or a gap that is not one of the named spacing steps.

A departure that the ticket, an ADR or the GOV.UK table records a reason for is not a finding ([ADR-0021](../adr/0021-govuk-is-the-reference-for-service-patterns.md)). Name it and the place the reason is recorded, and move on.

The report is **under 400 words**, the same limit as the other two axes. A finding names the screen, the width, and the law or the GOV.UK row. Skip what tooling already enforces: the copy specs, axe, the keyboard walk, and the continuum sweep of [ADR-0025](../adr/0025-layout-is-verified-across-the-continuum.md). It adds no rule: every obligation it checks is already in the brief or the GOV.UK table.
