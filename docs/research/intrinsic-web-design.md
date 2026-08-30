# Intrinsic Web Design: definition, breakpoints, and the CSS that replaces queries

Research date: 2026-08-30. Sources are primary where available (Jen Simmons'
own talks/slides, liveblog transcripts of her talks, MDN, CSS WG-adjacent
docs, Rachel Andrew, Every Layout). Third-party summaries are marked as such.

## 1. Simmons' definition, and the contrast with Responsive Web Design

Jen Simmons coined "Intrinsic Web Design" (IWD) for her An Event Apart Seattle
2018 talk "Everything You Know About Web Design Just Changed," and expanded it
in the follow-up talk "Designing Intrinsic Layouts." She was, at the time, a
Mozilla Designer/Developer Advocate and CSS Working Group member (later at
Apple).

Her own framing, from the transcript of her interview on Jeffrey Zeldman's
*The Big Web Show* (episode 176):

> "It's not that float-based thing where everything's set in widths with using
> percents. It's this new set of technologies... It's not just because the
> tech is new, it's also because the possibilities of what you can actually do
> are new, and the ways in which you can get content to morph and shift and
> change based on how much space is available is actually really different
> than Responsive Web Design."
> — Jen Simmons, [transcript, zeldman.com](https://zeldman.com/2018/05/02/transcript-intrinsic-web-design-with-jen-simmons-the-big-web-show/)

She is explicit that IWD is scoped to layout, not to responsive design as a
whole discipline:

> "Nothing in the rest, or about what mobile is, or how content should be
> structured in a content management system. All of that is definitely the
> same. I'm just talking about layout."
> — same source

Jeremy Keith's liveblog of the original AEA talk (a close, near-verbatim
transcription made live at the event, standard practice in the CSS community
for capturing conference talks that are not otherwise published as text)
records Simmons contrasting Responsive Web Design's three named ingredients —
a fluid grid, flexible/fluid images, and media queries to reset the design at
breakpoints (Ethan Marcotte's original 2010 definition) — against what she
called Intrinsic Web Design's expanded list. Per the liveblog, Simmons lists
these as the new era's characteristics:

1. **Fluid *and* fixed** — layouts can deliberately mix flexible and fixed
   elements (Responsive Web Design's dogma was "everything fluid"; IWD says a
   logo column can stay a fixed width while others flex).
2. **Multiple "stages of squishiness"** — a single track or item can move
   through distinct behaviors as space changes: fixed, then `fr` (fully
   fluid), then `minmax()` (fluid until it hits a floor/ceiling), then `auto`
   (sized to content) — several transitions happening in one continuous
   resize, not one breakpoint jump.
3. **Truly two-dimensional layout** — rows and columns together, with
   intentional whitespace as a first-class layout element (via CSS Grid),
   rather than one-dimensional float/flexbox stacking.
4. **Nested layout contexts** — Flow, Flexbox, Grid, and Multicolumn can all
   be nested and mixed inside one another.
5. **Content that expands and contracts in more than one way** — wrap and
   reflow, enlarge or shrink, add or remove real whitespace, slide/overlap —
   not just "get narrower."
6. **"Media queries, as needed"** — named explicitly as the final item on her
   list: media queries are demoted from *the* mechanism (as in RWD) to an
   optional finishing tool.

Source: [Jeremy Keith's liveblog, "Everything You Know About Web Design Just
Changed by Jen Simmons"](https://adactio.com/journal/13671).

## 2. Her stated position on breakpoints and named device widths

Simmons' position is nuanced, not absolutist. She does not say "eliminate
media queries." She says the foundation of the layout should come from
content- and container-aware CSS (grid, flexbox, intrinsic sizing keywords),
and breakpoints/media queries become a secondary, occasional tool used to
tweak details once that foundation is in place — the opposite priority order
from Responsive Web Design, where the breakpoint *is* the layout mechanism.

Direct evidence:

- Her own sixth named characteristic of IWD is literally **"Media queries, as
  needed"** — an explicit item in her list, not an omission. (Source:
  [adactio.com/journal/13671](https://adactio.com/journal/13671), liveblog of
  her AEA talk.)
- From the Big Web Show transcript, discussing a Grid layout built with
  `minmax()`, `fr`, fixed widths, and `auto`: "This is all without any media
  queries. You don't have to use media queries to make these things happen,"
  describing four independent "stages of squishing or growing" that occur
  continuously as the viewport resizes — the point being that a lot of what
  used to require 3–4 breakpoints now happens automatically without a single
  `@media` rule. (Source: [zeldman.com transcript](https://zeldman.com/2018/05/02/transcript-intrinsic-web-design-with-jen-simmons-the-big-web-show/))
- In the liveblog of "Designing Intrinsic Layouts," Simmons ties breakpoints
  specifically to the failure mode of percentage-based (purely fluid) sizing:
  once columns get too narrow, content breaks — "This is what we need
  breakpoints for" in a percentage-only world. Her argument is that
  `minmax()` and intrinsic keywords (`min-content`, `max-content`, `auto`)
  remove that specific failure mode, because "the content will never get
  smaller than the minimum content size," so the breakpoint that existed only
  to rescue percentage layouts from breaking is no longer needed for that
  purpose. (Source: [adactio.com/journal/14889](https://adactio.com/journal/14889))

So: her stated position is **minimize breakpoints tied to arbitrary
device/viewport widths for layout purposes, by giving CSS enough intrinsic
information (content sizes, minmax bounds, fr distribution) that it can make
good layout decisions on its own** — not "never write a media query."

## 3. The concrete CSS mechanisms (no query required)

| Mechanism | What it does | Right tool when… |
|---|---|---|
| `flex-wrap: wrap` + `flex-basis` | Items lay out in a row and drop to a new line once they no longer fit at their basis width; `flex-basis` sets each item's "ideal" main-axis size before growing/shrinking is applied. | You have an unknown/variable number of same-ish-sized items (nav items, tags, cards in a single row) and want wrapping driven by available space, not a device width. [MDN: flex-wrap](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/flex-wrap), [MDN: flex-basis](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/flex-basis). Every Layout's "Sidebar" primitive uses an extreme-ratio variant of this (`flex-basis: 0; flex-grow: 999` on one side) to force a hard wrap once the non-sidebar column would drop under `min-inline-size: 50%` — entirely without a media query ([every-layout.dev/layouts/sidebar](https://every-layout.dev/layouts/sidebar/)). |
| `grid-template-columns: repeat(auto-fit/auto-fill, minmax(X, 1fr))` | Lets the grid itself decide how many columns fit, given a minimum column width `X`, without you naming a column count per breakpoint. | A card/tile grid where you want "as many columns of at least X as fit," and column count should just be a function of container width. [Rachel Andrew, "Flexible Sized Grids with auto-fill and minmax"](https://rachelandrew.co.uk/archives/2016/04/12/flexible-sized-grids-with-auto-fill-and-minmax/). See §4 for `auto-fit` vs `auto-fill`. |
| `min()` / `max()` / `clamp()` | Math functions usable anywhere a length is valid. `clamp(MIN, PREFERRED, MAX)` = `max(MIN, min(PREFERRED, MAX))` — a fluid preferred value (often a `vw`-based one) held between a floor and a ceiling. | Fluid type scales, fluid spacing, and "grow with the viewport but never below/above X" sizing — the single biggest reason a design can drop 2–3 breakpoints outright. [MDN: clamp()](https://developer.mozilla.org/en-US/docs/Web/CSS/clamp). Accessibility note from MDN: for fluid font sizes, keep the max at least 2× the min so 200% browser zoom (WCAG 1.4.4) still works. |
| Intrinsic sizing keywords: `min-content`, `max-content`, `fit-content` | Size a box to the smallest size that avoids overflow (`min-content`), to the size it would be with no wrapping (`max-content`), or to whichever is smaller of that and the available space (`fit-content`). | Grid/flex tracks or boxes that should be "exactly as wide as this specific content needs," e.g. a logo column, a label column, a "shrink to content" button. Simmons demonstrates `grid-template-columns: max-content auto max-content` for a header (logo / flexible middle / nav) with no query. ([adactio.com/journal/14889](https://adactio.com/journal/14889)) |
| `aspect-ratio` | Locks a box's width:height ratio; the browser derives the missing dimension. | Responsive media/embeds/cards that must keep proportions as their width changes, without a padding-hack or a query recalculating height per breakpoint. Baseline (widely available) since Sept 2021. [MDN: aspect-ratio](https://developer.mozilla.org/en-US/docs/Web/CSS/aspect-ratio). |
| `grid-auto-flow` (incl. `dense`) | Controls how auto-placed grid items fill the grid; `dense` backfills earlier holes left by larger items instead of only moving forward. | Item counts/sizes vary and you want the grid to pack tightly on its own rather than hand-authoring placement per breakpoint. [MDN: grid-auto-flow](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/grid-auto-flow). |
| Subgrid (`grid-template-columns/rows: subgrid`) | A nested grid item can adopt its parent's track sizing instead of defining independent tracks, so nested content lines up with the outer grid at any size. | Card layouts, nested components, or multi-column text blocks that must align across siblings regardless of each card's own content. Baseline (widely available) since Sept 2023. [MDN: Subgrid](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_grid_layout/Subgrid). |

Newer than the 2018 talks (all genuinely reduce query dependence further —
see §6 for detail): container queries/units, container style queries,
`calc-size()`, `interpolate-size`, and the draft `reading-flow` property.

## 4. `auto-fit` vs `auto-fill` — exact behavior and the failure mode

Both are keywords for the *repetition count* argument of `repeat()`, used
with `minmax()`, e.g. `repeat(auto-fill, minmax(100px, 1fr))`. Both compute
"how many tracks of at least 100px can fit," but they differ in what happens
to tracks that end up with **no item** in them:

- **`auto-fill`**: keeps those empty tracks as real (collapsed-but-present)
  tracks in the grid. Rachel Andrew: "If you use the auto-fill keyword empty
  tracks will remain as part of the grid." ([rachelandrew.co.uk](https://rachelandrew.co.uk/archives/2016/04/12/flexible-sized-grids-with-auto-fill-and-minmax/))
- **`auto-fit`**: does the same computation, but then **collapses** any
  track that ended up empty once items are placed, and the `1fr` on the
  remaining, non-empty tracks expands to consume that freed space. Rachel
  Andrew: "...this would behave in the same way as described above but once
  all grid items have been placed any completely empty tracks will be
  dropped." (same source)

**Failure mode of picking the wrong one**, per CSS-Tricks' worked example
("Auto-Sizing Columns in CSS Grid: auto-fill vs auto-fit"):

- Pick **`auto-fill`** when you actually wanted items to stretch to fill a
  wide, under-populated row (e.g. 3 cards in a container wide enough for 6):
  the browser still reserves 3 more empty, collapsed-width tracks, so your 3
  real cards stay pinned at their minmax minimum width and don't grow —
  visually, unexplained dead space to the right instead of bigger cards.
- Pick **`auto-fit`** when you actually wanted stable track slots (e.g. items
  arriving asynchronously, or a design that should hold its column grid even
  when temporarily short an item): because empty tracks collapse, as items
  come and go the remaining ones visibly stretch and shrink and the whole
  grid "jumps," instead of holding steady in fixed-width slots with a gap.

Source: [CSS-Tricks, "Auto-Sizing Columns in CSS Grid Layout: auto-fill vs
auto-fit"](https://css-tricks.com/auto-sizing-columns-css-grid-auto-fill-vs-auto-fit/),
[Rachel Andrew](https://rachelandrew.co.uk/archives/2016/04/12/flexible-sized-grids-with-auto-fill-and-minmax/).

## 5. Where a query is still genuinely necessary

The sources are honest that intrinsic/fluid sizing does not cover everything.
Concretely:

- **Container queries themselves have a structural limit: circularity.**
  You cannot query a container's size along an axis whose size is itself
  determined by its own descendants — the browser cannot lay out the
  descendants (needed to know the container's content-derived size) before
  it has evaluated the query that would style those same descendants. This
  is why `container-type: inline-size` forces layout, style, and
  *inline-size containment* on the container — the container's inline size
  must become independent of its content before it can be queried at all.
  Source: [MDN, "Using container size and style
  queries"](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_containment/Container_queries)
  and the [WICG cq-usecases wiki, "Circularity and Container
  Queries"](https://github.com/WICG/cq-usecases/wiki/Circularity-and-Container-Queries).
  Practically: you cannot say "make this box shrink-to-fit its content, AND
  also let its children respond to that box's resulting width" — one of the
  two has to give.
- **Every Layout flagged the exact same gap from the other direction, before
  container queries existed.** Their Sidebar primitive (pure flexbox, no
  query) explicitly could not tell the difference between "this component is
  in a 300px-wide slot" and "this component is in a 500px-wide slot" at the
  *same viewport width* — only the (then-hypothetical) "container queries"
  could give a component real awareness of its own containing box: "Only
  with a capability like the mooted container queries might we teach our
  component layouts to be fully *context aware*." Source:
  [every-layout.dev/layouts/sidebar](https://every-layout.dev/layouts/sidebar/).
  (This gap is now closed by container queries for the size-awareness case —
  see §6 — but it demonstrates that pure content-driven flex/grid sizing was
  never claimed to solve component-level context-awareness on its own.)
- **Queries that are about environment, not size, have no intrinsic-sizing
  substitute at all**, because they answer questions no amount of flexible
  sizing can answer: `prefers-color-scheme` (dark mode), `prefers-reduced-motion`,
  `prefers-contrast`, `forced-colors`, `hover`/`any-hover`/`pointer`/`any-pointer`
  (mouse vs. touch input), `print` media, and screen resolution/`min-resolution`.
  These remain `@media` features because they are not about how much space
  is available — no CSS WG source suggests intrinsic sizing was ever meant to
  replace them.
- **A structural change to the DOM/interaction pattern** (e.g., swapping a
  full nav bar for a hamburger menu, not just reflowing nav items) is a
  decision about which markup/behavior to show, not a sizing decision — this
  is the kind of thing Simmons scoped out of IWD entirely when she said "I'm
  just talking about layout" (§1). It is why "media queries, as needed"
  remained on her own list rather than being dropped.

## 6. What has landed since (as of 2026) that further reduces query need

- **Container queries (`@container`), size queries** — Baseline widely
  available; let a component respond to its *containing box's* size rather
  than the viewport, directly closing the "context-awareness" gap Every
  Layout flagged in 2019/2020 (§5). Setup: `container-type: inline-size` (or
  `size`) plus optional `container-name`; new length units `cqw`, `cqh`,
  `cqi`, `cqb`, `cqmin`, `cqmax`. Source: [MDN, Container
  queries](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_containment/Container_queries).
- **Container *style* queries (`@container style(...)`)** — let CSS branch
  on a custom-property value set on an ancestor container, no JavaScript
  needed. Per web.dev's Baseline coverage, style queries on custom
  properties reached cross-browser support through 2025–2026 (Chrome,
  Firefox, Safari all shipped); style queries on regular (non-custom)
  properties are not yet supported anywhere. Sources: [MDN, "Using container
  size and style
  queries"](https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Containment/Container_size_and_style_queries),
  [web.dev Baseline digest, Aug
  2025](https://web.dev/blog/baseline-digest-aug-2025).
- **Name-only container queries** — apply styles based only on "is this
  inside a named container," no size condition at all; useful for
  contextual theming without a size threshold.
- **`calc-size()`** — performs arithmetic on intrinsic-size keywords
  (`auto`, `min-content`, `max-content`, `fit-content`), e.g.
  `calc-size(auto, size + 100px)`, and is what makes it possible to animate
  *to or from* `auto`/`fit-content` at all (previously impossible because
  browsers can't normally interpolate a keyword). Experimental/limited
  support as of this writing. Source: [MDN,
  calc-size()](https://developer.mozilla.org/en-US/docs/Web/CSS/calc-size).
- **`interpolate-size`** — the simpler companion: set to `allow-keywords` to
  opt an element (or, via inheritance, a subtree) into transitioning between
  a length and an intrinsic-size keyword without needing `calc-size()`'s
  arithmetic. MDN: not yet Baseline as of March 2025, Chromium-only at that
  point. Source: [MDN,
  interpolate-size](https://developer.mozilla.org/en-US/docs/Web/CSS/interpolate-size).
- **`reading-flow`** (CSS Display Module Level 4, still a Working Draft) —
  lets the *reading/focus order* of a flex/grid/block container's children
  be set independently of their visual/painted order (which `order` or grid
  placement can scramble), so sequential (tab) navigation and screen-reader
  order stay correct even when a layout visually reorders items for a given
  size. Not itself a sizing/query mechanism, but relevant because it removes
  a common reason teams reached for a media query to re-order markup at a
  breakpoint for accessibility reasons. Source: [MDN,
  reading-flow](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/reading-flow),
  [Chrome for Developers, "Use CSS reading-flow for logical sequential focus
  navigation"](https://developer.chrome.com/blog/reading-flow).

## Key primary sources consulted

- Jen Simmons, "Everything You Know About Web Design Just Changed" — [talk
  notes/slides](https://talks.jensimmons.com/20LmLE); [Jeremy Keith's
  liveblog](https://adactio.com/journal/13671)
- Jen Simmons, "Designing Intrinsic Layouts" — [talk
  page](https://talks.jensimmons.com/15TjNW); [Jeremy Keith's
  liveblog](https://adactio.com/journal/14889)
- Jen Simmons interview, *The Big Web Show* ep. 176 — [transcript via
  zeldman.com](https://zeldman.com/2018/05/02/transcript-intrinsic-web-design-with-jen-simmons-the-big-web-show/)
- Rachel Andrew, ["Flexible Sized Grids with auto-fill and
  minmax"](https://rachelandrew.co.uk/archives/2016/04/12/flexible-sized-grids-with-auto-fill-and-minmax/)
- Andy Bell & Heydon Pickering, [Every Layout](https://every-layout.dev/) and
  the [Sidebar layout](https://every-layout.dev/layouts/sidebar/)
- MDN Web Docs: [clamp()](https://developer.mozilla.org/en-US/docs/Web/CSS/clamp),
  [aspect-ratio](https://developer.mozilla.org/en-US/docs/Web/CSS/aspect-ratio),
  [Subgrid](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_grid_layout/Subgrid),
  [Container queries](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_containment/Container_queries),
  [calc-size()](https://developer.mozilla.org/en-US/docs/Web/CSS/calc-size),
  [interpolate-size](https://developer.mozilla.org/en-US/docs/Web/CSS/interpolate-size),
  [reading-flow](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/reading-flow),
  [flex-wrap](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/flex-wrap),
  [flex-basis](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/flex-basis),
  [grid-auto-flow](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/grid-auto-flow)
- CSS-Tricks, ["Auto-Sizing Columns in CSS Grid Layout: auto-fill vs
  auto-fit"](https://css-tricks.com/auto-sizing-columns-css-grid-auto-fill-vs-auto-fit/)
- WICG, ["Circularity and Container
  Queries"](https://github.com/WICG/cq-usecases/wiki/Circularity-and-Container-Queries)
- web.dev, [Baseline digest, Aug 2025](https://web.dev/blog/baseline-digest-aug-2025)

## Notes on source reliability

Two "third-party" sources are used above where no better primary source
exists: Jeremy Keith's liveblogs (near-verbatim transcription made live at
the conference, standard practice for capturing CSS conference talks that
were never published as text — treated here as close to primary) and
web.dev's Baseline digest for exact ship dates of very recent (2025–2026)
container style query support, since browser-vendor release notes were not
individually cross-checked for every engine. If exact ship dates matter for
a decision, verify against caniuse.com or the MDN browser-compatibility
tables directly before relying on them.
