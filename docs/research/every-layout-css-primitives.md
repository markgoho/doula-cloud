# Every Layout, read firsthand — the book behind ADR-0003

Research for GitHub issue [#520](https://github.com/markgoho/doula-cloud/issues/520)
(part of [#518](https://github.com/markgoho/doula-cloud/issues/518)). Source: the
Every Layout EPUB/PDF itself — Heydon Pickering and Andy Bell, **3rd edition,
point release 3.1.7.14** (the "Container" pseudo-layout and the "now with
logic(al) properties" cover badge date this exact release) — read cover to
cover, all 172 pages, at `/Users/mgoho/Downloads/every-layout.pdf`. Every claim
below cites the page it came from. Page numbers are PDF page numbers (page 1 is
the cover).

This supersedes the file's previous version, which was compiled from
every-layout.dev web pages (some free, some fetched through a paid-license
browser session) rather than the book. That version is not simply "thin" —
parts of it are accurate reproductions of the same content (the paid web tier
ships the same text as the book), but its own analysis, at the one point where
it draws a conclusion the book explicitly addresses, contradicts the book's
stated position. See "Where the previous version was wrong" below.

## Where the previous version was wrong

- **It reversed the book's own verdict on container queries vs. Sidebar's
  technique.** The old file's self-authored "Assessment" section argued that
  since container queries have now shipped, "a from-scratch
  Sidebar/Switcher-equivalent could react to the *component's own* available
  width via `container-type: inline-size` instead of... the flex-basis
  arithmetic tricks Every Layout documents" — i.e., that `@container` is the
  more capable modern replacement for Sidebar's technique. The book's own
  dedicated **Container** chapter (pp. 165–172), which the old file also had
  access to and summarized correctly in isolation, argues the *opposite*, with
  a worked example: a container query only knows the container's own
  dimensions, not the state of the elements inside it, so reproducing
  Sidebar's self-adjusting breakpoint with `@container` requires manually
  re-deriving a breakpoint per sidebar width — shown as a `:has()`-based
  multi-rule mess that is *more* complex than Sidebar's original CSS (p. 168).
  The book calls `@container` (and `@media`) "circuit breakers we wire into
  layouts we know are going to error... I'd sooner not have them anywhere I
  know they're not needed" (p. 167). The old file's own editorializing
  contradicted the very source it was citing, in the one place a firsthand
  read would have caught it.
- **It omitted composition almost entirely**, despite the book building its
  whole rationale for having *primitives at all* on it. The Composition
  chapter (pp. 14–17) isn't a policy note about BEM naming (which is as far as
  the old file's summary went) — it's three worked diagrams showing a dialog,
  a registration form, and a conference-slide layout each decomposed into
  nested Stack/Box/Center/Cluster/Cover/Sidebar instances, plus the book's
  literal definition of a "layout" as a tree where every node is either an
  element or another layout (p. 25). See "Composition" below — this is the
  part the issue asks to be recorded, not just the catalogue.
- **It missed the book's own vocabulary for exactly the territory it was
  editorializing about**: Sidebar, Switcher, and the Flexbox grid are each
  called a **"quantum layout"** — one CSS declaration existing simultaneously
  in more than one configuration until the browser resolves which one applies,
  based on available space, with no discrete state chosen ahead of time (pp.
  85, 94, 96). This is the book's load-bearing alternative to reaching for a
  query at all, and it isn't in the old file anywhere.
- **Several specific accessibility callouts are absent** that the book states
  explicitly next to the primitive the old file did otherwise describe
  accurately: the warning that horizontally centering content risks moving it
  out of view for a zoomed-in user, since only the left edge is guaranteed
  visible (Center, p. 69); and "one `<h1>` per page," with successive
  `<cover-l>` instances needing an `<h2>` instead (Cover, p. 116).
- **The primitive catalogue and licensing terms were accurate.** All 13
  primitives, their custom-property tables, their default values, and the
  quoted licensing terms in the old file check out against the book. That part
  did not need fixing so much as re-sourcing.

## The rudiments: axioms before recipes

Every Layout's "Rudiments" section (pp. 6–45) is the reasoning the 13
primitives sit on top of. The issue asks for axioms over recipes — this is
where they live.

### Boxes (pp. 6–13)

"Everything in web design is a box... layout is inevitably, therefore, the
arrangement of boxes" (p. 6, citing Rachel Andrew). The chapter's real claim
is about *how* a box should get its dimensions: "the dimensions of our
elements should be largely *derived* from their inner content and outer
context. When we try to *prescribe* dimensions, things tend to go amiss... The
CSS of suggestion is at the heart of algorithmic layout design. Instead of
telling browsers what to do, we allow browsers to make their own calculations"
(p. 13). `box-sizing: border-box` is recommended universally via `*` — with
one named exception, `.center-l { box-sizing: content-box }`, because Center
needs to measure content itself rather than content-plus-padding (p. 11).

### Composition (pp. 14–17)

Covered in its own section below — this is the chapter the issue is most
pointed about.

### Units (pp. 18–23)

Argues against `px`: a CSS pixel isn't a stable atomic unit (sub-pixel
rendering, device pixel ratios, zoom), and `px` font sizing overrides a user's
browser/OS font-size preference — "there are more users who adjust their
default font size in browser settings than there are users of the browsers
Edge or Internet Explorer... disregarding users who adjust their default font
size is as impactful as disregarding whole browsers" (p. 19, citing Evan
Minto). Also rejects width-based `@media` breakpoints as arbitrary: "What's so
special about `960px`? Can we really say the smaller size is acceptable at
`959px`?" (p. 21). States the load-bearing analogy this repo's ADR-0023 leans
on without citing: **"The `em` unit is to the `rem` unit what a container
query is to a `@media` query"** (p. 21) — `em` is context-relative the way a
container query is, `rem` is document-relative the way a `@media` query is.
`ch` is singled out as "the only appropriate unit" for measure, since measure
is inherently a characters-per-line question (p. 23).

### Global and local styling (pp. 24–33)

Sets out a three-tier system, credited to Harry Roberts' ITCSS ("specificity
is inversely proportional to reach," p. 29): (1) universal/inherited styles,
(2) layout primitives, (3) utility classes (`property:value` naming,
`!important`-suffixed, "for final adjustments," p. 27). The chapter states the
tree structure a layout is, in the book's own words: **"Each layout requires a
container element which establishes a formatting context for its children.
Simple elements, without children for which they establish a context, can be
thought of as 'end nodes' in the layout hierarchy"** (p. 25, caption under a
diagram literally titled "layout" with three `element or layout` leaves under
it) — this is the composition mechanism stated as a rule, not just shown by
example. The chapter's taxonomy of *local* (instance-specific) styling is
`id` selectors, inline `style` attributes, and Shadow DOM (p. 30) — it does
not consider build-time component-scoped stylesheets (CSS Modules-, Vue-,
Svelte-style) as a category at all, which matters for how this repo reads it
(see "Conflicts" below).

### Modular scale (pp. 34–38)

A single ratio (book's own worked example: 1.5, matching a `line-height` of
1.5) drives signed, step-indexed custom properties (`--s-1`... `--s0`...
`--s5`) via repeated multiplication/division, "intended for producing harmony"
the way a musical scale is (p. 37). Every primitive's default spacing value
(`var(--s1)`) draws from this scale.

### Axioms (pp. 39–45)

Defines "axiom" as a small, global, unqualified design rule — worked example:
**"the measure should never exceed 60ch"** — enforced "as pervasively as
possible" via the lowest-specificity mechanism available (universal
`*`-plus-exceptions), not applied manually element-by-element (pp. 39–41). The
chapter argues explicitly for a deny-list over an allow-list: "An exception
based approach is smarter, since we only have to remember which elements
should *not* be subject to the rule" (p. 42). Primitives reuse the same
`--measure` custom property as a prop default with silent fallback on invalid
input — the Switcher's `threshold` prop is the worked example (p. 45).

## The 13 primitives, in the book's own terms

Every one of these is presented as a single CSS rule (or small rule-set)
solving one narrow, named problem, with a props table for the custom-element
implementation. Page ranges cover each chapter in full (problem → solution →
generator → component → examples).

| Primitive | Problem it names (book's own framing) | Core mechanism | Pages |
|---|---|---|---|
| Stack | Margin belongs to the *relationship* between two elements, not either element — direct margin styling produces doubled-up or orphaned spacing | Owl selector: `.stack > * + * { margin-block-start: var(--space, 1.5rem); }` | 46–56 |
| Box | A layout primitive (Stack) should do one job; box shape (padding, visible border/background) is a *different* job that needs its own primitive so Stack's job description stays a nonsense-free single sentence | Symmetrical padding only, forced `color: inherit`, transparent `outline` for high-contrast mode | 57–64 |
| Center | `text-align: center` only centers text, and `margin: 0 auto` collides with vertical margins a parent Stack may already have applied | `margin-inline: auto` + `max-inline-size`, or Flexbox `align-items: center` for "intrinsic" (content-width) centering | 65–73 |
| Cluster | Words in a paragraph wrap and space themselves evenly on every wrapped line; a list of same-priority elements (buttons, tags) needs the same behavior but earlier techniques double spacing at edges or drop it on wrap | `display: flex; flex-wrap: wrap; gap: var(--space)` | 74–82 |
| Sidebar | Two adjacent elements (narrow + wide) need to lay out responsively *to the space they are given*, not the viewport, and wrap to stacked without an awkward in-between state | `flex-grow: 999` on the non-sidebar forces the sidebar to its ideal width until `min-inline-size` forces a wrap | 83–93 |
| Switcher | A set of equal-priority elements needs to switch directly between a row and a stack at a *container* threshold, without the uneven-row-counts "orphan" problem multi-column wrapping produces | `flex-basis: calc((var(--threshold) - 100%) * 999)` — resolves to a huge positive or invalid-negative value, forcing full-width or side-by-side | 94–104 |
| Cover | One "principal" element should stay vertically centered with optional header/footer, robust to dynamic content height, with zero CSS change when header/footer are added or removed | `margin-block: auto` on the principal child, `:not()` strips redundant top/bottom margin from the actual first/last child | 105–116 |
| Grid | A grid-like formation should reconfigure automatically as space changes, without a fixed column count or `@media` breakpoints, and without a hard-coded minimum causing overflow below it | `repeat(auto-fit, minmax(min(x, 100%), 1fr))` — final answer uses the CSS `min()` function, not JavaScript or `@container` | 117–129 |
| Frame | An arbitrary element needs a fixed aspect ratio and cropped content, without hard-coded width/height, for media of unpredictable dimensions | `aspect-ratio: n / d` (formerly the `padding-bottom` percentage hack) + `object-fit: cover` | 130–139 |
| Reel | An accessible, JS-optional alternative to a carousel widget — a horizontally scrolling single-file row using native scrolling | `display: flex` without `flex-wrap`, `overflow-x: auto` | 140–152 |
| Imposter | A general-purpose way to superimpose one element centrally over another, without foreknowledge of the imposed element's own dimensions | `position: absolute; inset-block-start: 50%; inset-inline-start: 50%; transform: translate(-50%, -50%)` | 143–153 |
| Icon | An inline SVG icon needs to track font size, sit on the text baseline, and space correctly from accompanying text in both LTR and RTL, without bespoke per-instance CSS | `width/height: 0.75em` (`1cap` as an enhancement layer), `margin-inline-end` for LTR/RTL-correct spacing | 154–164 |
| Container | Not a layout — a "meta-layout utility" that establishes `container-type: inline-size` (named or unnamed) so other CSS can query it | `container: myContainer / inline-size;` | 165–172 |

Custom-property defaults, per-primitive props tables, and generator code are
unchanged from what the old file already captured accurately from the paid web
tier (which mirrors the same book text) — not reproduced again here in full;
see the page ranges above to verify any specific value against the source.

## Composition: how primitives nest and combine

This is the part the issue calls out as "the part a component author gets
wrong first," and it is the weakest part of the superseded file.

**The book's argument for having primitives at all is a composition
argument, not a catalogue argument.** The Composition chapter opens by naming
"composition over inheritance," borrowed from programming and explicitly
citing React's docs on the same principle (p. 14). Its worked example is a
dialog box, first built the way most component libraries build it — a
`.dialog` block with `.dialog__header`, `.dialog__body`, `.dialog__foot`
children, BEM-namespaced. The book's diagnosis: **"since everything here is
namespaced under `.dialog`... when we come to make the next component, we'll
end up duplicating would-be shared styles. This is where most CSS bloat comes
from"** (p. 15). The fix isn't a different naming convention — it's realizing
the dialog was never one thing: "The mistake in the last example was to think
of everything about the dialog's form as isolated and unique when, really,
it's just a composition of simpler layouts" (p. 15).

**"Primitive" is used in its programming-language sense on purpose.** "A
primitive is something without its own meaning or purpose as such, but which
can be used *in composition* to make something meaningful... In JavaScript,
the Boolean data type is a primitive. Just looking at the value `true`... tells
you very little about the larger... application. The object data type, on the
other hand, is *not* primitive... The dialog is meaningful, as a piece of UI,
but its constituent parts are not" (p. 15). Stack, Center, Cluster, Box and
the rest are deliberately meaningless alone — the meaning is assembled at the
call site, by nesting.

**The book then shows the assembly, three times, as diagrams — not
prose:**

- The dialog box is re-drawn as **Stack** (wrapping the whole thing) ⊃
  **Center** (the heading) + **Box** (the outer shape) + **Cluster** (the
  Okay/Cancel button row) (p. 16).
- A registration form is **Stack** (top level: label/input pairs plus the
  submit row) ⊃ nested **Stack** instances (each label-above-input pair) +
  **Cluster** (the submit button, right-aligned) + **Box** (the form's outer
  shape) (p. 16).
- A conference-talk slide layout is **Box** (outer shape) ⊃ **Stack** (title
  above body) + **Cover** (vertically centers the slide content) + **Sidebar**
  (edit/delete/share controls beside the slide) (p. 17).

The pattern across all three: **one primitive wraps another primitive**, each
establishing its own formatting context for its own children, and *no
primitive knows or cares what it contains* — a Stack's children can themselves
be a Cluster, a Box, or plain content, because the Stack's rule (`.stack > * +
*`) matches by position, not by type. This is the concrete referent for the
tree-of-`element-or-layout` rule already stated in "Global and local styling"
(p. 25, quoted above) — Composition is where that abstract rule is shown
working on real UI.

**"Quantum layout."** Three primitives are explicitly named this way: Sidebar
"is a *quantum* layout, existing simultaneously in one of the two
configurations — horizontal and vertical — illustrated below. Which
configuration is adopted is not known at the time of conception, and is
dependent entirely on the space it is afforded when placed within a parent
container" (p. 85). Switcher and the Flexbox-based Grid get the identical
framing (pp. 94, 96) — "quantum layouts existing simultaneously in different
states." This is the book's name for exactly the "responds to its own space,
not a chosen breakpoint" property ADR-0003 and ADR-0023 both assume, and it is
absent from the superseded file entirely.

**A composed layout inherits its children's spacing rules, not the other way
round**, and the book is explicit that this needs deliberate handling, not
assumption: the Stack chapter's "Nested variants" section shows resetting
vertical margin at the top and re-declaring it per nesting level
(`.stack-large > * + *`, `.stack-small > * + *`, p. 49) specifically because a
plain recursive Stack selector (`.stack * + *`) would also space out elements
that were never meant to be spaced, like `<li>` items inside a `<ul>` (p. 48).
Composition is not free of surprises just because the primitives compose.

## Relationship to container queries, named grid areas, and intrinsic sizing keywords

ADR-0023 leans on three mechanisms — container queries, `grid-template-areas`,
and the intrinsic sizing keywords `min-content`/`max-content`/`fit-content` —
that the book predates in parts and only partially covers even in this
current (3.1.7.14) edition. The relationship is not uniform across the three:

- **Container queries: covered directly, and the book's position is a genuine
  qualification of ADR-0023's stance, not just an old-book gap.** The
  dedicated Container chapter (pp. 165–172) opens by asking its own question —
  *"Now we have container queries, is Every Layout obsolete?"* — and answers
  no, arguing container queries and `@media` queries are "circuit breakers we
  wire into layouts we know are going to error... manual intervention" (p.
  167), to be reached for only when an intrinsically-sound query-free layout
  "genuinely can't be devised" (p. 165 framing, p. 172 "not really a layout"
  warning). Its worked argument: `@container` can only see the *container's*
  own size, never the internal state of its children, so replicating
  Sidebar's self-adjusting behavior (which already reacts correctly to any
  sidebar width without being told what that width is) with `@container`
  requires manually re-deriving a breakpoint per sidebar width via a
  `:has()`-based rule for every case — shown as strictly more code and less
  robust than Sidebar's original three-line solution (p. 168). ADR-0023's rule
  2 — "reach for the query-free answer first... when the query-free answer
  runs out, use a container query" — is consistent with this, but ADR-0023
  frames container queries as the *default* mechanism for structural
  rearrangement once the query-free tier is exhausted, with no justification
  owed. The book goes further than "default when needed": it treats a
  container query as evidence a layout has *not yet* been made properly
  intrinsic, and would ask what query-free option was ruled out first. This
  is a difference in posture worth naming, not a contradiction — both agree
  container queries are the right tool once genuinely needed.
- **Named grid areas: not covered at all.** Across the Grid chapter (pp.
  117–129) and the Imposter chapter, which is the only other place
  `grid-area` appears (as a numeric line-based placement example, `grid-area:
  2 / 2 / 5 / 8`, p. 144, explicitly *not* using named areas), the book never
  discusses `grid-template-areas`. Its Grid primitive solves a different
  problem — an auto-fit, auto-count responsive tiling of interchangeable
  cells (`repeat(auto-fit, minmax(min(x, 100%), 1fr))`, p. 127) — not a named,
  rearrangeable region layout. ADR-0023's rule 2 — "name the regions with
  `grid-template-areas`... a component whose regions are named can be
  rearranged wholesale inside `@container`... with no change to the markup" —
  describes a technique the book simply does not have an opinion on, because
  the book's Grid primitive is solving for a *list* of same-shaped items, not
  a component with distinct, nameable regions. This is a real gap between the
  book and this repo's mechanism, not a disagreement.
- **Intrinsic sizing keywords (`min-content`, `max-content`, `fit-content`):
  not named anywhere in the book.** The book's own use of "intrinsic" is
  narrower and specific: "My use of 'intrinsic' in this section specifically
  refers to the inevitable width of an element as determined by its contents.
  A button's width, unless explicitly set, is the width of what's inside it"
  (Sidebar chapter, p. 89) — i.e. the *absence* of an explicit size, not the
  named CSS keywords. The book's actual sizing mechanism throughout is
  `flex-basis` plus `flex-grow`/`flex-shrink` (Sidebar, Switcher, the Flexbox
  Grid) or `minmax()`/`min()` (the CSS Grid). ADR-0023's "the intrinsic
  sizing keywords `min-content`, `max-content` and `fit-content`" are a
  different, later-standardized toolset the book never reaches for. Same
  category as named grid areas: a gap, not a conflict.

## Conflicts with ADR-0003 and ADR-0023, named plainly

- **ADR-0003's "no Shadow DOM, deliberately" rationale is correct against the
  book's own stated reasoning**, and this firsthand read confirms rather than
  revises it: the "Eschewing Shadow DOM" blog post the ADR cites is separate
  from the book itself (it's a 2019 blog post, not a book chapter), but the
  book's own "Global and local styling" chapter independently corroborates
  the same conclusion from a different angle — it lists `id` selectors,
  inline styles, and Shadow DOM as the *only* three mechanisms it considers
  for local/instance styling (p. 30), and does not consider a build-time
  scoped stylesheet (Svelte's mechanism) as a category at all. ADR-0003's
  claim that this is a genuine gap the book leaves open, which Svelte's
  scoping addresses without Shadow DOM's specific cascade-blocking drawback,
  holds up against the book's own text — not just the separate blog post.
- **ADR-0023's framing of container queries as a no-justification-owed default
  is a stronger commitment to `@container` than the book itself makes**, per
  the previous section. This is not a contradiction that breaks anything —
  both documents want container queries used where genuinely needed and avoid
  them elsewhere — but a reader implementing ADR-0023's rule 2 should not
  expect the book to back "reach for `@container` as the default, no
  justification needed" without qualification; the book's own position is
  closer to "container queries are evidence of a layout that could not be
  made intrinsic, and that should be rare."
- **No conflict was found on Grid or intrinsic sizing keywords** — these are
  gaps (the book has no opinion), not disagreements, and ADR-0023 does not
  claim the book as its source for either; ADR-0003 cites the book only for
  the primitive catalogue and the custom-elements-without-Shadow-DOM decision,
  not for grid regions or sizing keywords.

## Licensing (carried forward, re-verified)

The book's own front matter (pp. 2–3) confirms the terms the superseded file
already quoted from the website: purchase is a one-time license to the
content "authored and owned by Heydon Pickering and Andy Bell," re-publishing
or re-selling is "strictly forbidden," and the license is revocable "for
unfair usage or irresponsible sharing." Nothing in this rewrite reproduces
book prose beyond short quotations for citation purposes, consistent with
those terms.

