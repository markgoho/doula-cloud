# The GOV.UK Design System is our reference for service patterns, and never for markup or look

Doula Cloud has been reaching for the [GOV.UK Design System](https://design-system.service.gov.uk/) without saying so. `FormPage.svelte:13` cites it for where the `<form>` element goes. `accept-invite/+page.svelte:155` cites it for a `tabindex="-1"` heading that takes focus. [#464](https://github.com/markgoho/doula-cloud/issues/464) is built from their question-page and error-summary guidance, and `b7e8ab7` asks for a date of birth in three boxes because their Dates pattern says a memorable date is never a picker.

Four citations, no decision. This records the decision, on the wayfinder map [Holistic application design](https://github.com/markgoho/doula-cloud/issues/405).

## The decision

**When a screen asks a person for something, or tells a person something went wrong, GOV.UK is the default reference.** Their decision is taken unless there is a recorded reason not to, and the burden falls on departing rather than on adopting.

**Only the decision is taken.** Not `govuk-frontend`, not the `govuk-*` classes, not Nunjucks, not Transport, not the yellow focus bar, not the black-and-white chrome. Our components are Svelte, our layout primitives are the custom elements of [ADR-0003](0003-css-layout-primitives-as-native-custom-elements.md), our component tiers are the Atomic Design layers of [ADR-0018](0018-templates-are-a-design-system-layer-with-two-named-exits.md), and the color, type and spacing truth is `app/src/lib/styles/tokens.css`. **GOV.UK never overrules a token.**

## Why a design system built for government

Most design systems publish components. GOV.UK publishes decisions, and the research behind them. That is the part worth having, and it is the part a four-month runway cannot produce on its own.

The fit is close for three reasons. Doula Cloud is a **service**, opened to complete a task and leave, not an app to browse. It is **form-heavy and low-frequency**, so nobody builds fluency in it. And some of the people using it are **mid-crisis** -- a client filling in a birth plan after a loss, a doula doing admin between two births -- which is precisely the person their whole posture is designed for.

This does not conflict with [the design brief](../design/brief.md)'s appeal to Jakob's Law. The brief asks the product to behave the way software people already know; GOV.UK's patterns are what that looks like for the specific job of asking someone a question. The brief's own governing sentence -- *conventional in pattern and behavior, distinctive in execution* -- already draws this line. This ADR names which convention.

## What was considered instead

**Nothing, and keep citing it ad hoc.** This is the status quo, and it works only while somebody remembers. It produced four citations in four files and no shared error summary.

**Adopt `govuk-frontend` outright.** Rejected. It would replace tokens.css and the brief in one move, and the brief was chosen deliberately on [#409](https://github.com/markgoho/doula-cloud/issues/409). The look is not the part that is scarce.

**A US federal reference instead --** the U.S. Web Design System. Rejected as the primary reference: it is stronger on components and visual system, and thinner on the researched service patterns that are the reason for this decision. Nothing stops a specific question being settled against it later, with a reason.

## Consequences

- A new screen that asks for something checks [`docs/design/govuk-alignment.md`](../design/govuk-alignment.md) first. That document is the living half of this decision, and it changes every time a screen is built. This ADR should not change, which is why the table is not in it -- the same split [ADR-0019](0019-pen-dev-is-the-working-surface-and-code-is-the-truth.md) makes with the design workflow.
- **A departure needs a recorded reason, on the same commit that departs.** A departure with no reason is the only kind of GOV.UK finding this project treats as a defect. Four departures are already recorded: split names, no confirmation pages, no task list, no breadcrumbs.
- Their content rules become ours where they touch what a person reads: no "please", no "valid" or "invalid", no "required" in an error message. This sits alongside [#463](https://github.com/markgoho/doula-cloud/issues/463)'s no-pronoun rule, and neither overrules the other.
- Their UK-specific patterns are N/A with a reason on the table, so nobody re-derives it.
- We do not track their releases. The table is checked when we build a screen, not when they ship a version.

## Amendment (2026-09-01): what to do when GOV.UK has no pattern at all

The decision above assumes GOV.UK either has a pattern or is departed from with a recorded reason. It says nothing about the third case: **GOV.UK has no pattern at all.** Raised by the account owner during [#512](https://github.com/markgoho/doula-cloud/issues/512)'s grilling and confirmed on [#514](https://github.com/markgoho/doula-cloud/issues/514). [#508](https://github.com/markgoho/doula-cloud/issues/508) hit this first: GOV.UK publishes nothing about tables at narrow widths beyond a small-text class, their own backlog issue [alphagov/govuk-design-system-backlog#61](https://github.com/alphagov/govuk-design-system-backlog/issues/61) has been open since 2018 with 62 comments, and an HMRC engineer commented in October 2025 that they had to port ONS's and NHS's design-system CSS by hand for exactly this gap.

**A GOV.UK gap is not a licence to invent.** It is a prompt to walk this order until a source has an opinion:

1. **GOV.UK Design System** -- unchanged, this ADR's own default.
2. **NHS.UK Design System and ONS Design System**, tied -- both built on GOV.UK's own foundations, so they extend it rather than fight it, and both are publicly maintained and user-tested.
3. **Inclusive Components** (Heydon Pickering) and **Adrian Roselli's** writing, tied -- the accessibility-first component answer, and the primary source most others cite on tables, `display: contents`, and scroll regions.
4. **Every Layout** -- already licensed by this project and already the basis of [ADR-0003](0003-css-layout-primitives-as-native-custom-elements.md).
5. **Original work, with a recorded reason**, only once 1-4 are checked and found wanting.

No tie-break rule exists within a tier: two sources rarely disagree, and where they genuinely do, the row says so and picks one, rather than this ADR pre-solving a conflict that has not happened yet.

**Being answered this way is not a Departure.** Departure, above, is reserved for knowingly overriding a GOV.UK decision that exists. This is GOV.UK having no decision to override -- a different finding, and a defect only if the row fails to say where the answer came from. [`docs/design/govuk-alignment.md`](../design/govuk-alignment.md) records it as its own outcome, **Answered -- <source>**, alongside Aligned/Open/N/A/Departed.
