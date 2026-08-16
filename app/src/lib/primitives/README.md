# Layout primitives

13 Every Layout-inspired layout primitives, built as native custom elements
(no Shadow DOM, no Svelte). See ADR-0003
(`docs/adr/0003-css-layout-primitives-as-native-custom-elements.md`) for the
architectural why. This doc is the consumer-facing when/how.

Each primitive works with zero JS at its default settings — the default
styling lives in `app/src/lib/styles/primitives.css`. Setting an attribute
to a non-default value triggers a small JS sync (`defineLayoutPrimitive.ts`)
that injects a scoped override style. Call `registerLayoutPrimitives()`
(from `app/src/lib/primitives/index.ts`) once, client-side, before using any
of these tags.

Attribute names are kebab-case and match the CSS values shown below
directly (e.g. `space="var(--space-6)"`, `min-height="50vh"`). Boolean
toggles (`invert`, `and-text`, `intrinsic`, `no-pad`, `no-bar`, `breakout`,
`fixed`) are plain presence attributes — `<box-l invert>`, not
`invert="true"` — and are CSS-only; they never reach the JS layer.

## The 13 primitives

### `<stack-l>` — vertical spacing between flow children

Adds block-direction margin *between* siblings only (never at the start or
end), so spacing never doubles against a parent's padding.

- `space` — margin between children. Default `var(--space-4)`.

```html
<stack-l space="var(--space-6)">
  <p>First</p>
  <p>Second</p>
</stack-l>
```

### `<box-l>` — a padded, bordered rectangle

Owns padding, border, and background/text color — nothing about its own
width, height, or margin, which stay inferred from context.

- `padding` — default `var(--space-4)`.
- `border-width` — default `var(--border-thin)`.
- `invert` (boolean) — swaps to an inverted background/text theme.

```html
<box-l padding="var(--space-6)"><p>Content</p></box-l>
<box-l invert><p>Inverted</p></box-l>
```

### `<center-l>` — horizontal centering with a max measure

Centers a single column of content on the inline axis and caps its width,
without the readability harm of `text-align: center`.

- `max` — max inline size. Default `var(--measure)`.
- `gutters` — minimum inline padding. Default `0`.
- `and-text` (boolean) — also centers text (`text-align: center`).
- `intrinsic` (boolean) — centers children by their own content width
  instead of stretching them.

```html
<center-l max="var(--measure-narrow)" gutters="var(--space-4)">
  <p>Readable column</p>
</center-l>
```

### `<cluster-l>` — a wrapping group of variable-width items

Lays out items of unpredictable width (tags, buttons, nav links) so they
wrap like words in a paragraph, with even spacing on every side including
between wrapped lines.

- `space` — gap between items. Default `var(--space-4)`.
- `justify` — `justify-content` value. Default `flex-start`.
- `align` — `align-items` value. Default `flex-start`.

```html
<cluster-l justify="center">
  <button>One</button>
  <button>Two</button>
  <button>Three</button>
</cluster-l>
```

### `<sidebar-l>` — two elements side by side, wrapping to stacked

Places exactly two children adjacently — a narrow one and a wide one —
wrapping to stacked when the wide one doesn't have enough room, with no
in-between state.

- `space` — gap between the two children. Default `var(--space-4)`.
- `content-min` — min inline size of the non-sidebar (last) child before it
  wraps. Default `50%`.

```html
<sidebar-l>
  <nav>Sidebar</nav>
  <main>Content</main>
</sidebar-l>
```

### `<switcher-l>` — same-priority items that flip row↔column as a whole

Switches a set of equal-priority children directly between a horizontal row
and a vertical stack at a container-width threshold — never a partial wrap
with uneven row counts.

- `threshold` — container-width breakpoint. Default `var(--measure)`.
- `space` — gap between items. Default `var(--space-4)`.
- `limit` — max items allowed in a row before forcing a stack regardless of
  width. Default `4`.

```html
<switcher-l limit="3">
  <article>A</article>
  <article>B</article>
  <article>C</article>
</switcher-l>
```

### `<cover-l>` — one main element vertically centered, header/footer at the edges

Vertically centers a "main" element inside a full-height container, with an
optional header above and/or footer below — robust to overflow and dynamic
content height.

- `space` — margin around/between children. Default `var(--space-4)`.
- `min-height` — min block size of the cover. Default `100vh`.
- `centered` — selector for the centered (main) element. Default `h1`.
- `no-pad` (boolean) — removes the container's own padding.

```html
<cover-l centered="h1">
  <header>Header</header>
  <h1>Main content</h1>
  <footer>Footer</footer>
</cover-l>
```

### `<grid-l>` — a responsive grid of equal-sized cells

A `repeat(auto-fit, minmax(...))` grid that reflows column count as space
changes, without fixed breakpoints or overflow risk.

- `min` — minimum cell inline size, floored to 100% of the container.
  Default `16rem`.
- `space` — gap between cells. Default `var(--space-4)`.

```html
<grid-l min="12rem">
  <div>Card</div>
  <div>Card</div>
  <div>Card</div>
</grid-l>
```

### `<frame-l>` — forces an aspect ratio and crops media to fill it

Gives an element (typically an `img`/`video` child) a fixed aspect ratio,
cropping via `object-fit: cover` regardless of the media's own dimensions.

- `ratio` — CSS `aspect-ratio` value. Default `16 / 9`.

```html
<frame-l ratio="1 / 1"><img src="/photo.jpg" alt="" /></frame-l>
```

### `<reel-l>` — a horizontally scrolling row of items

An accessible, JS-optional alternative to a carousel: a single-file row of
items that scrolls horizontally with native scrolling instead of wrapping.

- `space` — gap between items. Default `var(--space-2)`.
- `item-width` — inline size of each item. Default `auto`.
- `height` — block size of the reel itself. Default `auto`.
- `no-bar` (boolean) — hides the scrollbar.

```html
<reel-l no-bar>
  <img src="/thumb-1.jpg" alt="" />
  <img src="/thumb-2.jpg" alt="" />
</reel-l>
```

### `<imposter-l>` — superimposes an element over its positioning context

Absolutely (or fixed-) positions an element centered over its containing
block, recentered via `transform` so no foreknowledge of its own dimensions
is required. Used for dialogs, popups, dropdowns.

- `margin` — min space to the positioning container's edges (when not
  `breakout`). Default `0px`.
- `fixed` (boolean) — positions relative to the viewport instead of the
  nearest positioned ancestor.
- `breakout` (boolean) — allows the element to exceed the positioning
  container's bounds.

```html
<div style="position: relative;">
  <imposter-l><dialog open>Modal content</dialog></imposter-l>
</div>
```

### `<icon-l>` — sizes, aligns, and spaces an inline icon next to text

Sizes an inline SVG icon to track font size and sit on the text baseline,
with correct LTR/RTL spacing to adjacent text.

- `space` — spacing to adjacent text (`margin-inline-end`). Default: none
  (natural word spacing).
- `label` — when set, reflects `role="img"` and `aria-label` for assistive
  tech; when unset, the icon is treated as decorative.

```html
<p><icon-l label="Warning"><svg>...</svg></icon-l> Low on time</p>
```

### `<container-l>` — establishes a container-query context

Doesn't lay anything out itself — sets `container-type: inline-size` (and
optionally a name) so descendant CSS can use `@container` queries against
it. An escape hatch for cases a query-free layout genuinely can't solve,
not a default choice.

- `name` — CSS `container-name` value. Default: none.

```html
<container-l name="card">
  <div class="card"><!-- styled with @container card (...) { ... } --></div>
</container-l>
```

## When to reach for which

**Stack vs. Cluster vs. Sidebar vs. Switcher vs. Reel** — all group multiple
children, but differ in axis and wrap behavior:

- **Stack**: one axis (block/vertical), no wrap logic — just spacing between
  stacked elements.
- **Cluster**: inline items of varying, unrelated widths that should wrap
  freely, each item equal priority (tags, button groups, nav).
- **Sidebar**: exactly two children — one narrow, one flexible — that wrap
  to fully stacked as a pair, not a multi-item wrap.
- **Switcher**: several same-priority children that flip *as a whole* between
  row and column at one threshold — never a partial wrap with uneven rows.
- **Reel**: items that must **not** wrap — they scroll horizontally instead
  (filmstrips, carousels).

**Center vs. Sidebar** — Center is single-column: it centers one block of
content and caps its width. Sidebar places two different elements
side by side. Don't reach for Center to lay out two things next to each
other.

**Cover vs. Center** — Cover vertically centers one "main" element inside a
full-height container with optional header/footer at the edges (hero
sections). Center only centers horizontally and has no header/footer
concept.

**Grid vs. Cluster vs. Switcher** — Grid is for a true grid of same-sized
cells (card grids, image grids). Cluster is inline wrapping of items with
no shared size or column alignment. Switcher is for a small, fixed set of
same-priority items that should flip as a unit, not reflow into a grid.

**Imposter vs. Cover** — Imposter takes an element *out of flow* to
superimpose it over something else (modals, dropdowns, popups). Cover keeps
its main element in flow, just vertically centered within a taller
container.

**Frame vs. Box** — Frame forces an aspect ratio and crops media
(`object-fit: cover`) to fill it. Box is a plain padded/bordered rectangle
with no ratio enforcement — use it for cards, panels, callouts.

**Container** is not a layout choice among the others — it doesn't arrange
its own children at all. Reach for it only when wrapping an element so its
*descendants'* CSS can use `@container` queries, and only after confirming
an intrinsic (query-free) layout genuinely can't solve the problem — see
ADR-0003 and the Container section of
`docs/research/every-layout-css-primitives.md`.

**Icon** is not a container layout at all — it's for sizing/spacing a
single inline icon next to text.
