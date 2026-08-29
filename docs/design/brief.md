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

Smoothness is also where the brief's second governing reference bites hardest.
Jakob's Law is not adopted alone: the thirty laws at
[lawsofux.com](https://lawsofux.com/) govern this product's interaction design,
and each one's concrete obligation here is set out under
[The Laws of UX](#the-laws-of-ux).

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

> **Amended 2026-08-28 on [#417](https://github.com/markgoho/doula-cloud/issues/417).**
> The values below are unchanged from the rendered direction. Three *role
> assignments* changed, because the originals failed this brief's own contrast
> floors when the arithmetic was actually run, and a fourth changed because
> pure white is not used anywhere in this product. The reasoning is under
> [Rules the palette must obey](#rules-the-palette-must-obey); the arithmetic
> lives in `app/src/lib/styles/tokens.spec.ts` and runs in CI.

| Token role | OKLCH | Hex | Used for |
|---|---|---|---|
| `surface-bright` | `oklch(98.5% 0.008 7)` | `#fff8f9` | **Cards, menus** — the raised surface |
| `surface` | `oklch(96.7% 0.011 357)` | `#fbf1f4` | The page ground |
| `surface-container` | `oklch(94.9% 0.011 357)` | `#f5ebee` | Inset panels, table headers |
| `surface-container-high` | `oklch(93.3% 0.010 2)` | `#efe6e8` | A deeper panel |
| `surface-container-highest` | `oklch(91.5% 0.011 355)` | `#e9e0e3` | The deepest panel |
| `on-surface` | `oklch(22.4% 0.009 352)` | `#1f1a1c` | Body and heading text |
| `on-surface-variant` | `oklch(40.0% 0.023 343)` | `#51434b` | Secondary text, nav items at rest |
| `on-surface-muted` | `oklch(50.8% 0.023 346)` | `#706068` | Metadata, timestamps, help text |
| `outline` | `oklch(57.3% 0.023 346)` | `#83737b` | Form-control borders, icon strokes at rest |
| `outline-variant` | `oklch(83.1% 0.026 346)` | `#d5c1cb` | Hairlines, dividers, card edges, the top-bar rule |
| `primary` | `oklch(41.2% 0.119 339)` | `#722c60` | Primary button fill, active nav, links |
| `primary-hover` | `oklch(49.9% 0.121 339)` | `#8e4479` | The lighter accent tone; hover on primary |
| `on-primary` | `oklch(99% 0.005 339)` | — | Text and icons on the accent |
| `error` | `oklch(50.6% 0.193 28)` | `#ba1a1a` | Validation and destructive |
| `on-error` | `oklch(99% 0.005 28)` | — | Text on an error fill |

Cards sit on the `surface` ground at `surface-bright`, bounded by a one-pixel
`outline-variant` border. That pairing is the direction's signature at the
surface level: containers are declared by an edge, never by a fill and never by
a shadow.

**A card is the top of the surface ladder, not an exception above it.** The
original wording made cards pure `#ffffff` — lighter than a ground that every
other step descends from, so the card was the one element escaping the ladder,
and escaping it into a value this brief forbids by name two sections later. It
is now simply the lightest rung. Nothing in this product is pure white or pure
black, in either theme: those extremes fatigue the eye over a long shift, which
is the wrong trade for a tool read at 3am.

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
- **Contrast floors are non-negotiable, and are checked rather than claimed.**
  Body text and its background meet WCAG 2.2 SC 1.4.3 at 4.5:1. The boundary of
  a **user-interface component** — a form control, a focus ring — meets SC
  1.4.11 at 3:1, which is why form borders use `outline` and not the lighter
  `outline-variant`.

  SC 1.4.11 covers UI components and graphics that carry meaning. It does **not**
  cover a decorative divider, and this brief does not pretend otherwise: forcing
  a hairline to 3:1 would drag it to a mid-grey and destroy "containers are
  declared by an edge". `outline-variant` is exempt, deliberately.

  These roles follow [Material Design 3](https://m3.material.io/styles/color/roles)
  as M3 defines them: `outline` is a component's border, `outline-variant` is a
  decorative divider, and `surface-bright` is the surface lighter than the page
  ground. An earlier draft of this brief swapped the first two and hard-coded
  the third, which is precisely what produced its failing pairs — a
  form-control border at **1.62:1** against a 3:1 floor, and metadata text at
  **3.83:1** against a 4.5:1 floor. Metadata now has its own tone,
  `on-surface-muted`, because one value cannot serve two floors at once.

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
   `on-surface-muted`, hairline-separated rows — and reuse that treatment
   everywhere an audit trail is shown. It is the one component that is
   recognisably this product rather than any practice-management tool.

   > **Amended 2026-08-29 on [#433](https://github.com/markgoho/doula-cloud/issues/433),
   > in three places.**
   >
   > **The actor's colour.** This entry originally read *"the actor in
   > `outline`"*. `outline` is the form-control-border role, chosen for SC
   > 1.4.11's 3:1 boundary floor; as text on `surface-bright` it sits under
   > this brief's own non-negotiable 4.5:1. `on-surface-muted` exists because,
   > in this brief's own words, "one value cannot serve two floors at once".
   >
   > **No vertical column rule.** A fixed-width date column already aligns the
   > dates, so a rule between the columns is decoration, and decoration is
   > ruled out two sections above. Hairlines separate rows and nothing else.
   >
   > **It is built, and it is not what people come for.** Eight journey maps
   > were searched for a moment where anyone — staff or Client — needs to
   > reconstruct who did what and when. Staff: none. The four near misses are
   > all something else (a Visit's *content*, an *agreed fee* recalled, a
   > *current* Contract status, an Offer). Clients: fragments, never a feed —
   > *did I sign that*, *what have I paid*, *when did Maya come*. So the ledger
   > keeps its place on this list, because `CLAUDE.md` requires the record and
   > because the treatment is genuinely this product's own — but the claim that
   > it is *"the one a person actually notices"* is withdrawn. It sits low on
   > every page it appears on, and behind a closed disclosure in the Client
   > portal. What people come for is *who is on this birth*, *has she signed*,
   > and *what do I owe*.
   >
   > Its time format is set by
   > [ADR-0022](../adr/0022-one-activity-log-with-a-subject-and-three-kinds-of-actor.md):
   > relative under seven days, absolute beyond, on a 12-hour clock with a
   > lowercase `am`/`pm`, and the exact instant always carried underneath.

**This is not a licence to add a fifth.** If a later ticket wants a signature
move that is not on this list, that is a change to the brief and belongs on the
map, not in a component.

## The Laws of UX

Jakob's Law decided the direction, and it is not adopted alone. **The thirty laws
at [lawsofux.com](https://lawsofux.com/) are this product's standing reference
for interaction design**, on the same footing as the palette and the type scale.

They are listed here once each, grouped by what they actually decide, with **what
each one demands in Doula Cloud** — not its definition, which the site already
gives. A law with no concrete obligation here would be decoration, so every row
names a real screen, a real number, or a real prohibition. Where two laws pull
against each other, the conflict is named and resolved at the end rather than
left for somebody to discover mid-ticket.

### The shell, navigation and controls

| Law | What it demands here |
|---|---|
| **Jakob's Law** | The governing law of this brief. Top bar, flat nav, tenant name and switcher, account behind an avatar at the end of the chrome — the arrangement every product in the [#406](https://github.com/markgoho/doula-cloud/issues/406) survey converged on. No invented navigation pattern, ever. |
| **Mental Model** | The app's nouns are the domain's nouns. A Practice, a Client, an Engagement, an Offer, a Staff member and a Plan mean on screen exactly what `CONTEXT.md` says they mean. A screen never renames a domain concept for the sake of a nicer label, and never invents a grouping the domain does not have. |
| **Fitts's Law** | Primary actions are large and close to where the work happened — a form's submit button sits at the end of the form, not in a distant toolbar. Destructive actions are deliberately *not* adjacent to their benign neighbours. Hit targets never fall below 44px, as already stated under Density. |
| **Selective Attention** | The plum accent is the attention signal and is spent on primary actions and active state only. Nothing else competes: no coloured card headers, no badges used decoratively, no second accent. A screen where the eye cannot find the one thing to do next has failed this law, not the user. |
| **Serial Position Effect** | First and last items in a series are remembered, so put the important ones there. In the top bar, the product identity is first and the account menu is last. In an activity ledger the newest event is first. In a long form, the fields a person came to fill in are not buried in the middle. |
| **Paradox of the Active User** | Nobody reads onboarding. Every screen has to be usable cold, on the first morning, with no tour and no tooltip tour. Help is inline, at the field, at the moment it is needed — never a modal that must be dismissed before work can start. |

### Forms — the hardest surface in this product

[ADR-0017](https://github.com/markgoho/doula-cloud/blob/trunk/docs/adr/0017-twelve-columns-a-practice-defined-layer-and-an-engagement-that-is-asked-for.md)
grows a Client from two fields to a twelve-column structural core **plus a
Practice-defined layer of arbitrary added fields**. That makes the intake form
the screen where these laws bite hardest, and where getting them wrong is most
expensive.

| Law | What it demands here |
|---|---|
| **Hick's Law** | Decision time grows with the number and complexity of choices. A form is presented in sequence, not as a wall: one decision at a time where the domain allows it, and a default chosen for the person wherever a sensible default exists. Applies as hard to Settings as to intake. |
| **Choice Overload** | A Practice that has defined thirty custom fields must not get thirty inputs on one screen. Practice-defined fields are grouped into Practice-named sections, appended below the structural core — Cliniko's shape, which #406 found is the one that matches ADR-0017. |
| **Miller's Law** and **Working Memory** | No group of fields, nav items or options exceeds roughly seven before it is broken up. Nothing a person must *remember* from an earlier step to complete a later one: if a later field depends on an earlier answer, the earlier answer is still on screen. |
| **Chunking** | The structural core is chunked into meaningful groups with headings — identity, contact, care, billing — not presented as twelve equal rows. Chunk by what the information *is*, never by what fits. |
| **Tesler's Law** | Some complexity is irreducible, and somebody absorbs it. A Practice's intake genuinely has many fields; the design's job is to carry that weight *for the doula*, in the software, rather than hand it to her as a longer form. Where complexity cannot be removed it is moved — to a default, to a template, to a later step — and never simply hidden. |
| **Postel's Law** | Be liberal in what a field accepts, conservative in what it emits. Phone numbers, dates and names are accepted in whatever shape a person types, then normalised on the way to storage. A field that rejects a valid answer over formatting is a defect, not validation. |
| **Goal-Gradient Effect** | Effort rises as the end comes into view, so a multi-step form always shows where the end is — a real step count, real progress, and never a fake one. A saved draft resumes at the step it left, not at the beginning. |
| **Zeigarnik Effect** | An interrupted task is remembered and wants finishing. Nothing in this product loses work: an intake form partially filled at 3am survives the tab closing, and an incomplete Engagement or unsent Invoice is visible somewhere as unfinished rather than silently absent. |
| **Parkinson's Law** | A task with no visible end inflates to fill the time available. Give every long task a stated shape — how many steps, what remains — so it does not become an open-ended sitting. This is the same instruction as Goal-Gradient, arrived at from the other side. |

### Grouping and visual structure

These five Gestalt laws are why this direction can drop shadows and still read
as structured. They are the mechanism behind "neutrals carry structure" in the
palette rules, and they are what a Template layer
([#410](https://github.com/markgoho/doula-cloud/issues/410)) exists to apply
consistently.

| Law | What it demands here |
|---|---|
| **Law of Proximity** | Spacing does the grouping before any border does. A label sits nearer its own field than the next field; a card's title sits nearer its own links than the card edge. If two things are related, close the gap before drawing a line. |
| **Law of Common Region** | A shared bounded area groups its contents. This is what a card *is* in this direction — a white surface inside a one-pixel `surface-container-highest` border. Use a region when a group genuinely is a unit; do not wrap things in a card to make a page look busy. |
| **Law of Uniform Connectedness** | The strongest grouping cue of the five. A run of rows sharing hairline dividers reads as one list, which is exactly how the activity ledger and every table are built. Never split one logical list across two visual containers. |
| **Law of Similarity** | Things that look the same are assumed to behave the same. One appearance per role, without exception: every link looks like every other link, every primary button like every other primary button. The corollary is stricter — **something that is not a link must never look like one.** |
| **Law of Prägnanz** | People resolve complexity into the simplest reading available. Prefer a plain rectangle, a plain rule, a plain list. Ornament that has to be decoded is a cost with no return, which is the same conclusion the direction reached on shadows. |
| **Von Restorff Effect** | The thing that differs is remembered — and this law is why the restrained accent works rather than being merely timid. Plum is memorable *because* it is almost the only colour on the page. Every additional coloured element spends that memorability. Applies once per screen. |

### Pace, feedback and how the product feels

These are the same commitments as
[the governing principle's smoothness list](#the-governing-principle), stated as
the laws they come from. [#418](https://github.com/markgoho/doula-cloud/issues/418)
turns them into checks.

| Law | What it demands here |
|---|---|
| **Doherty Threshold** | The interaction budget is **under 400ms**, end to end, for anything a person does routinely — opening a client, saving a field, filtering a list. The brief's stricter "acknowledge within 100ms" is the *feedback* floor and sits inside this budget; 400ms is the completion budget. Over 400ms, the work is done optimistically or with skeletal loading, never with a spinner over the page. |
| **Cognitive Load** | Every element on a screen costs the reader something. This is the standing argument for cutting: if a thing does not help this person do this task now, it is removed. No metric tiles nobody asked for, no decorative icons, no counts that answer no question. |
| **Flow** | A doula working through a caseload is in a state worth protecting. Nothing interrupts it: no modal that was not asked for, no notification that steals focus, no navigation that loses scroll position or unsaved work. |
| **Aesthetic-Usability Effect** | The reason a form-heavy internal tool deserves a real design at all — a considered interface is perceived as more usable and is forgiven more. It carries a warning that this brief accepts: it also **masks usability problems**, so a screen looking good is never evidence that it works. That evidence comes from watching somebody use it. |
| **Peak-End Rule** | An experience is judged by its peak and its end. The peaks here are the moments that matter to a practice — sending an invoice, a client accepting an offer, completing an intake — and each should end in a clear, calm confirmation of what happened and what comes next. A task that ends by dumping the person back on a blank page has wasted its best moment. |

### What gets built, and what gets cut

| Law | What it demands here |
|---|---|
| **Occam's Razor** | Between two designs that serve the task equally, take the one with fewer parts. Applies to components as much as screens: a new atom must justify itself against composing existing ones. |
| **Pareto Principle** | A small share of the work carries most of the value. Design the paths a 14-doula agency walks every day — the client list, intake, the activity ledger, invoicing — to a higher standard than the paths walked once a quarter, and be explicit about which is which rather than spreading effort evenly. |
| **Cognitive Bias** | Our own judgement is the unreliable instrument. Two working rules: a design is not validated by the team liking it, and a default is never neutral — whatever is pre-selected is what most people will accept, so choose defaults as deliberately as any other decision. |

### Where the laws conflict

Three real tensions. Each is resolved here so no ticket has to relitigate it.

- **Hick's Law and Choice Overload against Tesler's Law.** Fewer choices are better, and a Practice's intake genuinely needs many fields. **Tesler wins on what exists; Hick wins on what is shown at once.** No field is deleted to make a form shorter. Fields are sequenced, grouped, defaulted and deferred so that few decisions are present at any moment — and the irreducible complexity is absorbed by templates and defaults rather than passed to the doula.
- **Von Restorff and Selective Attention against the Aesthetic-Usability Effect.** Restraint makes the one accent memorable; a richer surface is perceived as more usable. **Restraint wins, and the aesthetic budget is spent on typography, rhythm and smoothness instead of on colour** — which is exactly what [Where the character comes from](#where-the-character-comes-from) already says.
- **Goal-Gradient and Zeigarnik against "nothing moves that the user did not cause".** Progress indicators and unfinished-work cues are motion and attention the user did not ask for. **The Motion rule holds:** progress is *shown*, never animated at somebody; an unfinished Engagement appears in a list, never as a badge that pulses or a banner that follows a person around.

**Standing instruction.** These laws are a checklist for review, not a vocabulary to
sprinkle through tickets. Cite one when it decides something. If a design has to
break one, say which and why in the ticket — that is a legitimate outcome, and an
undocumented one is not.

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

- ~~**The token file itself.**~~ **Settled** on
  [#417](https://github.com/markgoho/doula-cloud/issues/417): M3 role names
  under a `--color-` prefix, eight type steps at four properties each, and a
  closed purpose-named prop on `Text` and `Heading` so a route never names a
  size. Working through it amended the palette above; this brief gave the
  values, and the API it did not give now exists in
  `app/src/lib/styles/tokens.css`.
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
