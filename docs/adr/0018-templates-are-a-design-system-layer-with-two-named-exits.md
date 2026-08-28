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
| **D** Record detail | `templates/RecordDetail.svelte` | `title`, `summary?`, `actions?`, `sections: { heading, content }[]` |
| **E** Long form | `templates/FormPage.svelte` | `title`, `intro?`, `fieldsets: { legend, content }[]`, `error?`, `actions` |

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
Templates here** — B, D and E. A (unauthenticated entry), C (index/list), F (settings/editor) and G
(document/print) are deliberately left, as is retrofitting the remaining routes onto these three.

The Practice landing page's *content* is not decided here either. The persona and journey documents supply
it — an Offer inbox, roster health, credit and Connect state, and the empty state — but three of the
blocks they ask for cannot be served: there is no practice-wide contracts-awaiting-signature endpoint, no
unpaid-invoice roll-up, and coverage is blocked at the schema, since `00007_visit.sql` gives a Visit no
date at all. Those are product and backend work beyond this map's destination.
