# Doula Cloud design brief

**Direction: _Plum Dusk, evolved_.** Chosen by Mark Goho on 2026-08-28 from four
candidate directions generated for
[#409](https://github.com/markgoho/doula-cloud/issues/409), a sub-ticket of the
[Holistic application design](https://github.com/markgoho/doula-cloud/issues/405)
map.

This document is the brief. A later ticket must be able to build from it without
seeing the images, so everything here is written in words and numbers. The
images that produced it are kept at
[`directions/`](directions/) and on the
[comparison canvas](https://claude.ai/code/artifact/598a8036-be11-4b4a-9561-b5dd92fa5b24).

## Why this direction won

Three reasons were recorded, in the words given:

1. **It looked the best** of the four.
2. **Restraint is right for the job.** A form-heavy tool that a 14-doula agency
   reads all day should be quiet.
3. **It looks most like a SaaS application — Jakob's Law of UX.** People spend
   most of their time in other software; Doula Cloud should work the way that
   software already works.

The third reason governs the other two. A doula opening this product on her
first morning should already know where the nav is, what a primary button looks
like, and what happens when she clicks the avatar. Novelty in those places is a
cost paid by every user, every day.

A fourth thing was stipulated alongside the choice, and it is not a caveat:

4. **Doula Cloud should still have something of its own** — not so radical that
   a person cannot work out how to use it, but genuinely there. **Smooth UX is
   a primary goal**, and it is the thing the product should be recognised for.

So the direction's stated weakness — that it was the safest and least memorable
of the four — is *not* accepted as it stands. Familiarity is the floor, not the
ceiling. What follows is how both are satisfied at once.

## The governing principle

> **Conventional in pattern and behaviour. Distinctive in execution.**

Every decision below descends from that sentence. Where a convention exists —
a top bar, a flat nav, an avatar menu at the end of the chrome, a primary
button on the right of a form — follow it, and follow it exactly. Do not invent
a new interaction to be interesting. Spend the whole distinctiveness budget on
typography, rhythm and one signature component, described under
[Where the character comes from](#where-the-character-comes-from).

This is not a licence to be generic. A brief that only said "look like other
SaaS" would produce something anonymous, and anonymous is exactly what has been
ruled out. The instruction is narrower: **be unsurprising in what the interface
does, and unmistakable in how well it does it.**

**Smoothness is where the character lives.** The thing this product should be
recognised for is not a shape nobody has seen before — it is that everything
answers immediately, nothing jumps, focus always lands where a person expects,
a form remembers what was typed, and a long list never stutters. That is
difficult, it is felt by every user on every visit, and almost nothing in this
category does it well. It is a far better place to be distinctive than a novel
nav pattern, and it costs the user nothing to learn.

Concretely, smoothness is a requirement and gets checked, not hoped for:

- **Nothing shifts after it paints.** No layout jump when data arrives, no
  reflow when an image or a font loads. Space is reserved before content
  arrives; a font is loaded with a metric-compatible fallback.
- **Every action acknowledges itself within 100ms**, even when the work behind
  it takes longer. A button that has been pressed looks pressed.
- **Loading is skeletal, not spinning**, and only where content will actually
  appear. A spinner covering a whole page is a failure.
- **Focus is always visible and always predictable.** Closing a dialog returns
  focus to what opened it; submitting a form moves focus to the result.
- **Keyboard is a first-class path through the whole app**, not a fallback.
- **A dense list stays at 60fps while scrolling** under a real Practice's data,
  not a fixture's.

These are the standing expectations `CLAUDE.md` already sets — performance and
accessibility — restated as an aesthetic commitment, because in this direction
they *are* the aesthetic.

## Mood, in words

Quiet, professional, unhurried, and legible at the end of a long shift. The
product is used at 3am by somebody who has been awake for twenty hours, and at
9am by an owner reconciling invoices. It should never shout, never decorate,
and never make a person hunt.

Warm rather than clinical, but warm by a few degrees only — the plum is a
family resemblance, not a theme. Nothing here should read as domestic,
therapeutic, or soft. It is a record-keeping tool for a working practice.

## Colour

### Does `Plum Dusk` survive?

**It survives, and it changes.** The hue family is kept; the numbers are not.
This is a real edit to `app/src/lib/styles/tokens.css`, not a no-op.

| | Today (`tokens.css`) | This brief |
|---|---|---|
| Accent hue | `325` | **`339`** — rosier, less violet |
| Accent lightness | `48%` | **`41%`** — darker, so it carries text at small sizes |
| Accent chroma | `0.13` | `0.12` — effectively unchanged |
| Neutral hue | `320` throughout | **`~350`** — the neutrals lean pink, not violet |
| Neutral steps | one `--color-bg`, one `--color-panel-bg` | **five surface steps**, see below |

The direction that was chosen and rendered used hue `339`. Adopt `339`. Do not
quietly keep `325` on the grounds that it is close — it is the difference
between the palette that won the comparison and the one that did not.

### Light palette

OKLCH is authoritative; the hex is the value that was rendered and is given so
the two can be checked against each other.

| Token role | OKLCH | Hex | Used for |
|---|---|---|---|
| `surface` | `oklch(98.5% 0.008 7)` | `#fff8f9` | The page ground |
| `surface-container-low` | `oklch(96.7% 0.011 357)` | `#fbf1f4` | A panel one step up from the page |
| `surface-container` | `oklch(94.9% 0.011 357)` | `#f5ebee` | A panel two steps up |
| `surface-container-high` | `oklch(93.3% 0.010 2)` | `#efe6e8` | A panel three steps up |
| `surface-container-highest` | `oklch(91.5% 0.011 355)` | `#e9e0e3` | Hairline borders, dividers, the top-bar rule |
| `on-surface` | `oklch(22.4% 0.009 352)` | `#1f1a1c` | Body and heading text |
| `on-surface-variant` | `oklch(40.0% 0.023 343)` | `#51434b` | Secondary text, nav items at rest |
| `outline` | `oklch(57.3% 0.023 346)` | `#83737b` | Metadata, timestamps, icon strokes at rest |
| `outline-variant` | `oklch(83.1% 0.026 346)` | `#d5c1cb` | Form-control borders |
| `primary` | `oklch(41.2% 0.119 339)` | `#722c60` | Primary button fill, active nav, links |
| `primary-container` | `oklch(49.9% 0.121 339)` | `#8e4479` | The lighter accent tone; hover on primary |
| `error` | `oklch(50.6% 0.193 28)` | `#ba1a1a` | Validation and destructive |

Cards are **white** (`#ffffff`) on the `surface` ground, bounded by a one-pixel
`surface-container-highest` border. That pairing is the direction's signature at
the surface level: containers are declared by an edge, never by a fill and never
by a shadow.

### Rules the palette must obey

- **The accent appears on primary actions and active state, and nowhere else.**
  Not on card headers, not on icons at rest, not as a decorative rule. If a
  screen has more than a few plum marks on it, the screen is wrong.
- **Neutrals carry structure.** Grouping, hierarchy and separation are done with
  the five surface steps and the two outline tones. Reach for a surface step
  before reaching for colour.
- **Status colour is not accent colour.** `error`, and the status / info /
  warning family already in `tokens.css`, keep their own hues and are not
  harmonised toward the plum.
- **Contrast floors are non-negotiable.** Body text and its background meet
  WCAG 2.2 SC 1.4.3 at 4.5:1; form-control borders and other non-text
  boundaries meet SC 1.4.11 at 3:1. `outline-variant` was chosen for form
  borders for exactly this reason — the lighter divider tone does not qualify.

### Dark

Renders were light-only, by design, so dark is specified here in words and
derived by a later ticket rather than copied from an image.

- **Derive, do not invent.** Dark keeps the same hues — accent at `339`,
  neutrals at `~350`. Only lightness moves.
- **Invert the surface ladder, do not mirror it.** The darkest surface is the
  page ground; each container step gets *lighter*, the reverse of light mode.
  Nothing in dark mode is pure black, and nothing is pure white.
- **The accent must get lighter, not darker.** A `41%` plum on a dark ground
  fails contrast. Dark's accent sits near `oklch(84% 0.116 341)` — the
  `inverse-primary` tone the direction already carries — with a dark
  `on-primary` for text sitting on it.
- **The card convention flips cleanly.** A card in dark mode is one surface step
  above the page, still bounded by a one-pixel border. Still no shadows.
- **Both themes ship together.** A component is not done in light only.

The mechanism is already correct in `tokens.css` — `prefers-color-scheme` with a
`[data-theme]` override — and stays.

## Typography

**One family: Hanken Grotesk.** Headings, body, labels, buttons, tables. There
is no second family and no display serif. Hierarchy comes from size, weight and
tracking, never from a change of voice.

Hanken Grotesk is the deliberate choice, not a default. It is a humanist
grotesque with slightly open apertures and a real italic, so it stays legible at
13–14px in a dense table while having more character than the Inter / Roboto /
system-ui field the rest of the category sits in. It is on Google Fonts under
the SIL Open Font License; **self-host it** alongside the Phosphor icon
pipeline rather than linking to a Google CDN, for the same privacy and
performance reasons that decision was made for icons.

Fallback stack, in order, so a face that has not loaded still sets close to the
metrics: `"Hanken Grotesk", ui-sans-serif, system-ui, sans-serif`.

### The scale

The current `--text-*` ramp in `tokens.css` was never chosen — it is a
placeholder. Replace it. Sizes are `rem`, at a `16px` root.

| Step | Size | Weight | Line height | Tracking | Used for |
|---|---|---|---|---|---|
| `display` | `2.25rem` / 36px | 600 | 1.15 | `-0.02em` | The one page title on a hub |
| `heading-lg` | `1.75rem` / 28px | 600 | 1.2 | `-0.018em` | Page titles elsewhere |
| `heading` | `1.25rem` / 20px | 600 | 1.3 | `-0.012em` | Section headings |
| `subheading` | `1rem` / 16px | 600 | 1.4 | `-0.006em` | Card and group titles |
| `body` | `0.9375rem` / 15px | 400 | 1.55 | `0` | Prose, form values, list rows |
| `body-sm` | `0.875rem` / 14px | 400 | 1.5 | `0` | Table cells, dense rows |
| `label` | `0.8125rem` / 13px | 500 | 1.35 | `0.005em` | Field labels, nav, buttons |
| `meta` | `0.75rem` / 12px | 400 | 1.4 | `0.01em` | Timestamps, attribution, help text |

Three standing rules:

- **Tracking tightens as size grows and loosens as it shrinks.** The table above
  already does this; do not set a heading at `0` tracking.
- **Tabular figures everywhere a number can be compared** — money, dates,
  invoice numbers, counts in a table column. `font-variant-numeric: tabular-nums`.
- **Prose gets a measure.** The existing `--measure: 65ch` cap stays and applies
  to any run of reading text. Form fields are sized to their content, not to
  the measure.

## Density

The survey behind [#406](https://github.com/markgoho/doula-cloud/issues/406)
landed on one sentence, and this brief adopts it whole: **compact rows, airy
forms.**

- **Lists and tables are compact.** A table row is `40px` at `body-sm`. A person
  scanning fifty clients should see as many as will fit without the page
  becoming a wall.
- **Forms are comfortable.** A form control is `40px` tall with `12px` of
  horizontal padding; consecutive fields are `20px` apart; a labelled field
  group is `28px` from the next. Filling in an intake form is careful work and
  is not the place to save vertical space.
- **The spacing scale stays on a 4px base.** The existing `--space-*` ramp is
  correct and is kept. Page gutters are `40px` at desktop width; card padding is
  `20px 22px`.
- **The top bar is `60px`.** It carries the product name, the flat nav, the
  Practice switcher and the avatar menu on one line, and it does not grow.
- **Hit targets never fall below 44px** in any pointer or touch context, whatever
  the visual height of the control.

## Depth and shape

- **No drop shadows anywhere.** Not on cards, not on the top bar, not on
  dropdowns. Depth is expressed by the surface ladder and by one-pixel borders.
  A menu that must float is a white surface with a `surface-container-highest`
  border, and nothing else.
- **Radius is `8px`** on cards, panels, inputs and buttons. The `--radius-sm: 8px`
  token in `tokens.css` is already right; a second, larger radius is not
  introduced.
- **Borders are `1px`.** The `--border-thin` token stands. A `2px` rule is
  reserved for the active-state marker under a nav item and appears nowhere else.

## Motion

Motion is functional. It explains a change of state or a change of place, and it
is never decorative.

- **State transitions: 120ms, `ease-out`.** Hover, focus, active, disabled,
  expanding a disclosure. Anything slower feels sticky in a tool used all day.
- **Entering content: 180ms, `ease-out`,** with a small opacity and 4px
  translate. Toasts, menus, newly revealed field groups.
- **Navigation gets one view transition, and only one.** Cross-document view
  transitions are available on the browser target (latest Chrome, Edge and
  Safari). Use one shared-element transition per navigation at most — typically
  the page title — at `200ms`. Never animate a whole page.
- **Nothing moves that the user did not cause.** No carousels, no auto-advancing
  anything, no attention-seeking pulses.
- **`prefers-reduced-motion: reduce` removes movement, not feedback.** Under
  that query, transforms and view transitions are dropped; colour and opacity
  changes stay, so a control still visibly responds.

## Iconography

Unchanged. Phosphor `duotone` and `light`, self-hosted raw SVG with the sync
pipeline, per [#96](https://github.com/markgoho/doula-cloud/issues/96). This
direction works with it without argument: `light` for interface icons at rest,
stroked in `outline`, at 18px in the chrome and 20px beside a section heading.
`duotone` is reserved for empty states and other places where an icon is the
subject rather than a marker.

Stitch's generated HTML pulls Material Symbols. That is an artefact of the
generator and is not adopted.

## Where the character comes from

The direction is deliberately conventional in its patterns, so the whole
distinctiveness budget is spent in four places. A later ticket should be able to
point at these and say the work is done. The first of them is the largest.

1. **Smoothness, as specified under
   [The governing principle](#the-governing-principle).** It is listed first
   because it is the one a person actually notices, and the one that separates
   this product from the category. It is also the only item here that is not a
   visual decision — it is built, and it is measured.
2. **Typography, executed strictly.** One family with a real scale, tracking
   that changes with size, tabular figures where numbers line up, and a measure
   on prose. Most products in this category get this wrong; getting it right is
   visible even to somebody who could not say why.
3. **Rhythm.** One spacing scale, one card padding, one row height, one gutter,
   applied without exception. The character of a quiet interface is consistency,
   and consistency is what a Template layer exists to enforce — see
   [#410](https://github.com/markgoho/doula-cloud/issues/410).
4. **One signature component: the activity ledger.** Every feature in this repo
   records who did a thing and when; that is a standing expectation, not a
   feature. The Recent-activity feed on the hub is the visible face of it, and
   it appears again on every record. Give it a considered treatment — a fixed
   date column in `meta` at tabular figures, the event in `body`, the actor in
   `outline`, hairline-separated rows — and reuse that treatment everywhere an
   audit trail is shown. It is the one component that is recognisably this
   product rather than any practice-management tool.

**This is not a licence to add a fifth.** If a later ticket wants a signature
move that is not on this list, that is a change to the brief and belongs on the
map, not in a component.

## Adopting this in `hugo/`

The marketing site is out of scope for the map that produced this brief, and no
Hugo template changes happen because of it. The brief is nevertheless written so
the site can adopt it later without a redesign:

- **The palette, the type scale and the spacing scale are stated as values, not
  as app components.** Nothing above depends on Svelte, on the atom library, or
  on `tokens.css` specifically. A Hugo stylesheet can consume the same numbers.
- **Hanken Grotesk is self-hosted and open-licensed**, so the same font files
  serve both properties.
- **Where the two properties should differ, they may.** A marketing page is read
  once, at a distance, by somebody deciding; the app is read every day by
  somebody working. The marketing site may use the display step more freely and
  more generous vertical space. It must not introduce a second accent hue, a
  second family, or drop shadows.
- Adoption happens through the same process this map established — a ticket, a
  decision, a written record — as a separate effort.

## What this brief does not decide

Named here so nobody reads silence as permission:

- **The token file itself.** Which custom properties exist, what they are
  called, and how a component consumes them is the token-overhaul ticket's work.
  This brief gives the values; it does not give the API.
- **The application shell's implementation.** The top bar described under
  Density is the shape the chosen direction rendered. Building it for the Staff
  side and the Client portal is separate work.
- **Templates.** The archetype set, what a Template owns, and its API are
  [#410](https://github.com/markgoho/doula-cloud/issues/410).
- **The design tool.** Which surface the team designs on is
  [#412](https://github.com/markgoho/doula-cloud/issues/412), downstream of the
  pen.dev trial. Choosing this direction does not choose a tool; it gives every
  remaining tool experiment the same target.
- **The Client portal's relationship to the Staff side.** Whether the portal
  shares this palette exactly or is deliberately distinguished from it is not
  settled here.
