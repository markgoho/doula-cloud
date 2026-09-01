# The #551 reach trial: what a session built with the carrier in place, and whether it ever met it

A record, not a suite member. The spec beside it under
`app/src/routes/practices/[practiceId]/invoices/zz-reach-measure.svelte.spec.ts` exists only on
this branch. It reports by failing deliberately and would redden CI.

Run it with `bunx vitest --run --project client <path>` from `app/`.

It is [#521](https://github.com/markgoho/doula-cloud/issues/521)'s harness with the route and the
fixtures changed and nothing else: the same 1px sweep from 1400px to 280px, the same
`getBoundingClientRect`/`scrollWidth` overflow test, the same report-by-failing-assertion, the
same planted-700px sanity case. One case is added that #521 could not have: the sweep is run
both before and after `document.fonts.ready`, because [#550](https://github.com/markgoho/doula-cloud/issues/550)
found the continuum check measures in the fallback face.

## The run

A fresh session, in its own worktree, forbidden from spawning subagents, was given
[#265](https://github.com/markgoho/doula-cloud/issues/265) with #521's prompt verbatim and only
the issue number changed. #265 replaced #551's own choice of
[#427](https://github.com/markgoho/doula-cloud/issues/427), whose acceptance criteria are an API
and would have put no UI in front of the session at all; the swap and its reasons are recorded on
#551. #265 is clean of layout prompting under #521's own grep.

It shipped three commits: a Go endpoint with its migration, `app/src/lib/invoice.ts`'s client half,
and a new route at `app/src/routes/practices/[practiceId]/invoices/+page.svelte`, 118 lines.

## Results, 2026-08-31, against the trial branch

### The sweep: nothing overflows, at any width, on any content

| content | result |
| --- | --- |
| modest fixture content | no overflow anywhere from 1400px to 280px |
| realistic longer content (double-barrelled surnames, five-figure amounts) | no overflow |
| longest realistic client name | no overflow |
| realistic content, after `document.fonts.ready` | no overflow |
| **a URL as a Client's name — #537's hostile vocabulary, in the only free-text column** | **no overflow** |

Against #521's baseline, which was 13px past the edge at 400px, 53px at 360px, **93px at 320px**
and 133px at 280px on one URL in a note.

The screen is not safe by luck and it is not safe because the fixture was polite: the hostile case
was run and holds. It is safe because [#542](https://github.com/markgoho/doula-cloud/issues/542)
gave `DataTable`'s cells `max-inline-size: var(--measure)` and `overflow-wrap: anywhere`, and
because `DescriptionList` on this screen holds only money and counts, so
[#530](https://github.com/markgoho/doula-cloud/issues/530) has no free text to break on.

### The wide end: `DataTable` takes a second configuration, the `dl` still takes one

| space given | `dl` width | `dl` columns | `table` |
| --- | --- | --- | --- |
| 1600px | 1600px | `122.3px 1455.3px` | 702px |
| 1280px | 1280px | `118.0px 1141.2px` | 672px |
| 900px | 900px | `113.0px 768.1px` | 636px |
| 600px | 600px | `109.1px 473.5px` | `display: none` — record view |
| 400px | 400px | `106.5px 277.1px` | `display: none` — record view |
| 320px | 320px | `105.5px 198.5px` | `display: none` — record view |

`DataTable` is a quantum layout: it switches to the record view below its 46rem floor and
shrink-to-fits above it, 636px to 702px rather than filling the room, which is #542 landed and
working. The `DescriptionList` is #521's finding unchanged — one configuration across the whole
continuum, its value track swelling to 1455px at 1600px. The label track is no longer frozen
(105.5px to 122.3px), which is [#540](https://github.com/markgoho/doula-cloud/issues/540)'s fluid
scale showing up in a measurement taken for another reason.

### The sanity case

`probeDetected: 2` with a planted 700px element; `dl` computes `display: grid`; `body` resolves
`"Hanken Grotesk"`. The no-overflow results are the absence of a break, not the absence of CSS or
of the real font.

## The transcript

Zero unquoted layout reasoning, the same as #521. `intrinsic` appears twice, both quoted from
`CLAUDE.md`'s motion rules; `ADR-0024` four times, all quoted from `DataTable.svelte`'s own
comments; `320` never appears as a width at all, only inside message UUIDs. `container query`,
`available space`, `breakpoint` and `responsive`: zero.

The one exception is not in the transcript but in what the session shipped — its own comment at the
top of the route:

> This page composes existing components only and writes no CSS of its own: the totals are a
> `DescriptionList`, the book is a `DataTable`, and both already adapt to the space they are given.
> That is also what keeps it inside CLAUDE.md's no-new-components block.

## Whether it met the exercise

It did not. `layout-exercise` appears twice in the transcript, both times inside one directory
listing of `app/src/routes/style-guide/`. `continuum` appears zero times, and so do the failure
message's own words — `worked answer`, `inside the 320px it was given`.

The session ran `bun run test:unit:coverage`, so the continuum check did execute. It passed:
51 passed, 2 expected fail. There was no failure message to read.
