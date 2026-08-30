# The #521 baseline harness, and what it measured

A record, not a suite member. The spec file beside it under
`app/src/routes/practices/[practiceId]/engagement-requests/[requestId]/zz-baseline-measure.svelte.spec.ts`
exists only on this branch; it was deleted from trunk once it had produced the numbers below,
because it reports by failing deliberately and would redden CI.

Run it with `bunx vitest --run --project client <path>` from `app/`.

## What it measures

It renders the real route in real Chromium with the real stylesheet, gives the page a width,
and asks every element whether it reaches past the frame's edge — then repeats, one pixel at a
time, from 1400px down to 280px. It names no width in its assertion. The width appears only in
the *output*, as the onset point of a break that was found by looking at all of them.

## Results, 2026-08-30, against commits `40858c8` and `53e3750`

| content | result |
| --- | --- |
| modest fixture content | no overflow anywhere from 1400px to 280px |
| realistic longer content (long surname, three-line note, two engagements, empty balance) | no overflow anywhere from 1400px to 280px |
| long client name alone | no overflow |
| long requester email alone | no overflow |
| **one URL in the note** | 13px past the edge at 400px, 53px at 360px, **93px at 320px**, 133px at 280px |

The browser breaks on `-` and on `@`, so neither a long hyphenated surname nor an email address
is hostile. A `/`-separated URL breaks nowhere, and `DescriptionList`'s `grid-template-columns:
auto 1fr` gives both tracks an `auto` minimum, so the URL sets a floor under the grid that is
passed up through `RecordDetail`'s `.body` grid to the page itself. Filed as #530.

## The wide end: one configuration, not several

| space given | label track | value track | textarea |
| --- | --- | --- | --- |
| 1600px | 84px | 1116px | 1216px |
| 1280px | 84px | 1116px | 1216px |
| 900px | 84px | 759px | 859px |
| 600px | 84px | 459px | 559px |
| 400px | 84px | 259px | 359px |
| 320px | 84px | 179px | 279px |

The label track is frozen at the max-content width of "Balance after" at every size, and the value
track simply swells to whatever is left — 1116px, a measure of roughly 150 characters, at the
`--page-max` cap. Every Layout's "quantum layout" is a component that exists in several
configurations until the browser resolves one from available space. This screen has exactly one,
stretched across the whole continuum.

## Three techniques worth keeping

Whoever builds the map's real continuum check will need all three; each took a try or two.

1. **Nothing sets up the app's CSS or its custom elements for a component test.** `vite.config.ts`
   declares no `setupFiles`; the root `+layout.svelte` imports `app.css` and calls
   `registerLayoutPrimitives()` at runtime, so a harness must do both itself — guarded on
   `customElements.get`, since the registry survives between tests in a file and a second
   `define` throws.
2. **Browser-mode `console.log` does not reach the terminal.** The report comes back as a
   deliberately failing `expect(report).toBe('REPORT')`, which prints it in the diff.
3. **A harness that finds nothing is worthless until it is shown able to find something.** The
   `harness sanity` case plants a 700px-min-width element and asserts the sweep catches it. It
   caught 39 elements — and it also confirmed `container-l` really does establish
   `container-type: inline-size` and the `dl` really is a grid, so the "no overflow" results are
   not the absence of CSS.

## The caveat

The route is mounted without the app shell around it, so what is measured is the space the
*page* is given rather than the space left over after a top bar. Every finding here is about the
page's own boxes, so the shell's absence changes no result's class; it would change absolute
numbers if the shell ever took horizontal room, which today it does not.
