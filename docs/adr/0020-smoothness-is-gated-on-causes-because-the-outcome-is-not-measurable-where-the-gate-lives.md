# ADR-0020: Smoothness is gated on causes, because the outcome is not measurable where the gate lives

Status: accepted, 2026-08-28. Decided on [#418](https://github.com/markgoho/doula-cloud/issues/418).

## Context

[The design brief](../design/brief.md) makes smoothness the first place Doula Cloud's distinctiveness is spent, and says it "gets checked, not hoped for". It lists six requirements: nothing shifts after it paints; every action acknowledges itself within 100ms; loading is skeletal, not spinning; focus is visible and predictable; keyboard is a first-class path; and a dense list stays at 60fps under a real Practice's data.

Two of those six name a number a browser can report — Cumulative Layout Shift and Interaction to Next Paint — so the obvious design is to measure the outcome and fail the build on a threshold. Before committing to it, #418 measured whether the numbers can actually be read where a gate would run.

They cannot. A probe rendering `DataTable` at 350 and 2000 rows in the Vitest browser project, four runs:

| Instrument | Result | Usable? |
|---|---|---|
| `requestAnimationFrame` deltas | p50 **8.30ms in every run**, at both 350 and 2000 rows; worst frame 12ms | No. Headless Chromium paces frames at a fixed ~120Hz regardless of load, so an fps assertion always passes. |
| `longtask` entries | **Zero**, even at 2000 rows with a 65ms mount | No. |
| Element count | 1780 at 350 rows, 10030 at 2000 — exactly 5.086 per row, zero variance | Yes, exactly. |
| Mount cost | 13.9 / 16.9 / 15.5ms at 350; 65.5ms at 2000 | Partly. Real signal, ±20% run to run. |
| `layout-shift` in `supportedEntryTypes` | present | Yes. |

The second constraint is where a gate can live. CI's `app` job runs Postgres, the Firebase Auth emulator, the Go BFF and Chromium on one `ubuntu-latest` runner, and `playwright.config.ts` sets `retries: 2` with a comment explaining that timing stalls under that contention must not fail the build. A millisecond budget measured there would test the runner. Worse, a real latency regression would retry into green.

So the honest split is not "machine-checkable versus human-reviewed". It is **space versus time**. A fact about space — did something move, how many elements is this, is focus visible — reads the same on a contended runner as on an idle laptop. A fact about time does not.

## Decision

**Smoothness is enforced on its causes, in the unit suite, blocking. The outcome is measured only where the measurement is honest.**

Concretely:

- **Motion** is a parse gate. [`motion.spec.ts`](../../app/src/lib/styles/motion.spec.ts) reads every `.svelte` and `.css` file under `app/src` and asserts that no `transition` or `animation` carries a raw duration or easing keyword, that anything animating a transform sits inside a `prefers-reduced-motion: no-preference` block, that every `@keyframes` is justified, that no motion token is declared without a consumer, and that every `<img>` has an intrinsic width and height. It follows `tokens.spec.ts`: parse the source, not a rendered DOM, because a rendered check only sees what some test happened to mount.
- **Dense lists** are bounded rather than timed. [`DataTable.usage.spec.ts`](../../app/src/lib/components/organisms/DataTable.usage.spec.ts) asserts no route hands `DataTable` an unbounded list, and [`DataTable.performance.svelte.spec.ts`](../../app/src/lib/components/organisms/DataTable.performance.svelte.spec.ts) caps a row at six elements and asserts mount cost stays linear in the row count. A ratio, not a millisecond budget: it catches per-row work that reads the rest of the list, which is what actually makes a list stutter, and it means the same thing on any machine.
- **Layout stability** is the one outcome measured, at the component seam: [`Skeleton.layoutShift.svelte.spec.ts`](../../app/src/lib/components/atoms/Skeleton.layoutShift.svelte.spec.ts) asserts a skeleton reserves the space the content it stands in for will occupy.
- **Focus visibility and keyboard reachability** are handed to accessibility outright ([#447](https://github.com/markgoho/doula-cloud/issues/447)) rather than checked twice under two names. Focus *return* — where focus lands after a dialog closes — is a sequence no snapshot scanner can see, so it becomes a required behaviour of the Dialog component's own spec when a Dialog is first built.
- **The gate runs in the pre-commit hook** (`scripts/hooks/pre-commit`) as well as CI. The unit suite with the coverage gate is 5.5s, against 2.4s for `check` and 4.8s for `lint`. The Playwright e2e suite stays out: it builds the app and starts Postgres, the BFF and an emulator.

## What is deliberately not checked

Named here so the gap is a decision and not an oversight.

- **Frame rate.** No automated check asserts 60fps anywhere. The instrument lies in headless. Scroll feel is confirmed by a person, on a real display, when a ticket touches a list.
- **The 100ms acknowledgement floor and the 400ms Doherty completion budget.** Both are dominated by network and BFF latency, which is not what the brief's smoothness requirement is about, and neither can be read honestly on a shared runner.
- **Route-level Cumulative Layout Shift.** A route needs the SvelteKit runtime that browser-mode tests do not have, and route pop-in is fixed by giving each route a loading state rather than by observing that it lacks one.
- **The blank first frame.** The app is a client-rendered SPA — `adapter-static` with a `200.html` fallback and `ssr = false` — so every route paints blank before JS boots. No skeleton removes that. The intended answer is a service-worker precache rather than server rendering, and the groundwork is already paid for: `app/src/service-worker.ts` ships and is auto-registered, scoped to push only. It is fog on [#405](https://github.com/markgoho/doula-cloud/issues/405), not this ADR.

## Consequences

A rule that fails on trunk today is the only evidence a rule works. All five motion rules did, and fixing them is what this decision cost: `Checkbox` hardcoded `150ms ease` instead of the brief's tokens; `Button`'s loading spinner ran an ungated infinite rotation; `MessageThread` had an unsized image; and all four motion tokens were declared and unused. `DataTable` also turned out not to implement the brief's own Density rule — its rows were ~22px against a specified 40px — which only surfaced because a skeleton cannot reserve honest space for a table that does not obey the brief.

The cost is that a component can satisfy every rule here and still feel wrong. Nothing in this ADR would catch that. It is accepted because the alternative on offer was not a better check but a check that cannot fail.

The escape hatch is `motion:ignore` in a comment attached to the declaration, carrying the reason — the same shape as `coverage:ignore` in `api/`. The app's single keyframe animation, `Button`'s spinner, uses it: an indeterminate rotation is neither a state change, an entrance, nor a navigation, so none of the three motion tokens describes it.

## Alternatives considered

- **Measure CLS and INP in the Playwright e2e suite and fail on a threshold.** Rejected on the probe: the runner is contended and `retries: 2` would retry a real regression into green. Kept in a reduced form — layout stability at the component seam, where it is a fact about space.
- **A committed smoothness report, with CI failing when its commit SHA falls behind `HEAD`.** Rejected: it fires on every unrelated commit and becomes noise people learn to bypass.
- **A nightly job on a dedicated runner.** Rejected for now. It would measure GitHub's scheduling as much as our design, and a non-blocking trend line is not what the brief asked for.
- **Deleting the unused motion tokens.** Rejected: the brief specifies all three durations. They are recorded as awaiting a named consumer instead, and the spec fails if one is adopted without being taken off that list.
