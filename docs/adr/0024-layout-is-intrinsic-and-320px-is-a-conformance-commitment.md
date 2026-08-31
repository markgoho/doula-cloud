# Layout is intrinsic, and 320px is a conformance commitment

Supersedes [ADR-0023](0023-layout-is-intrinsic-and-320px-is-the-floor.md), which said this in 2,500 words and carried verification and teaching in the same document. This one holds the commitment and the mechanism, and nothing else. Verification lives in [ADR-0025](0025-layout-is-verified-across-the-continuum.md).

Every route in Doula Cloud is complete and usable from 320px of available space upwards, for Staff and Clients alike, and no screen is exempt. A component adapts to **the space it is actually given**, never to a device it assumes it is on — so it behaves correctly on a full page, in a narrow column, and one day embedded in a Practice's own website, without knowing which of those it is in. This extends [ADR-0003](0003-css-layout-primitives-as-native-custom-elements.md), which already chose intrinsic design as the mechanism; what was missing was the *expectation*, so nothing reached for it deliberately.

The vocabulary this ADR uses — **intrinsic layout**, **available space**, **content floor**, **the continuum**, **quantum layout**, and the narrowed **responsive** — is defined in `CONTEXT.md` and is not restated here. The terms it must not use are listed there too.

## 320px, and why it is not a phone number and not a content floor

320 CSS pixels is WCAG 2.2 [1.4.10 Reflow](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html)'s figure, derived from a 1280px window at 400% browser zoom. It is a **low-vision number**, not a small-phone number. That is why it binds a desktop-only screen exactly as hard as a screen someone reads on a train, and why it is not negotiable upward to 360: choosing 360 would silently drop 400% zoom, which is an accessibility regression wearing a scope reduction's clothes.

It is a **conformance commitment**, and it is deliberately not a content floor. A content floor is discovered from the content — the space below which a named thing stops fitting. 320 is discovered from a standard. No component may derive its floor from it, and a layout that changes configuration *at* 320px has read this ADR backwards: 320 is where the layout must already be correct, not a size it is built for.

`docs/design/govuk-alignment.md`'s fourth honesty rule cites this number rather than naming it independently, so the two cannot drift apart.

## What the commitment covers, and what it does not

It covers **works**: every route complete and usable, nothing unreachable, nothing lost, and the page never scrolling sideways. It does **not** cover **refined**: touch targets sized for thumbs, phone-first navigation, install prompts, camera or offline affordances. Those are interaction design rather than layout, they are a separate effort, and this ADR deliberately promises nothing about them. A reader who takes "works at 320px" to mean "designed for phones" has read it wrong.

## The mechanism, in order

1. **Reach for the query-free answer first.** `flex-wrap` with a real `flex-basis`, `repeat(auto-fit, minmax(<content floor>, 1fr))`, `clamp()`, and the intrinsic sizing keywords `min-content`, `max-content` and `fit-content` between them handle most adaptation with no query at all. This is the majority case, not the clever case, and it is the only mechanism that produces a **quantum layout** with no authored configuration in it: the browser resolves the arrangement again and again across the continuum and nobody wrote down a single one of those moments.

2. **When the query-free answer runs out, use a container query, and name the content floor that made you.** A container query asks how much room *this component* was given, so the same component is correct in a full page, a sidebar, and an embedded surface that owns no window of its own. It is the right escape hatch, and it is the only sanctioned one.

   It is not free. Every Layout — read firsthand on [#520](https://github.com/markgoho/doula-cloud/issues/520) — calls `@container` and `@media` *"circuit breakers we wire into our layouts... I'd sooner not have them anywhere I know they're not needed"* (p. 167), and shows a worked case where replicating its Sidebar with a container query needs more code rather than less (p. 168). Its objection is exact and it is right: a query knows the container's state and not the state of the elements inside it, so a threshold in a query is a number a **person** picked and re-entered by hand, where the query-free answer derives the same behaviour from the content itself.

   So the condition's literal **is a content floor and is written as one**: each `@container` rule carries a comment naming what stopped fitting at that value. That is what stops a container query hiding a device width, and it is the same duty rule 3 puts on the only media query that remains. ADR-0023 said a container query "does not owe anyone a justification". That clause is dropped.

3. **A media query is for a stated user preference, and never for space.** `prefers-color-scheme`, `prefers-reduced-motion`, `prefers-contrast`, `print`. That is the whole of rule 3, and the narrowed **responsive** in `CONTEXT.md` is exactly this set.

   ADR-0023 allowed a second case: the outermost application shell, whose box *is* the window by construction. That exception is removed. The shell is a component that currently happens to span the window, and it stops doing so the day a Practice embeds anything; it declares its own containment context and queries that, like everything else. `StaffTopBar.svelte` and `PortalTopBar.svelte` are the two files this changes, and removing the exception collapses the rule to one sentence with no allowlist behind it.

Intrinsic Web Design is not a ban on queries and this ADR is not either. Jen Simmons' own six characteristics end with *"media queries, as needed"* — kept deliberately, demoted from *the* mechanism to a finishing tool. What intrinsic sizing removes is the specific job breakpoints were invented for: rescuing percentage-based layouts from breaking. Simmons' firsthand tool order, verified on [#519](https://github.com/markgoho/doula-cloud/issues/519), matches rule 1 — but the phrase *container query* appears nowhere in her 2019 talk, so **rule 2 does not claim her as its source** and ADR-0023's citation of her there was wrong. See `docs/research/intrinsic-web-design.md`.

## Grid with named areas is the default, and it is our own bet

**Lay components out with CSS Grid by default**, and name the regions with `grid-template-areas`. Grid is what makes rule 2 cheap: a component whose regions are named can be rearranged wholesale inside `@container` — a different area string, a different track list, a region moved from beside its sibling to beneath it — with no change to the markup, no duplicated subtree, and nothing hidden with `display: none`. Flexbox stays the right tool for a genuine single row or column that simply wraps. Reach for a second markup tree only when the two layouts are genuinely different *content*, not the same content in a different arrangement.

**Nothing supports this rule and it is stated as a bet.** [#519](https://github.com/markgoho/doula-cloud/issues/519) found `grid-template-areas` in zero of eleven major design systems, GOV.UK — this repo's own reference system per [ADR-0021](0021-govuk-is-the-reference-for-service-patterns.md) — included. [#520](https://github.com/markgoho/doula-cloud/issues/520) found Every Layout never discusses it at all. Their silence is explained by years of legacy surface and browser-support floors set long ago, and this repo has neither, but it is silence and not evidence. If the bet is wrong the cost lands on this repo alone, and this paragraph is where a future reader should start.

## No named set of widths, ever

There is no `--breakpoint-*` token, no canonical set of named widths, and no rule about which values a query may use. Two reasons. The mechanical one: `var()` does not resolve inside a `@media` or `@container` *condition*, only inside declarations, so a shared width could not be consumed where it is actually needed anyway. The substantive one: **a shared width name is a breakpoint**, and naming a set is how a codebase drifts back to designing for device sizes.

A content floor is therefore derived from the content and lives where it is used, in exactly one place.

## What a component does when its container cannot hold it

**It renders a different layout, not a smaller one.** Two wrong answers are ruled out. Mangling a `<table>` with `display: block`/`flex`/`grid`/`contents` destroys table semantics in Safari and Firefox, so screen readers stop announcing headers entirely. Wrapping everything in a scrollbox treats a layout problem as a scrolling problem.

`DataTable` is the worked example and deliberately the *uncommon* one: a `<table>`'s row-and-column binding is structural rather than a sizing problem, so no track list or area string turns a table into a stack — which is why it earns a second markup tree where most components must not have one. It renders a `<table>` when its container can hold one, and one `<dl>` per record — each column's header as `<dt>`, its value as `<dd>` — when it cannot, **both generated from the same column configuration**, so no route authors the second tree by hand.

A labelled scroll region remains correct for genuinely tabular numeric data, where reading across matters more than reading a record. WCAG 1.4.10 explicitly exempts *"data tables (not individual cells)"* from its ban on two-dimensional scrolling, so this is conformant; when used it carries `tabindex="0"`, `role="region"` and an accessible name, without which a keyboard user cannot reach the scroll at all.

This departs from Adrian Roselli's user testing, which found a scroll wrapper beat a reflowed version. The departure is deliberate and unchanged by this map's research: his test case was a generic data table, and every table in this application is a list of **records with names** — a Client, an Invoice, a Staff member — which is the case a description list is good at and his was not. GOV.UK has no pattern here at all; the answer is read off the ONS Design System's table rather than invented, per [#514](https://github.com/markgoho/doula-cloud/issues/514). See `docs/research/narrow-width-tables-and-testing.md`.

## Consequences

- `CLAUDE.md`'s **Layout** expectation cites this ADR, and says the one thing [#521](https://github.com/markgoho/doula-cloud/issues/521) proved was missing: **choosing a component is a layout decision.** That session laid out a whole route without writing a line of CSS, so a rule addressed to whoever writes CSS never reached it. Whoever picks a component owns what it does at 320px with a Practice's real content in it.
- `StaffTopBar.svelte` and `PortalTopBar.svelte` both hold a width media query this ADR no longer permits, and both must move to a containment context of their own.
- Verification, the fixture rules and the source gates are [ADR-0025](0025-layout-is-verified-across-the-continuum.md)'s, so they can change without reopening the mechanism.
- Teaching is in neither document. [#519](https://github.com/markgoho/doula-cloud/issues/519) searched for evidence that any written artifact changes what an author later writes and found none anywhere, and this repo has already run that experiment: ADR-0023, a 288-line research file and a `CLAUDE.md` pointer produced roughly zero uses of the mechanisms they recommend.
