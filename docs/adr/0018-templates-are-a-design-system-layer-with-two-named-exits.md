# Templates are a design-system layer, and there are exactly two ways to deviate from one

Twenty-three routes each invent their own page layout. That is not an accident, it is a decision:
[#93](https://github.com/markgoho/doula-cloud/issues/93)'s scope note read *"Atomic Design scope: atoms,
molecules, organisms only. Templates/pages stay as ordinary SvelteKit routes, not design-system
artifacts."* This document reverses it, on the wayfinder map
[Holistic application design](https://github.com/markgoho/doula-cloud/issues/405), decided on
[#410](https://github.com/markgoho/doula-cloud/issues/410).

The reversal is small. The part worth writing down is the second half of the title.

## Template means the Atomic Design layer, and the domain nouns stay qualified

`CONTEXT.md` already defines **Plan Template** and **Client Field Template**: a Practice's own field
definitions, per [ADR-0001](0001-practice-defined-plan-templates.md) and
[ADR-0017](0017-twelve-columns-a-practice-defined-layer-and-an-engagement-that-is-asked-for.md). Adding a
UI layer called Template creates an ambiguity, and it is tolerated rather than designed away, because
Atomic Design is one of two things the map declared it would not change.

The ambiguity is resolved by convention, not by renaming: **the domain nouns are always written
qualified**, and a bare "Template" therefore always means the Atomic Design layer. Considered and
rejected: calling the layer `PageShell` or `Scaffold`, which removes the collision at the cost of no
longer speaking Atomic Design in the one codebase organised by it.

The components themselves carry **no `Template` suffix** — `templates/RecordDetail.svelte`, not
`RecordDetailTemplate.svelte` — for the same reason `Button.svelte` is not `ButtonAtom.svelte`: the
directory carries the tier. This also keeps every bare "Template" in prose meaning the layer and never
a component.

## What a Template owns

A Template owns **page-level arrangement, and nothing else**.

- **Chrome belongs to `+layout.svelte`.** The top bar, the flat nav, the Practice switcher and the
  avatar menu described in [the design brief](../design/brief.md)'s Density section are built once per
  side — Staff and Portal — in the existing layout files. A Template never renders navigation and never
  renders sign-out, so it can be dropped into any route without knowing which side of the app it is on,
  and can be rendered in a test with no session.
- **Page gutters and max-width belong to the Template**, not the layout. This is the one piece of
  chrome-adjacent styling that moves down, and it moves so that a full-bleed document page — archetype
  G, the portal Birth Plan and Contract print views — can opt out later without fighting the shell.
- **Layout primitives are internal.** A Template is built out of [ADR-0003](0003-css-layout-primitives-as-native-custom-elements.md)'s
  light-DOM custom elements — `stack-l`, `sidebar-l`, `switcher-l` and the rest — and exposes none of
  them. No `gap` prop, no `sidebarWidth`. **Page-level arrangement is the Template's job; region-internal
  arrangement is the page's** — inside a region, a page still reaches for a primitive to put three
  buttons in a row. This finally gives the twelve primitives a clear consumer; they were built in one
  pass ahead of any.

## The API extends #97 rather than reopening it

[#97](https://github.com/markgoho/doula-cloud/issues/97) fixed typed Svelte props as the sole external
configuration surface: no `class` or `style` passthrough, no general CSS-variable escape hatch. A
Template is nothing but regions of markup passed in from outside, so it makes **Snippets** central where
they had been incidental. This is an extension, not a reversal — #97 governs *styling* escape hatches,
and a Snippet is content. It is the third confirmed use, after `LabeledField`'s typed `children` and
`DataTable`'s `rowActions`, which [#199](https://github.com/markgoho/doula-cloud/issues/199) already
called a confirmed second.

The shape is hybrid, and the hybrid is forced by a real page: the staff Engagement detail is an `h1`, a
`DescriptionList`, then **a variable number** of `<h2>`-headed sections — Visits, N Plan sections,
Contract, Invoices, Offers, Messages — which named region props cannot express. So fixed regions are
named Snippet props and the repeatable part is a typed array, `DataTable.rowActions`'s shape generalised.

| Archetype | Component | Regions |
|---|---|---|
| **B** Overview hub | `templates/OverviewHub.svelte` | `title`, `primary`, `secondary?`, and **`isEmpty` + `empty`, both required** |
| **D** Record detail | `templates/RecordDetail.svelte` | `title`, `summary?`, `actions?`, `sections: { heading, content }[]`, **`contents?`** — see the amendment below |
| **E** Long form | `templates/FormPage.svelte` | `title`, `intro?`, `fieldsets: { legend, content }[]`, `error?`, `actions` |
| **E** Question page | `templates/QuestionPage.svelte` | `journey`, `steps`, `allStepsHref?`, `backHref`, `errorSummary?`, `caption?`, `question`, `hint?`, `content`, `actions` — see the second amendment below |
| **E** Check answers | `templates/CheckAnswers.svelte` | `journey`, `steps`, `allStepsHref?`, `backHref`, `title`, `caption?`, `errorSummary?`, `sections`, `isWide?`, `actions` — same amendment |

`FormPage.fieldsets` is ADR-0017's shape: the twelve-column structural core is one fieldset and each
Practice-defined section is another appended below it — the pattern the
[#406](https://github.com/markgoho/doula-cloud/issues/406) survey found in Cliniko and endorsed as the
one matching ADR-0017.

`OverviewHub`'s required empty-state pair is the surprising entry, and it is the strongest argument for
the layer existing at all. `docs/journeys/evaluator-doula.md` names the Practice landing page as Tasha
Bell's **abandon point** — *"It is an empty filing cabinet, not proof"* — because nobody had to think
about the zero-data case. A Template that **cannot be instantiated without** an empty-state region makes
that structurally impossible to forget. It is the difference between a Template being page furniture and
being a rule.

## Two named exits, and no third

Uniformity is the point, but deviation has to stay possible or people will fight the Templates instead
of using them. The rule above closes every *anonymous* escape route — no `class`, no `style`, no
primitive knobs — so deviation needs a named mechanism.

There are exactly two:

1. **Opt out entirely.** A route that genuinely does not fit imports no Template and composes raw. This
   is visible in review as a deliberate act, and findable later by grepping for routes with no Template
   import. That grep *is* the enforcement mechanism, and it costs nothing to build.
2. **Propose a variant prop or a new Template.** The deviation is absorbed, and the second page that
   needs it gets it free.

Which exit applies is decided by the bar this repo already uses for extraction:
[#196](https://github.com/markgoho/doula-cloud/issues/196) built `DescriptionList` only once two
byte-identical consumers existed, while [#187](https://github.com/markgoho/doula-cloud/issues/187)'s
diverging pair did not clear it; [#189](https://github.com/markgoho/doula-cloud/issues/189) kept
`invite`'s inline `<code>` as a raw exception rather than widen `Text`. **One consumer stays a raw
exception; two identical consumers earn a variant or a Template.**

Considered and rejected: `class` passthrough on Templates only, on the grounds that they are page-level
rather than atoms. Rejected because it is anonymous and ungrepable — it reopens #97 by the side door,
and it removes the asymmetry that makes this work. The asymmetry is the whole design: deviating is
permitted and costs one moment of thought, conforming is free, so conformity wins by default rather than
by prohibition.

This rule is expected to be revised. As the Templates meet real routes, where deviation is acceptable —
or necessary — will become clearer than it can be now; what this document fixes is that deviation is
never *anonymous*, not that today's bar is final.

## Where Templates live, and how they are proven

`app/src/lib/components/templates/`, beside the three existing tiers. That location opts Templates into
both CI gates knowingly:

- The **100% line-coverage gate** on `src/lib/**`. Route files under `routes/**` sit outside it, which is
  why [#202](https://github.com/markgoho/doula-cloud/issues/202) shipped without a route spec; a Template
  is mostly markup, so its spec is cheap, but it is a real spec per Template rather than none.
- **`style-guide.spec.ts`**, which reads the tier directories off disk and fails CI for any component with
  no matching `/style-guide` page. Its `tiers` array grows by one string.

Each Template gets its own style-guide page rendered with realistic placeholder content, because an
unfilled Template is an empty frame. Those pages render **outside** `style-guide/+layout.svelte`'s
`box-l`/`stack-l` wrapper, inside a bordered frame standing in for the viewport — otherwise a Template
that owns its own gutters and max-width renders nothing like it does in the app. Exempting Templates from
the gate was considered and rejected: Templates are the layer most likely to drift, precisely because they
are the least visible.

## Scope

Seven layout archetypes were found across all 23 routes and are recorded in #405's Notes. **Three get
Templates here** — B, D and E. (Archetype E later turned out to be three Templates rather than one; see
the 2026-08-29 amendment below.) A (unauthenticated entry), C (index/list), F (settings/editor) and G
(document/print) are deliberately left, as is retrofitting the remaining routes onto these three.

The Practice landing page's *content* is not decided here either. The persona and journey documents supply
it — an Offer inbox, roster health, credit and Connect state, and the empty state — but three of the
blocks they ask for cannot be served: there is no practice-wide contracts-awaiting-signature endpoint, no
unpaid-invoice roll-up, and coverage is blocked at the schema, since `00007_visit.sql` gives a Visit no
date at all. Those are product and backend work beyond this map's destination.


## Amendment, 2026-08-29 — `RecordDetail` gains a `contents` region

Added on [#433](https://github.com/markgoho/doula-cloud/issues/433), drawing the two Engagement detail
pages. The staff one is the page this Template was shaped against, and drawing it produced **eight**
sections: Contract, Visits, Care Plan, Birth Plan, Invoices, Offers, Messages, Activity.

Eight `<h2>`s in one column is a page you scroll to search. Two journey gaps say so independently:
**PR-G5** ([#280](https://github.com/markgoho/doula-cloud/issues/280)) is that the Birth Plan is *"a
section partway down a long page with no deep link"*, found at Priya Raman's moment of truth — reading it
on a phone, in a hospital corridor, under time pressure — and **PR-G9**
([#283](https://github.com/markgoho/doula-cloud/issues/283)) is that the page renders **no `<a>` elements
at all**, so there is no way off it or around it.

So `RecordDetail` takes an optional **`contents`** region: a list of the page's own sections, rendered
beside the column at desktop width and as a jump-to strip under the title at narrow width. It is
optional because archetype D covers short records too, and a contents list above three sections is
furniture.

Two things this deliberately is not. It is **not a nav** — a Template still renders no navigation, and
these are in-page anchors, not routes, so the rule in *What a Template owns* stands unbroken. And it is
**not a tab set or an accordion**: `docs/design/govuk-alignment.md` marks both *"nothing needs one"*, and
hiding the Birth Plan behind a tab is PR-G5 with extra steps rather than a fix for it.

The arrangement is not new — it is the 260px rail plus 1052px column the Intake question pages already
established, so archetype D reads as the same product as archetype E rather than inventing a second page
geometry.

### Built on [#424](https://github.com/markgoho/doula-cloud/issues/424): the region is a boolean

`contents` is the one region that is **not** a Snippet. It is `isContentsShown`, a boolean, and the list
is derived from `sections`. That is what keeps the "it is not a nav" promise enforceable rather than
merely stated: a Snippet region would let a route hand the rail a route, and nothing in the Template
could stop it.

The drawing put a 260px rail beside a 1052px column, which is 1360px of content and wider than
`--page-max`'s 76rem. The token wins: the rail stays at its drawn 16.25rem and the column takes whatever
`--page-max` leaves. A second page width for one archetype would be a worse answer than a column 144px
narrower than a 1440px artboard suggested.

**Superseded on [#541](https://github.com/markgoho/doula-cloud/issues/541).** `--page-max` no longer exists: it froze a page column at 1216px, which would have spent a quarter of the fluid ramp [#531](https://github.com/markgoho/doula-cloud/issues/531) introduced, whatever monitor the page was on. The rail still stays at its drawn 16.25rem; the column now takes whatever the space it is given leaves, and the paragraph above stands only as the reason a second page width was refused.

The same list is rendered twice — a rail and a jump-to strip — with exactly one of them `display: none`
at any width, which takes the other out of the accessibility tree entirely. One list restyled by a
container query is not available: the two looks are `Link` variants (`rail` and `chip`), and an atom does
not get to know how wide its page frame is.

## Amendment, 2026-08-29 — archetype E is three Templates, and one of them renders a landmark

Added on [#464](https://github.com/markgoho/doula-cloud/issues/464), building what
[#432](https://github.com/markgoho/doula-cloud/issues/432) drew. There are now **five** Templates, not
three.

### Why a prop could not do it

`FormPage` renders `<Heading level={1}>` and, separately, `<fieldset><legend>`. A GOV.UK question page
needs the legend — or the `<label>`, where the page holds a single input — to **be** the `<h1>`, so a
screen reader announces the question once rather than twice. The [Dates
pattern](https://design-system.service.gov.uk/patterns/dates/) ships the markup:
`<legend><h1 class="govuk-fieldset__heading">`. That is a different tree, not a different attribute.

A mode flag on `FormPage` was considered and rejected on the account owner's call: it hides two
genuinely different page shapes behind a boolean, which is the thing the *two named exits* rule above
exists to prevent. `FormPage` keeps the job it is right for — a genuinely multi-fieldset form — and
after this its caller is `invite`, not `clients/new`.

- **`QuestionPage`** — one question per page, where GOV.UK's *one thing* is a question and not a field.
  The question is a discriminated union, `{ as: 'legend' }` or `{ as: 'label', for }`, so the `for` a
  label needs cannot be forgotten and cannot be supplied where it means nothing.
- **`CheckAnswers`** — the summary page that ends the sequence, with key / value / **Change** rows on
  hairline dividers and GOV.UK's two column widths.

### The step rail is a `<nav>`, which amends *What a Template owns*

The rule above says a Template never renders navigation and never renders a landmark. `QuestionPage`
and `CheckAnswers` both render `organisms/StepRail.svelte`, which is a `<nav>` named after the journey.
That is a deliberate amendment, decided by the account owner on #464 over the two alternatives (a plain
unnamed list, or a slot in the shell).

The rule's *reason* was that chrome is site-wide and session-derived, so a Template rendering it could
not be dropped into any route or rendered in a test with no session. A journey rail is neither: it is
page-scoped, handed in as data by the route, and means nothing outside the one sequence it belongs to.
**The shell cannot render it, because the shell does not know the journey.** So the amended rule is
that a Template renders no *chrome* navigation; journey navigation scoped to the page's own task
sequence is a Template region. `banner` and `main` stay the shell's, and no Template renders either.

#432's drawing asked for the `<nav>` *before* `<main>`. That is not available:
[#452](https://github.com/markgoho/doula-cloud/issues/452) put `<main>` in the shell, so everything a
Template renders is already inside it. A `<nav>` inside `<main>` is valid and is a landmark either way.

This does **not** reopen the `contents` region above. That one stays a boolean deriving its list from
`sections`, because its entries are in-page anchors and "it is not a nav" has to stay enforceable. A
step rail's entries are routes, so it is the opposite case: it *is* navigation, and saying so is what
makes the step number announceable.

### Three smaller calls recorded here

- **`StepRail` is an organism, not a region on each Template.** Two identical consumers is exactly the
  extraction bar above, and #424's rule — a molecule is a part of a section, an organism is a whole one
  — puts it in the organism tier. `BackLink` is extracted as a **molecule** on the same bar: GOV.UK's
  Back link is a named component with rules of its own (top of the page, above the error summary, the
  word is *Back*), and without it those rules are copied into two stylesheets.
- **The error summary is a position, never markup.** Both Templates take `errorSummary?: Snippet` and
  render it below the back link and above the `<h1>`, which is GOV.UK's position and is page-level
  arrangement. Neither renders a `Notice` or an error box of its own; the component is
  [#467](https://github.com/markgoho/doula-cloud/issues/467)'s, and a second one built here is the
  duplication that ticket exists to remove.
- **`CheckAnswers` keeps its row markup internal rather than growing `DescriptionList`.** A
  check-answers row is a label, a value and an action, and `DescriptionList` has no action column.
  Growing a molecule for exactly one consumer is what the extraction bar exists to prevent. The row
  moves out when a second page wants it.

## Amendment, 2026-08-30 — a Template owns its own loading and load-error states

Filed as [#480](https://github.com/markgoho/doula-cloud/issues/480), found while retrofitting the two
Engagement detail pages: every retrofitted route had the same three-branch shape —
`{#if error}<Notice/>{:else if data}<Template/>{:else}<Skeleton/>{/if}` — and only the middle branch
ever reached the Template's frame. A `Notice` or a `Skeleton` rendered bare, at the viewport edge, so
the loaded state jumped into gutters and a max-width that its own placeholder never reserved.

**"What a Template owns" now covers the states a page is in before it has content, not only laid-out
content.** `OverviewHub`, `RecordDetail` and `FormPage` each gain two optional props:

- **`loading?: string`** — presence is the state, and the value is also the `Skeleton`'s accessible
  label, so a caller cannot ask for "loading" without saying what is loading (`Skeleton`'s own rule,
  extended to the prop that reaches it).
- **`loadError?: string`** — presence is the state, value is the `Notice`'s message.

Precedence is `loadError` → `loading` → normal content, matching the order every route already wrote by
hand. A route now renders its Template exactly once, unconditionally, and lets these two props carry
the state instead of branching outside it — deleting the three-way `{#if}` from every retrofitted route
rather than adding a competing frame primitive. That was the real choice this ticket carried: a route
could instead have been handed a bare, Template-free frame wrapper to put its `Notice`/`Skeleton` in,
but this map's own "Not yet specified" section already treats *"a frame lives only on a Template"* as
load-bearing — the next map's whole approach to archetypes A, C, F and G is "which Template does this
route get," not "here is a frame primitive it can reach for instead." A bare frame escape hatch would
have undercut that before the next map even starts.

This is the same kind of amendment the `isEmpty`/`empty` pair already made: a Template owning a named
*state*, not only content layout. It is not a third named exit — `loading`/`loadError` are alternate
values of the Template's own required inputs, not a new way to deviate from one.

**`loadError` is a new name, not `FormPage`'s existing `error?`.** The regions table above still lists
`FormPage` as taking `error?`; the shipped prop has always been `errorSummary?: Snippet` — GOV.UK's
validation error summary for a form the person is actively filling in, built by the route (#467). That
is a different concern from `loadError`, which is the page's own data failing to arrive before there is
a form to fail at all, and the two can be true independently (a page could, in principle, load and then
have its own submission refused). `loadError` borrows its name from the route-local variable
`account/+page.svelte` already used for exactly this state, rather than reusing or renaming
`errorSummary`.

**`RecordDetail`'s rail is not derived during loading, because it cannot be — `isContentsShown` already
does not depend on data.** Every call site sets `isContentsShown` as a static literal known at the
route's own authoring time (`isContentsShown` is unconditionally `true` on the staff Engagement page,
absent on the portal one); it has never varied by what `sections` turns out to hold. So `loading`
reuses the same prop to reserve the rail's column width in the container-query grid, filled with
nothing rather than placeholder links — an empty region carries no ARIA role, so it does not compete
with the `Skeleton`'s own `role="status"` for what gets announced.

**Fixed on six routes**: the Practice landing page and both Engagement detail pages (`OverviewHub`,
`RecordDetail` ×2 — #423, #424), the Client detail page (`RecordDetail`), and `account` and
`settings/website` (`FormPage` ×2, #474's `account` and a second, previously-unticketed instance on
`settings/website`). Two of the six — `account` and `settings/website` — had no loading branch at all
before this: `account`'s `{#if loadError}{:else if isLoaded}{/if}` and `settings/website`'s
`{#if loadError}{:else if current}{/if}` both left the gap between mount and the first response
uncovered, so nothing rendered there, not even outside the frame. `settings/website`
composes `FormPage` for only one of its three steps; the other two (`review`, `saved`) already built
`container-l`/`center-l` by hand and keep doing so — its `loadError` branch was made to match that
existing hand-built frame rather than routed through `FormPage`, since `FormPage` covers only the
`answers` step there.

`QuestionPage` and `CheckAnswers` are not touched: neither is wired into a real route yet, both are
style-guide-only per the amendment above, so there is no retrofitted `{#if error}` to find on either.
