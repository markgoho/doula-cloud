# Every Layout CSS primitives — fact-finding for wayfinder #95

Research for GitHub issue #95 ("CSS architecture"). Question: what does
every-layout.dev (Andy Bell / Heydon Pickering) publicly document about its
layout primitives, what is actually free to read vs. paywalled, and what
CSS mechanisms/custom properties are documented — so Doula Cloud can decide
whether/how to build a small page-level layout utility layer inspired by it.

All content below was fetched directly from every-layout.dev in August 2026.
No implementation code is proposed here — this is source material only.

## The primitives (full current list)

Every Layout's index page (`/layouts/`) currently lists **13** primitives.
Each entry below gives the one-line description shown on the index page
itself, plus the fuller problem statement where the individual page was
free to read.

| # | Primitive | Index-page description | URL |
|---|---|---|---|
| 1 | Stack | "read for free" tag; spacing between flow elements | [/layouts/stack/](https://every-layout.dev/layouts/stack/) |
| 2 | Box | "A simple rectangle shape" | [/layouts/box/](https://every-layout.dev/layouts/box/) |
| 3 | Center | "A rectangle shape centered in the horizontal space" | [/layouts/center/](https://every-layout.dev/layouts/center/) |
| 4 | Cluster | "boxes of different widths, laid out like words in a paragraph" | [/layouts/cluster/](https://every-layout.dev/layouts/cluster/) |
| 5 | Sidebar | "A narrow and wide element laid out adjacently, transforming into two elements on top of each other" | [/layouts/sidebar/](https://every-layout.dev/layouts/sidebar/) |
| 6 | Switcher | "horizontally aligned boxes transforming into vertically stacked boxes" | [/layouts/switcher/](https://every-layout.dev/layouts/switcher/) |
| 7 | Cover | "A box with one large box in its vertical centre and two shorter boxes at its head and foot" | [/layouts/cover/](https://every-layout.dev/layouts/cover/) |
| 8 | Grid | "A grid of equal sized boxes (three columns and three rows)" | [/layouts/grid/](https://every-layout.dev/layouts/grid/) |
| 9 | Frame | "A box with decorative corners" | [/layouts/frame/](https://every-layout.dev/layouts/frame/) |
| 10 | Reel | "A box with a horizontal scrollbar containing a line of box-like elements" | [/layouts/reel/](https://every-layout.dev/layouts/reel/) |
| 11 | Imposter | "One box superimposed over a grid of other boxes" | [/layouts/imposter/](https://every-layout.dev/layouts/imposter/) |
| 12 | Icon | "A cross icon" | [/layouts/icon/](https://every-layout.dev/layouts/icon/) |
| 13 | Container | (no description shown on index page) | [/layouts/container/](https://every-layout.dev/layouts/container/) |

Source: [every-layout.dev/layouts/](https://every-layout.dev/layouts/), fetched
August 2026.

Note: the task background mentions "Grid, Reel, Imposter, Frame, Icon" as if
provisional — confirmed, all are real, current primitives on the site. There
is no separate "Row" primitive; the closest ideas (row-of-items,
horizontal-word-wrap) are covered by **Cluster** and **Reel**.

**Access split at a glance**: all 13 primitives' content has now been read —
3 freely on the public site (Stack, Sidebar, Switcher), and the other 10
(Box, Center, Cluster, Cover, Grid, Frame, Reel, Imposter, Icon, Container)
via the user's own paid Every Layout license, fetched through an
authenticated browser session on 2026-08-16. Full detail in the
"Licensing/reuse terms" section below.

## Per-primitive detail

### Stack — FREE, full code visible

**Problem it solves**: flow (block-direction) elements need spacing to
separate them, but margins applied directly and symmetrically to elements
create context-insensitive problems — most notably margins doubling up
against a parent's padding, or orphaned margins at the start/end of a
container.

**Core mechanism**: the owl/lobotomized-owl selector (universal adjacent-
sibling combinator) applying a logical margin only *between* siblings, never
at the start or end of the stack:

> `.stack > * + * { margin-block-start: 1.5rem; }`

The page also documents a *recursive* variant (`.stack * + *`, applying the
rule at every nesting depth) and a "split after" behavior for pushing a
later child to the far end of the stack using `margin-block-start: auto`
inside a flex context.

**Custom properties**: `--space` controls the gap, used as
`margin-block-start: var(--space, 1.5rem)`; the page's own worked examples
default it to Every Layout's internal modular-scale token `var(--s1)`.

Source: [/layouts/stack/](https://every-layout.dev/layouts/stack/), fetched
August 2026.

### Sidebar — FREE, full code visible

**Problem it solves**: placing two adjacent elements (a narrow "sidebar"
and a wide "content" area) responsively *without media queries* — so the
layout responds to the actual space available to the component (its
container), not the viewport, and wraps to stacked when there isn't enough
room, without an awkward in-between state.

**Core mechanism**: flexbox with an intentionally large `flex-grow`
disparity so the non-sidebar element "wins" available space until a
`min-inline-size` threshold forces a wrap:

> ```
> .with-sidebar { display: flex; flex-wrap: wrap; gap: 1rem; }
> .sidebar { flex-basis: 20rem; flex-grow: 1; }
> .not-sidebar { flex-basis: 0; flex-grow: 999; min-inline-size: 50%; }
> ```

**Custom properties**: the documented component API exposes `space`
(gap between the two children, default `var(--s1)`) and `contentMin`
(the minimum width of the non-sidebar element before wrapping, default
`50%`).

**Modern-CSS note on this page**: the article speculates about a
better future mechanism: *"Only with a capability like the mooted
container queries might we teach our component layouts to be fully context
aware."* Container queries are no longer "mooted" — they shipped in all
evergreen browsers years after this text was written — but the page's own
wording has not been updated to reflect that; it still frames container
queries as a hoped-for future.

Source: [/layouts/sidebar/](https://every-layout.dev/layouts/sidebar/),
fetched August 2026.

### Switcher — FREE, full code visible

**Problem it solves**: switching a set of same-priority elements directly
between a horizontal row and a vertical stack at a container-width
threshold, without an intermediate state where rows end up with uneven
item counts (the "orphan" problem multi-column wrapping normally produces).

**Core mechanism**: flex-basis abuse — a calculation that yields either a
huge positive value (forcing full-width, i.e. one item per row) or a
negative value (invalid, so ignored, letting flexbox lay items out
side-by-side):

> `.switcher > * { flex-grow: 1; flex-basis: calc((var(--threshold) - 100%) * 999); }`

**Custom properties**: `--threshold` (the container-width breakpoint,
defaults to the internal token `var(--measure)`), `--space` (gap between
items, defaults to `var(--s1)`), and `--limit` (maximum number of items
allowed to lay out horizontally before forcing a stack regardless of width,
default `4`).

**Modern-CSS notes on this page**: the `gap` property is used directly for
spacing and the page explicitly says it is "now supported in all major
browsers" (i.e. the page itself has been updated post-`gap`-adoption,
unlike the Sidebar page's container-query wording). It also documents a
"quantity query" technique using `:nth-last-child(n+5)` to react to item
*count* rather than container size.

Source: [/layouts/switcher/](https://every-layout.dev/layouts/switcher/),
fetched August 2026.

### Box — unlocked via paid license

**Problem it solves**: separating concerns between a Box and the layout
primitives that place it. A Box should own only the styles intrinsic to a
single element — padding, and its visible shape (border/background) —
while margin, width, and height stay inferred from context (parent
layouts, content) rather than set on the Box itself.

**Core mechanism**: padding on all sides or none (never asymmetrical,
which the page argues is really margin's job), plus forced `color`
inheritance so a Box's light/dark theme can be swapped from one place:

> ```
> .box {
>   padding: var(--s1);
>   border: var(--border-thin) solid;
>   --color-light: #fff;
>   --color-dark: #000;
>   color: var(--color-dark);
>   background-color: var(--color-light);
> }
>
> .box * {
>   color: inherit;
> }
>
> .box.invert {
>   color: var(--color-light);
>   background-color: var(--color-dark);
> }
> ```

The page separately documents a transparent-outline technique for Windows
High Contrast Mode, where a background-color alone is insufficient to
convey the box shape:

> `outline: 0.125rem solid transparent; outline-offset: -0.125rem;`

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `padding` | string | `"var(--s1)"` | A CSS padding value |
| `borderWidth` | string | `"var(--border-thin)"` | A CSS border-width value |
| `invert` | boolean | `false` | Whether to apply an inverted theme. Only recommended for greyscale designs. |

**Modern-CSS notes**: the negative `outline-offset` trick to draw a border
only under `forced-colors`/high-contrast conditions, with zero effect on
layout otherwise, is the one notable modern-CSS technique on this page. No
`:has()`, `clamp()`, or container-query usage.

Source: [/layouts/box/](https://every-layout.dev/layouts/box/), fetched
via authenticated session, 2026-08-16.

### Center — unlocked via paid license

**Problem it solves**: horizontally centering a column of content and
capping its width to a readable measure, without the readability harm of
`text-align: center`, and without undoing vertical margins a parent Stack
may already have applied (ruling out the `margin: 0 auto` shorthand, which
sets `margin-top`/`margin-bottom` too).

**Core mechanism**: logical-property auto margins on the inline axis only,
plus a documented "intrinsic centering" variant using Flexbox to center
children by their own content width rather than stretching them:

> ```
> .center {
>   box-sizing: content-box;
>   margin-inline: auto;
>   max-inline-size: var(--measure);
> }
> ```
>
> ```
> .center {
>   box-sizing: content-box;
>   max-inline-size: 60ch;
>   margin-inline: auto;
>   display: flex;
>   flex-direction: column;
>   align-items: center;
> }
> ```

`box-sizing: content-box` is set deliberately, overriding a
border-box-by-default reset, so that any gutter padding grows outward from
the capped measure instead of shrinking the content area.

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `max` | string | `"var(--measure)"` | A CSS max-width value |
| `andText` | boolean | `false` | Center align the text too (`text-align: center`) |
| `gutters` | boolean | `0` | The minimum space on either side of the content |
| `intrinsic` | boolean | `false` | Center child elements based on their content width |

**Modern-CSS notes**: uses logical properties (`max-inline-size`,
`margin-inline`) throughout rather than `max-width`/`margin-left`/
`margin-right`. No `:has()`, `clamp()`, or container-query usage.

Source: [/layouts/center/](https://every-layout.dev/layouts/center/),
fetched via authenticated session, 2026-08-16.

### Cluster — unlocked via paid license

**Problem it solves**: laying out a group of elements with indeterminate,
differing widths (buttons, tags, nav items) so they wrap fluidly like
words in a paragraph, with even spacing on every side — including between
wrapped lines — which earlier techniques (`inline-block` plus
`font-size: 0`, or symmetrical margins on Flexbox children) got wrong,
either doubling spacing at container edges or dropping vertical spacing
on wrap.

**Core mechanism**: the page documents its pre-`gap` negative-margin
fallback in full, then supersedes it with `gap`:

> ```
> .cluster {
>   --space: 1rem;
> }
> .cluster > * {
>   display: flex;
>   flex-wrap: wrap;
>   margin: calc(var(--space) / 2 * -1);
> }
> .cluster > * > * {
>   margin: calc(var(--space) / 2);
> }
> ```
>
> ```
> .cluster {
>   display: flex;
>   flex-wrap: wrap;
>   gap: var(--space, 1rem);
> }
> ```

The generator's final CSS adds justification/alignment:

> ```
> .cluster {
>   display: flex;
>   flex-wrap: wrap;
>   gap: var(--space, 1rem);
>   justify-content: flex-start;
>   align-items: center;
> }
> ```

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `justify` | string | `"flex-start"` | A CSS justify-content value |
| `align` | string | `"flex-start"` | A CSS align-items value |
| `space` | string | `"var(--s1)"` | A CSS gap value. The minimum space between the clustered child elements. |

**Modern-CSS notes**: states flexbox `gap` support landed in "all major
browsers" by "mid-2021," and the page now recommends using `gap` without
`@supports` feature-detection outright, "accepting that layouts will
become flush in older browsers" — the negative-margin technique above is
kept only "if that's your preference instead." It also flags a
feature-detection trap: `@supports (gap: 1rem)` can report true because
`gap` is supported for Grid while still being unsupported for Flexbox in
the same browser, giving a false positive if used to gate Cluster's CSS.

Source: [/layouts/cluster/](https://every-layout.dev/layouts/cluster/),
fetched via authenticated session, 2026-08-16.

### Cover — unlocked via paid license

**Problem it solves**: vertically centering one "main" element within a
container while optionally accommodating a header above and/or a footer
below it — staying robust to overflow and dynamic content height (no
fixed heights or `transform` hacks), and requiring no CSS changes when a
header or footer is added or removed from the markup.

**Core mechanism**: `margin-block: auto` on the centered element pushes it
to the middle of the flex column; a cascade + `:not()` combination strips
the redundant top/bottom margin from whichever elements are actually
first/last:

> ```
> .cover {
>   display: flex;
>   flex-direction: column;
>   min-block-size: 100vh;
>   padding: 1rem;
> }
>
> .cover > * {
>   margin-block: 1rem;
> }
>
> .cover > :first-child:not(h1) {
>   margin-block-start: 0;
> }
>
> .cover > :last-child:not(h1) {
>   margin-block-end: 0;
> }
>
> .cover > h1 {
>   margin-block: auto;
> }
> ```

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `centered` | string | `"h1"` | A simple selector such an element or class selector, representing the centered (main) element in the cover |
| `space` | string | `"var(--s1)"` | The minimum space between and around all of the child elements |
| `minHeight` | string | `"100vh"` | The minimum height (block-size) for the Cover |
| `noPad` | boolean | `false` | Whether the spacing is also applied as padding to the container element |

**Modern-CSS notes**: uses logical properties throughout (`margin-block`,
`margin-block-start`/`-end`, `min-block-size`) instead of `margin-top`/
`margin-bottom`/`min-height`. No `:has()`, `clamp()`, or container-query
usage.

Source: [/layouts/cover/](https://every-layout.dev/layouts/cover/),
fetched via authenticated session, 2026-08-16.

### Grid — unlocked via paid license

**Problem it solves**: producing a responsive grid-like formation (columns
and rows that grow/shrink/wrap together) that reconfigures automatically
as available space changes, without prescribing a fixed column count or
resorting to `@media` breakpoints — and without the overflow risk a
hard-coded `minmax()` minimum creates in containers narrower than that
minimum.

**Core mechanism**: the `repeat(auto-fit, minmax(...))` responsive-grid
pattern (the page credits Jen Simmons' *Layout Land* series as its
source), with the `minmax()` floor wrapped in `min()` so it caps at 100%
of the container instead of overflowing:

> ```
> .grid {
>   display: grid;
>   grid-gap: 1rem;
> }
>
> @supports (width: min(250px, 100%)) {
>   .grid {
>     grid-template-columns: repeat(auto-fit, minmax(min(250px, 100%), 1fr));
>   }
> }
> ```

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `min` | string | `"250px"` | A CSS length value representing x in `minmax(min(x, 100%), 1fr)` |
| `space` | string | `"var(--s1)"` | The space between grid cells |

**Modern-CSS notes**: this is the richest modern-CSS page in the set. It
walks through, in order: (1) a plain Flexbox `flex-basis` grid
(imprecise column alignment on wrap); (2) bare
`repeat(auto-fit, minmax(250px, 1fr))`, flagged as unsafe past its
hard-coded minimum; (3) a JavaScript `ResizeObserver`-based enhancement,
described by the page itself as "the most efficient method yet for
creating container queries with JavaScript," toggling a class once
container width crosses a threshold; then (4) explicitly retiring all of
that in favor of the CSS `min()` function: "it is actually no longer
needed to solve this particular problem... we have the recently widely
adopted CSS min() function." The final documented solution uses
`@supports` feature detection on `min()`, not native `@container` syntax
— the ResizeObserver-based pseudo-container-query is presented as
superseded specifically by `min()`, not by `@container`.

Source: [/layouts/grid/](https://every-layout.dev/layouts/grid/), fetched
via authenticated session, 2026-08-16.

### Frame — unlocked via paid license

**Problem it solves**: giving an arbitrary element — not just `<img>`/
`<video>` — a fixed aspect ratio and cropping its content to fill that
ratio, without hard-coding width/height, so media of unpredictable
dimensions doesn't distort or overflow.

**Core mechanism**:

> ```
> .frame {
>   --n: 16;
>   --d: 9;
>   aspect-ratio: var(--n) / var(--d);
>   overflow: hidden;
>   display: flex;
>   justify-content: center;
>   align-items: center;
> }
>
> .frame > img,
> .frame > video {
>   inline-size: 100%;
>   block-size: 100%;
>   object-fit: cover;
> }
> ```

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `ratio` | string | `"16:9"` | The element's aspect ratio |

**Modern-CSS notes**: explicitly states the `aspect-ratio` property has
replaced an older `padding-bottom: 56.25%` "intrinsic ratio" hack (traced
to a 2009 technique): "Since support is now good for the aspect-ratio
property, we can go ahead and use that instead of this elaborate hack."
The page also shows an `@media (orientation: portrait)` example for
swapping the `--n`/`--d` ratio custom properties — the one place among the
unlocked primitives that still explicitly reaches for a viewport media
query rather than a container query.

Source: [/layouts/frame/](https://every-layout.dev/layouts/frame/),
fetched via authenticated session, 2026-08-16.

### Reel — unlocked via paid license

**Problem it solves**: an accessible, JavaScript-optional alternative to
carousel/slider widgets — a horizontally-scrolling single-file row of
items using native browser scrolling, including scrollbar affordance and
spacing, without a scripted carousel plugin.

**Core mechanism**:

> ```
> .reel {
>   display: flex;
>   block-size: auto;
>   overflow-x: auto;
>   overflow-y: hidden;
>   scrollbar-color: #fff #000;
> }
>
> .reel::-webkit-scrollbar {
>   block-size: 1rem;
> }
>
> .reel::-webkit-scrollbar-track {
>   background-color: #000;
> }
>
> .reel::-webkit-scrollbar-thumb {
>   background-color: #000;
>   background-image: linear-gradient(#000 0, #000 0.25rem, #fff 0.25rem, #fff 0.75rem, #000 0.75rem);
> }
>
> .reel > * {
>   flex: 0 0 auto;
> }
>
> .reel > img {
>   block-size: 100%;
>   flex-basis: auto;
>   width: auto;
> }
>
> .reel > * + * {
>   margin-inline-start: 1rem;
> }
>
> .reel.overflowing {
>   padding-block-end: 1rem;
> }
> ```

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `itemWidth` | string | `"auto"` | The width of each item (child element) in the Reel |
| `space` | string | `"var(--s0)"` | The space between Reel items (child elements) |
| `height` | string | `"auto"` | The height of the Reel itself |
| `noBar` | boolean | `false` | Whether to display the scrollbar |

**Modern-CSS notes**: this page deliberately does *not* use `gap`, even
though `.reel` is a flex container, and says so explicitly: "The main
advantage of gap is ensuring the margins don't appear in the wrong places
when elements wrap. Since the Reel's content is not designed to wrap, we
shall use the margin-based solution instead. It's longer and better
supported." Detecting horizontal overflow (to conditionally add trailing
padding) still relies on a `ResizeObserver` + `MutationObserver` pair in
JavaScript; the page notes the ideal native solution would be an
`:overflowed-content` pseudo-class that "currently exists as little more
than an idea" and has not shipped. No `:has()`, `clamp()`, or
container-query usage.

Source: [/layouts/reel/](https://every-layout.dev/layouts/reel/), fetched
via authenticated session, 2026-08-16.

### Imposter — unlocked via paid license

**Problem it solves**: a general-purpose way to superimpose one element
centrally over another (the viewport, the document, or a positioned
ancestor) — dialogs, popups, dropdowns — without hard-coding the imposed
element's width/height, since those are often unknown ahead of time.

**Core mechanism**: absolute/fixed positioning at the 50%/50% point,
recentered with a `transform` (rather than negative margins) so no
foreknowledge of the element's own dimensions is required:

> ```
> .imposter {
>   position: absolute;
>   inset-block-start: 50%;
>   inset-inline-start: 50%;
>   transform: translate(-50%, -50%);
> }
>
> .imposter.contain {
>   --margin: 0px;
>   overflow: auto;
>   max-inline-size: calc(100% - (var(--margin) * 2));
>   max-block-size: calc(100% - (var(--margin) * 2));
> }
> ```

The body text also shows the fixed/absolute toggle as a custom property:

> ```
> .imposter {
>   position: var(--positioning, absolute);
>   inset-block-start: 50%;
>   inset-inline-start: 50%;
>   transform: translate(-50%, -50%);
>   max-inline-size: calc(100% - 2rem);
>   max-block-size: calc(100% - 2rem);
> }
> ```

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `breakout` | boolean | `false` | Whether the element is allowed to break out of the container over which it is positioned |
| `margin` | string | `0` | The minimum space between the element and the inside edges of the positioning container over which it is placed (where breakout is not applied) |
| `fixed` | boolean | `false` | Whether to position the element relative to the viewport |

**Modern-CSS notes**: uses logical inset/sizing properties throughout
(`inset-block-start`, `inset-inline-start`, `max-inline-size`,
`max-block-size`). CSS Grid line-based placement is mentioned only as a
rejected alternative, for being non-general ("would only work where your
positioning element is set to display: grid ahead of time"). No `:has()`,
`clamp()`, or container-query usage.

Source: [/layouts/imposter/](https://every-layout.dev/layouts/imposter/),
fetched via authenticated session, 2026-08-16.

### Icon — unlocked via paid license

**Problem it solves**: reliably sizing, vertically aligning, and spacing
an inline SVG icon next to text — so icon height tracks font size, the
icon sits on the text baseline, and spacing to accompanying text stays
correct in both LTR and RTL — without bespoke per-instance CSS.

**Core mechanism**:

> ```
> .icon {
>   width: 0.75em;
>   width: 1cap;
>   height: 0.75em;
>   height: 1cap;
> }
>
> .with-icon {
>   display: inline-flex;
>   align-items: baseline;
> }
>
> .with-icon .icon {
>   margin-inline-end: 1rem;
> }
> ```

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `space` | string | `null` | The space between the text and the icon. If null, natural word spacing is preserved |
| `label` | string | `null` | Turns the element into an image in assistive technologies and adds an aria-label of the value |

**Modern-CSS notes**: documents the emerging `cap` unit (capital-letter
height) as the theoretically correct icon-sizing unit, layered as a
fallback pair with `em` because `cap` "is currently not supported very
well" — `height: 0.75em; height: 1cap;`, where the later declaration wins
only in browsers that support it. Uses `margin-inline-end` (logical) so
icon-to-text spacing flips correctly under `dir="rtl"`. No `:has()`,
`clamp()`, or container-query usage.

Source: [/layouts/icon/](https://every-layout.dev/layouts/icon/), fetched
via authenticated session, 2026-08-16.

### Container — unlocked via paid license

**Problem it solves**: this page isn't a layout primitive like the
others — it's Every Layout's own direct answer to "now we have container
queries, is Every Layout obsolete?" It documents `container-type`/
`container` + `@container` as a deliberate escape hatch for cases where an
intrinsically-sound, query-free layout genuinely can't be devised.

**Core mechanism**:

> ```
> .container {
>   container-name: myContainer;
>   container-type: inline-size;
> }
> ```

The body text also shows the unnamed form (`container-type: inline-size;`)
and the named shorthand (`container: myContainer / inline-size;`), queried
with `@container myContainer (width < 360px) { ... }`.

**Custom properties**:

| Name | Type | Default | Description |
|---|---|---|---|
| `name` | string | (none listed) | The name of the container, used as the CSS container-name value (optional) |

**Modern-CSS notes**: this entire page is about modern CSS, and is the
single most decision-relevant page in the unlocked set for a
container-queries-first architecture. It argues container queries do not
make Every Layout obsolete but frames them as a fallback, not a default:
"manual intervention... They are circuit breakers we wire into layouts we
know are going to error... I'd sooner not have them anywhere I know
they're not needed." It gives a worked argument for why Sidebar's own
intrinsic, query-free approach is *more* capable than `@container` for
that specific case — a container query only knows the container's own
size, not the states of the elements inside it, so replicating Sidebar's
self-adjusting breakpoint with `@container` would require manually
re-deriving breakpoints per sidebar width, shown via a `:has()`-based
multi-rule example (`@container (width < 640px) { .with-sidebar:has(.sidebar--large) > * {...} }`)
as the more complex alternative. It documents `container-type:
inline-size`, named containers via the `container` shorthand, resolution
to the nearest ancestor container when containers are nested, and
mentions container query units in passing.

Source: [/layouts/container/](https://every-layout.dev/layouts/container/),
fetched via authenticated session, 2026-08-16.

## Rudiments (foundational concepts)

Every Layout's "Rudiments" section (6 pages, distinct from the 13 layout
primitives above) sets out the conceptual foundations the primitives are
built on. All 6 pages were fetched via the user's authenticated session.

### Boxes

**Core idea**: everything renderable in CSS is fundamentally a box (per
Rachel Andrew), and layout is the arrangement of boxes. The page walks
through the box model (content/padding/border/margin), the `display`
property's effect on box behavior (`block` vs `inline` vs `inline-block`
vs `none`), and argues that box dimensions should be *derived* from
content and context rather than prescribed — hardcoding width/height
causes overflow and breakage, so authors should offer "suggestions"
(e.g. `min-height`, `flex-basis`) and let the browser calculate the rest.

**Concrete technique**: recommends `box-sizing: border-box` applied
universally via the wildcard selector:

> ```
> * {
>   box-sizing: border-box;
> }
> ```

It also demonstrates a common overflow bug: a child at `inline-size: 100%`
inside a padded `content-box` parent overflows by the padding amount,
while `inline-size: auto` (the default) does not, regardless of
`box-sizing`.

**Relevance to #95**: the page explicitly forward-references "Global and
local styling" as the place where the universal-selector technique (like
`* { box-sizing: border-box; }`) is justified architecturally — i.e.
Boxes treats "reach many elements with one low-specificity rule" as the
foundational efficiency argument for the whole global/local system
described below.

Source: every-layout.dev/rudiments/boxes/, fetched via authenticated
session, 2026-08-16.

### Composition

**Core idea**: argues for "composition over inheritance" in CSS
architecture, borrowing the term from programming and explicitly citing
React's docs on the same principle. Its worked example is a `.dialog`
component styled as a BEM-style namespaced block
(`.dialog`, `.dialog__header`, `.dialog__body`, `.dialog__foot`): the
page states this namespacing is "where most CSS bloat comes from,"
because styles that could be shared get re-declared under each new
component's namespace instead. Layout primitives exist to be the shared,
meaningless-alone building blocks (like a JS `boolean` primitive) that
compose into meaningful UI (a dialog, a form, a slide) without
per-component namespacing.

**Concrete technique/naming convention**: no new CSS mechanism here, but
it explicitly names the pattern it argues against:

> ```
> .dialog { /* ... */ }
> .dialog__header { /* ... */ }
> .dialog__body { /* ... */ }
> .dialog__foot { /* ... */ }
> ```

**Relevance to #95**: this is Every Layout's clearest stated position
against BEM-style, per-component-namespaced class blocks for layout
purposes — it is not a stance on component-local styling generally, but
specifically on using a monolithic namespaced class tree to reproduce
layout that shared primitives could handle instead. Doula Cloud's plan
(scoped Svelte `<style>` for component-specific concerns, a separate
small utility layer for page layout) does not use BEM either, so this
does not conflict with the plan; it is a data point in favor of *not*
reaching for BEM-style namespacing for the layout utility layer.

Source: every-layout.dev/rudiments/composition/, fetched via
authenticated session, 2026-08-16.

### Units

**Core idea**: argues against the `px` unit for sizing, on the grounds
that a CSS pixel is not a stable, meaningful atomic unit (sub-pixel
rendering, device pixel ratios, and user zoom all make "1px" fuzzy), and
that `px`-based font sizing overrides the user's browser/OS font-size
preference, an accessibility regression. It also argues against
width-based `@media` breakpoints as "arbitrary" hard-coded
reconfiguration points insensitive to actual available space —
consistent with the primitives' avoidance of breakpoints already noted
elsewhere in this doc (Sidebar, Switcher).

**Concrete technique/opinionated stance**:
- Prefer `rem` for block-level/document-relative sizing (e.g. `h2 { font-size: 2.5rem; }`), `em` for inline-context-relative sizing (the page's own analogy: "the em unit is to the rem unit what a container query is to a @media query").
- Prefer `ch`/`ex` specifically for measure/line-length constraints, since `1ch` scales with font-size, unlike `px`.
- Explicit rejection of viewport-only breakpoints, with a fluid-scaling example instead:

> ```
> :root {
>   font-size: calc(1rem + 0.5vw);
> }
> ```

**General-foundations note**: non-obvious opinionated stance — the
page frames `em`/`rem` choice by whether the element is block-level
(`rem`) or inline (`em`), rather than a blanket rem-everywhere rule, and
singles out `ch` as "the only appropriate unit" for measure specifically
because measure is defined in characters-per-line.

Source: every-layout.dev/rudiments/units/, fetched via authenticated
session, 2026-08-16.

### Global and local styling

**Core idea**: proposes three tiers of "global" CSS reach, ordered from
lowest to highest specificity/narrowest to broadest reach (attributed to
Harry Roberts' ITCSS — "Inverted Triangle CSS," where "specificity...
is inversely proportional to reach"): (1) universal/inherited styles
(`:root`, `*`, plain element selectors), (2) layout primitives
(reusable, composable, configured via props), and (3) utility classes
(single-purpose, `!important`-boosted, for final overrides). Separately,
it surveys the standard mechanisms for *local*/instance-specific
styling — `id` selectors, inline `style` attributes, and Shadow DOM —
and calls out a drawback for each (high/uncontrollable specificity for
`id`, unmaintainability for inline styles, and for Shadow DOM: it blocks
styles from leaking both out *and* in, "meaning you can no longer
leverage global styling").

**Concrete technique/naming convention**: utility classes use a
property-name:value naming convention (not BEM), echoing CSS
declaration syntax, with the colon escaped for validity:

> ```
> .font-size\:base {
>   font-size: 1rem;
> }
> .font-size\:biggish {
>   font-size: 1.75rem;
> }
> .font-size\:big {
>   font-size: 2.25rem;
> }
> ```

with each declaration `!important`-suffixed: "Utility classes are for
final adjustments, and should not be overridden by anything that comes
before them." Custom properties are shared between elements and
utilities by placing them on `:root`:

> ```
> :root {
>   --font-size-base: 1rem;
>   --font-size-biggish: 1.75rem;
>   --font-size-big: 2.25rem;
> }
> ```

The page also shows how a layout primitive (Stack) is packaged as a
custom element with a per-instance generated `<style>` block keyed to
the resolved prop value (e.g. `<style id="Stack-var(--s3)">`), so
identically-configured instances share one stylesheet:

> ```
> stack-l {
>   display: block;
> }
> stack-l > * + * {
>   margin-top: var(--s1);
> }
> ```

**Relevance to #95 (most decision-relevant page)**: Every Layout's
"global" tier (universal styles + primitives + utilities) is directly
analogous in spirit to Doula Cloud's planned small utility layer for
page-level layout — same idea of low-specificity, reusable, composable,
class-based rules with a plain naming scheme. That part validates the
plan. It *complicates* the plan on the "local" side, though: Every
Layout's own taxonomy of local/instance styling is `id` selectors,
inline styles, and Shadow DOM — it does not mention or account for
component-scoped stylesheets (CSS Modules-, Vue-, or Svelte-style
build-time scoping) as a category at all. The page's closest analog to a
Svelte scoped `<style>` block is Shadow DOM, which it criticizes
specifically because Shadow DOM blocks styles from getting *in* as well
as out ("you can no longer leverage global styling"). Svelte's scoping
does not have that particular drawback — global custom properties still
cascade into scoped component styles normally — so Doula Cloud's planned
split (scoped component styles + a shared custom-property token layer +
a separate utility layer) can be read as addressing the exact gap Every
Layout leaves open, rather than conflicting with its model. On naming:
Every Layout does **not** use BEM anywhere in this page (or in
Composition, which argues against BEM-style namespacing) — its own
precedent for a global utility layer is the property:value convention
above, worth considering (or explicitly rejecting in favor of something
else) if Doula Cloud's utility layer needs a naming convention.

**Relevance to #97**: the "Primitives and props" subsection is a direct
precedent for exposing custom-property configuration through a thin
JS/prop layer: a custom element defines a prop getter that falls back to
a default custom property —

> ```
> get space() {
>   return this.getAttribute('space') || 'var(--s1)';
> }
> ```

— and the resolved value is interpolated into a generated, per-configuration
`<style>` block scoped by a `data-i` attribute selector, rather than set
as an inline custom property per instance. Worth flagging for #97's
props-vs-custom-property-override discussion; not a resolution.

Source: every-layout.dev/rudiments/global-local-styling/, fetched via
authenticated session, 2026-08-16.

### Modular scale

**Core idea**: argues for deriving all spacing/type-size values from a
single ratio, multiplying/dividing a base value repeatedly (analogized
to a harmonic series in music) rather than choosing dimensions ad hoc.
This produces a sequence of custom properties, signed by step from a
`0` base, that both spacing and type size are meant to draw from.

**Concrete technique/naming convention**: signed, index-named custom
properties driven by a single `--ratio`:

> ```
> :root {
>   --ratio: 1.5;
>   --s-1: calc(var(--s0) / var(--ratio));
>   --s0: 1rem;
>   --s1: calc(var(--s0) * var(--ratio));
>   --s2: calc(var(--s1) * var(--ratio));
> }
> ```

The page notes these root-level custom properties are readable from
JavaScript (`getComputedStyle(document.documentElement).getPropertyValue('--s3')`)
and cross Shadow DOM boundaries, and shows a props-based interpolation
path (`<my-element padding="var(--s3)">`) with an optional regex check to
restrict a prop to a bare scale-step integer rather than an arbitrary
value.

**Relevance to #94 (context only, not an open decision)**: this is one
concrete implementation pattern for a modular type/spacing scale — a
single ratio driving signed, step-indexed custom properties consumable
by both CSS and JS. Doula Cloud's type scale is already decided in #94;
noted here only as background, not as something to reconsider.

**Relevance to #97**: the regex-validated, scale-step-only prop variant
is a second concrete precedent (alongside Global and local styling's
plain custom-property fallback) for constraining a component prop to
specific token values rather than accepting an arbitrary CSS value.

Source: every-layout.dev/rudiments/modular-scale/, fetched via
authenticated session, 2026-08-16.

### Axioms

**Core idea**: recommends stating a small number of global, unqualified
design rules ("axioms" — e.g. "the measure should never exceed 60ch")
and enforcing each one as pervasively as possible via the
lowest-specificity mechanism available, rather than applying it
manually per element via utility classes. It favors an exception-based
(deny-list) selector over an allow-list of specific elements, since a
deny-list only requires remembering what should be *excluded* from a
rule, not every element the rule should apply to.

**Concrete technique**: a universal rule with named exceptions, backed
by a `:root` custom property:

> ```
> :root {
>   --measure: 60ch;
> }
> * {
>   max-inline-size: var(--measure);
> }
> html, body, div, header, nav, main, footer {
>   max-inline-size: none;
> }
> ```

The same `--measure` custom property is then reused as a primitive
prop default — the Switcher primitive's `threshold` prop falls back to
`var(--measure)` when no value is supplied, and silently ignores
invalid values (falling back to the primitive's own default stylesheet
rather than erroring):

> ```
> get threshold() {
>   return this.getAttribute('threshold') || 'var(--measure)';
> }
> ```

**Relevance to #95**: reinforces the three-tier universal/primitive/
utility architecture from "Global and local styling" with a worked
example, and is another instance of the deny-list-plus-`*`-selector
pattern rather than any BEM- or component-namespaced approach.

**Relevance to #97**: the `threshold` example is a concrete pattern for
"prop with a shared-token default and silent fallback on invalid
input," directly relevant to how a component's custom-property-based
configuration API could degrade gracefully — flagged for #97, not
resolved here.

Source: every-layout.dev/rudiments/axioms/, fetched via authenticated
session, 2026-08-16.

## Blog: "Eschewing Shadow DOM" (Heydon Pickering, 14 June 2019)

Directly answers whether Every Layout's own shipped custom elements
(`<cluster-l>` etc., seen on the Cluster primitive page) use Shadow DOM —
they do not, and the post explains why, with specifics:

**Problems encountered with Shadow DOM, quoted/paraphrased from the post**:
- Universal (`*`) selectors pierce inconsistently — `color` crosses the
  shadow boundary, `box-sizing` does not, forcing per-component
  `box-sizing: border-box` repetition.
- `all: inherit` as a workaround for selective inheritance forces *every*
  property to inherit, including layout-affecting ones like `display`,
  which is unusable.
- `::slotted()` (styling Light DOM content from inside a Shadow root) only
  reaches direct children, not deeper descendants, and does not support
  sibling combinators (`::slotted(*) + *` does not work) — ruling out
  exactly the adjacent-sibling techniques Stack and other primitives rely
  on.
- Instance-specific prop-derived styles injected into the Shadow root lose
  a specificity fight against the component's own default/fallback
  document-stylesheet rule, forcing `!important` to win — "feels
  counter-intuitive and hacky" in the post's own words.

**The alternative pattern the post documents (no Shadow DOM)**: a light-DOM
custom element that, on `connectedCallback`/`attributeChangedCallback`,
computes an identifier from its prop value, stamps it as a `data-*`
attribute on itself, and injects a scoped `<style id="...">` block into
`document.head` (only if an identical one doesn't already exist) targeting
`[data-i="<id>"]`. A `get`/`set` accessor pair reflects the HTML attribute
to a JS property with a sane default (e.g. `measure` defaults to `65ch`).
Critically, the *document stylesheet also carries a plain-CSS default* for
the component's un-configured state (e.g. `center-l { margin-inline: auto;
max-inline-size: 65ch; }`), so — in the post's words — "these kinds of
layout-specific components do not require JavaScript to run on the client
at all — at least not for initial styling." JS only runs to override the
default when an instance sets a non-default prop value. The post also notes
this light-DOM approach is SSR-friendly (tested with JSDOM + Eleventy),
whereas Shadow DOM content could not be prerendered by the tooling
available at the time.

**Relevance to #95 (this ticket) and the custom-elements decision**: this
is the specific, sourced mechanism behind "build custom elements informed
by Every Layout but maintained ourselves" — light DOM (no Shadow Root),
default styling expressed as plain CSS with zero required JS, and
attribute-reflection JS reserved only for non-default instance
configuration. It also independently confirms the "Global and local
styling" page's implicit stance (Shadow DOM as the only "local" styling
option Every Layout's own taxonomy considers, and its cascade-blocking
drawback) with a concrete first-party account of why the author moved away
from it in practice.

Source: every-layout.dev/blog/eschewing-shadow-dom/, fetched via
authenticated session, 2026-08-16.

## Licensing/reuse terms

The homepage frames the offering as a $69 one-time purchase, with three
layouts and an introductory section given away free as a preview:

> "Buy Every Layout For $69" ... "Read the free rudiments and axioms"

What the $69 purchase includes, quoted directly from the homepage:

> "Access to all the site content, including all the layouts and layout
> generators. All available offline too!"
> "Every Layout as a book, in the EPUB format, for reading with your
> preferred reader."
> "The full set of layout components, implemented as interoperable custom
> elements."
> "Free updates for life. We are always improving, and have lots more
> content planned"

The site also offers bulk discounts for multi-seat company purchases via
direct contact (per the homepage).

Source: [every-layout.dev/](https://every-layout.dev/), fetched August 2026.

**What's free right now, precisely:**
- The introductory "rudiments and axioms" material (e.g.
  [/rudiments/boxes/](https://every-layout.dev/rudiments/boxes/), confirmed
  fully accessible, no paywall, with its own small CSS examples — the box
  model, `display`, logical properties, and formatting contexts — as
  conceptual background, not a layout primitive itself).
- Three full primitive pages with complete CSS and custom-property
  documentation: **Stack**, **Sidebar**, **Switcher**.

**What requires purchase:** the other 10 primitive pages (Box, Center,
Cluster, Cover, Grid, Frame, Reel, Imposter, Icon, Container), the EPUB
book, and the packaged custom-element implementations.

**Redistribution/reuse rights**, from the Terms and Conditions:

> "When you purchase a licence for Every Layout, you own a licence to the
> content that is authored and owned by Heydon Pickering and Andy Bell."
>
> "Re-publishing and re-selling of Every Layout is strictly forbidden and
> discovered instances will be pursued, legally, in accordance with United
> Kingdom copyright law."

The terms also reserve the right to revoke a license (without refund, after
a warning) for "unfair usage or irresponsible sharing" such as sharing
login credentials.

Source: [every-layout.dev/terms-and-conditions](https://every-layout.dev/terms-and-conditions),
fetched August 2026.

**Practical read for this decision**: the three free pages (Stack,
Sidebar, Switcher) publish real, complete CSS that is safe to read and
reimplement in our own words/tokens — nothing there is paywalled or marked
proprietary-only-with-purchase. The terms forbid re-publishing/re-selling
Every Layout's own content (e.g. don't copy the book's prose or mirror the
paid pages), but they do not purport to restrict independently
reimplementing a documented CSS *technique* once understood — CSS
techniques (adjacent-sibling margin, flex-basis wrapping tricks) are not
copyrightable expression in the way prose or a packaged product is. For the
10 paywalled primitives, we simply do not have the source material
publicly to know their exact documented approach; anything we'd write for
Box/Center/Cluster/Cover/Grid/Frame/Reel/Imposter/Icon/Container would be
our own independent design, not "sourced from Every Layout," because
Every Layout's own text for those isn't public.

## Custom-property configurability

Every Layout's own naming and defaults, as documented across all 13
primitive pages (3 free, 10 via the paid license), plus how each maps
conceptually onto our `--space-1`…`--space-12` base-4 scale (`tokens.css`)
as a default-value source. This is a mapping observation only — no CSS is
written here.

| Primitive | Every Layout's property name | Every Layout's own default | What it controls | Possible token mapping |
|---|---|---|---|---|
| Stack | `--space` | `var(--s1)` (Every Layout's own modular-scale token, roughly their "1 step" spacing unit) | gap between stacked siblings | Would default to one of our `--space-*` steps, e.g. `--space-4` |
| Sidebar | `space` (component prop) | `var(--s1)` | gap between sidebar and content | Same idea — a `--space-*` step |
| Sidebar | `contentMin` (component prop) | `50%` | minimum width of the non-sidebar column before wrap | Not a spacing token — a proportion, not on our `--space-*` scale |
| Switcher | `--threshold` | `var(--measure)` (Every Layout's line-length token, not a spacing token) | container width at which items switch from row to stack | Not a `--space-*` value — a width/measure value |
| Switcher | `--space` | `var(--s1)` | gap between switched items | A `--space-*` step |
| Switcher | `--limit` | `4` | max item count before forcing a stacked layout regardless of width | A count, not a token |
| Box | `padding` | `"var(--s1)"` | padding on all sides of the Box | A `--space-*` step |
| Box | `borderWidth` | `"var(--border-thin)"` | border width | Not a `--space-*` value — a border-width scale, not spacing |
| Box | `invert` | `false` | toggles inverted light/dark theme | Not spacing — a boolean |
| Center | `max` | `"var(--measure)"` | max-width of the centered column | Not a `--space-*` value — a measure/width, not spacing |
| Center | `andText` | `false` | whether to also `text-align: center` | Not spacing — a boolean |
| Center | `gutters` | `0` | minimum space on either side of the content | A `--space-*` step, if treated as a length rather than the documented boolean type |
| Center | `intrinsic` | `false` | centers children by content width instead of stretching | Not spacing — a boolean |
| Cluster | `justify` | `"flex-start"` | `justify-content` value | Not spacing — an alignment keyword |
| Cluster | `align` | `"flex-start"` | `align-items` value | Not spacing — an alignment keyword |
| Cluster | `space` | `"var(--s1)"` | gap between clustered children | A `--space-*` step |
| Cover | `centered` | `"h1"` | selector for the centered (main) element | Not spacing — a selector |
| Cover | `space` | `"var(--s1)"` | minimum space between/around all child elements | A `--space-*` step |
| Cover | `minHeight` | `"100vh"` | minimum block-size of the Cover | Not a `--space-*` value — a height, not spacing |
| Cover | `noPad` | `false` | whether `space` is also applied as container padding | Not spacing itself — a boolean toggle |
| Grid | `min` | `"250px"` | minimum column width in `minmax(min(x, 100%), 1fr)` | Not a `--space-*` value — a width, not spacing |
| Grid | `space` | `"var(--s1)"` | gap between grid cells | A `--space-*` step |
| Frame | `ratio` | `"16:9"` | aspect ratio | Not a `--space-*` value at all — a ratio |
| Reel | `itemWidth` | `"auto"` | width of each Reel item | Not a `--space-*` value — a width |
| Reel | `space` | `"var(--s0)"` | gap between Reel items | A `--space-*` step, likely a smaller one than Cluster/Cover's `--s1`-based default since `--s0` is Every Layout's smaller scale point |
| Reel | `height` | `"auto"` | height of the Reel | Not a `--space-*` value — a height |
| Reel | `noBar` | `false` | whether to show the scrollbar | Not spacing — a boolean |
| Imposter | `breakout` | `false` | whether the element may exceed its positioning container | Not spacing — a boolean |
| Imposter | `margin` | `0` | minimum space between the element and the positioning container's inside edges | A `--space-*` step |
| Imposter | `fixed` | `false` | positions relative to the viewport instead of an ancestor | Not spacing — a boolean |
| Icon | `space` | `null` | space between icon and text (word spacing preserved if null) | Loosely spacing-shaped, but Every Layout's own default is `null` (em-relative word spacing), so mapping to a fixed `--space-*` rem value would be a deliberate override, not a match to their default |
| Icon | `label` | `null` | accessible label, turns the icon into an ARIA image | Not spacing — a string |
| Container | `name` | (none listed) | `container-name` value | Not spacing at all — an identifier string |

Note: Every Layout's own default values (`--s1`, `--measure`) come from
its **own** modular-scale/measure token system documented in the free
"rudiments" section, not from any spacing scale identical to ours. Mapping
our `--space-*` scale onto their `--space`/`space` properties is a
reasonable adaptation (both are "one step of vertical/inline rhythm"), but
`--threshold`/`contentMin` are container-width and proportion values, not
spacing, and don't map onto `--space-*` at all. The same reasoning applies
to the newly-added rows above: `Box`, `Center`, `Cover`, `Grid`, `Reel`,
and `Imposter` all default their spacing-shaped properties to `--s1` or
`--s0`, Every Layout's own modular-scale tokens, not to anything numerically
tied to our `--space-*` scale.

## Modern-CSS notes

**What the free pages already reflect (sourced facts):**
- Switcher's page uses the `gap` property directly for spacing and states
  it is "now supported in all major browsers"
  ([/layouts/switcher/](https://every-layout.dev/layouts/switcher/)) — i.e.
  this page has been updated past the pre-`gap` negative-margin-hack era
  that older Every Layout material (and the original book) used.
- Sidebar's page also uses `gap: 1rem` directly in its `.with-sidebar`
  flex container example
  ([/layouts/sidebar/](https://every-layout.dev/layouts/sidebar/)).
- Sidebar's page, by contrast, still frames container queries as a
  hoped-for, not-yet-real capability ("the mooted container queries") —
  this part of the page's text has not been refreshed even though
  container queries have since shipped in evergreen browsers
  ([/layouts/sidebar/](https://every-layout.dev/layouts/sidebar/)).
- Stack and Sidebar both use logical properties (`margin-block-start`,
  `min-inline-size`) rather than physical `margin-top`/`min-width`.

No `:has()`, `clamp()`, CSS nesting, or `@layer` usage was found on any of
the three free pages.

**Assessment** (our own view, not sourced from Every Layout, given this
project's evergreen-only browser target):
- Sidebar's stated motivation — needing "a capability like the mooted
  container queries" to make the layout truly context-aware rather than
  viewport-aware — is exactly the gap container queries close today. Since
  Firefox is explicitly unsupported and Chrome/Edge/Safari all ship
  container queries, a from-scratch Sidebar/Switcher-equivalent could react
  to the *component's own* available width via `container-type: inline-size`
  instead of (or alongside) the flex-basis arithmetic tricks Every Layout
  documents, which were designed specifically to work around the absence of
  container queries.
- Switcher's `calc((var(--threshold) - 100%) * 999)` flex-basis trick is a
  workaround for not having a real container-width conditional. A
  container-query-based `@container` rule would express the same intent
  (switch layout at a container width) more directly and readably, without
  relying on integer-overflow-adjacent arithmetic.
- CSS nesting (native, no preprocessor) could clean up the presentation of
  these primitives' selectors (e.g. nesting a sibling-combinator rule
  inside a parent block) but doesn't change the underlying mechanism —
  it's a readability/authoring convenience, not a new capability, for
  these particular primitives.
- Stack's core mechanism (adjacent-sibling margin) has no simpler modern
  replacement — `gap` doesn't apply here because Stack targets arbitrary
  flow/block children (not necessarily a flex or grid container), so the
  selector-based technique remains the correct approach even in an
  evergreen-only target.

## Open questions / gaps

- **Whether the paid custom elements differ from the documented raw CSS**:
  the free pages describe hand-written CSS classes/custom properties; the
  paid tier additionally ships "interoperable custom elements." We could
  not verify whether the custom-element implementations use the same CSS
  internally or a different approach, since that content is paywalled.
- **No FAQ page was found** at a guessed `/faq` path (not linked in main
  navigation); licensing terms were instead confirmed via the homepage
  pricing copy and the separate Terms and Conditions page. If a dedicated
  FAQ exists at a different URL, it wasn't discovered in this pass.
- **Historical/older book content**: the task background notes Every
  Layout as a book "predates some current CSS features." We could not
  directly compare the current site's paywalled-primitive text against the
  original book's text (both are behind the paywall / not independently
  fetchable), so we can't confirm whether the paywalled pages have been
  updated for `gap`/logical properties the way Stack, Sidebar, and
  Switcher visibly have.

## Note on sourcing

Every factual claim above was pulled directly from every-layout.dev pages
(fetched August 2026), cited inline at the point each claim is made. No
secondary/aggregator sources were used. Where the public site does not
expose information (the 10 paywalled primitive pages), this is stated
explicitly rather than inferred or guessed at as fact.
