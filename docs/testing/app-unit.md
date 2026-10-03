# Testing: the `app/` unit suite

## Smoothness: gated on causes, not on frame rate

[ADR-0020](../adr/0020-smoothness-is-gated-on-causes-because-the-outcome-is-not-measurable-where-the-gate-lives.md) records why, with the measurements behind it. The short version: headless Chromium reports a fixed ~8.3ms frame no matter what it renders, and CI's `app` job runs Postgres, the Auth emulator, the Go BFF and Chromium on one shared runner with `retries: 2`. Neither frame rate nor interaction latency can be read honestly in either place. Facts about *space* can, so those are what is asserted.

Four specs carry it, all in the unit suite, all blocking:

- `app/src/lib/styles/motion.spec.ts` — parses every `.svelte` and `.css` file under `app/src` for raw durations and easing keywords, ungated transforms, unjustified `@keyframes`, motion tokens with no consumer, and `<img>` without intrinsic dimensions. Break a rule deliberately by putting `motion:ignore` plus the reason in a comment attached to the declaration or to the rule that encloses it — the same shape as `coverage:ignore` in `api/`.
- `app/src/lib/components/organisms/DataTable.usage.spec.ts` — no route may hand `DataTable` an unbounded list. Four routes are on a justified waiting list until [#446](https://github.com/markgoho/doula-cloud/issues/446) gives their endpoints a cursor; the spec fails if one is left on that list after it starts paginating.
- `app/src/lib/components/organisms/DataTable.performance.svelte.spec.ts` — a row costs at most six elements, and reads the row array a fixed, small number of times rather than reading the rest of the list per row. A read count, never a millisecond budget — [#489](https://github.com/markgoho/doula-cloud/issues/489) replaced the last wall-clock assertion in this file after it flaked on CI.
- `app/src/lib/components/atoms/Skeleton.layoutShift.svelte.spec.ts` — a skeleton reserves the space the content it stands in for will occupy.

A whole-tree scan like `motion.spec.ts`'s runs once, at module scope, never inside an `it` — Vitest's 5-second `testTimeout` is a per-test budget, and a scan of the whole of `app/src` charged against it passes on a quiet machine and times out under the full suite's contention for disk and CPU, then passes again on a bare rerun of the identical commit ([#1211](https://github.com/markgoho/doula-cloud/issues/1211)). `layout.usage.spec.ts` already reads this way and is the pattern to copy.

**What is not checked** is listed in ADR-0020 rather than left to be discovered: frame rate, the 100ms and 400ms latency budgets, route-level Cumulative Layout Shift, and the blank first frame an SPA paints before its JavaScript boots. Scroll feel is a human check on a real display when a ticket touches a list. Focus visibility and keyboard reachability belong to accessibility — [`e2e.md`](e2e.md)'s "Accessibility" section is what that sentence points at — not to this gate, so nothing is asserted twice under two names.

## Layout: the continuum check, and what a fixture owes it

Layout is verified by sweeping a subject across the space it can be given rather than by asserting at chosen widths — `app/src/routes/style-guide/continuum.svelte.spec.ts` over every component demo, `app/src/routes/route-continuum.svelte.spec.ts` over every route, and the drag surface at `/style-guide/drag-surface` as the same fixtures seen by a person. `docs/adr/0025-layout-is-verified-across-the-continuum.md` is the decision and its Fixtures section is where a fixture's rules live: hostile values rather than polite ones, a row set realizing every state a field renders differently, every session a route renders differently under, and every state a component demo renders, declared as `variants` on its style-guide page and never kept behind a button (`app/src/routes/style-guide/demoStates.spec.ts` fails the build on a page with a switch of its own).

`.claude/rules/svelte-tests.md` is the operational form of those rules and the file to read before writing a fixture or a route spec — where a route's content is declared, what a spec still owns, the two-row shape, and the `variants` list a role-varying route declares its other sessions in ([#913](https://github.com/markgoho/doula-cloud/issues/913)).
