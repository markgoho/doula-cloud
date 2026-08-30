# Layout is intrinsic: a component adapts to the space it is given, and 320px is the floor

Every route in Doula Cloud is complete and usable from 320px of inline space upwards, for Staff and Clients alike, and no screen is exempt. A component adapts to **the space it is actually given**, never to a device it assumes it is on — so it behaves correctly on a full page, in a narrow column, and one day embedded in a Practice's own website, without knowing which of those it is in. This extends [ADR-0003](0003-css-layout-primitives-as-native-custom-elements.md), which already chose intrinsic design as the mechanism and named container queries as the default; what was missing was the *expectation*, so nothing reached for it deliberately and nothing checked that it worked.

## Why this had to be written down

Before this, narrow-width behaviour was assumed everywhere and stated nowhere. There were no breakpoint tokens, no narrow-viewport test, and no rule — so responsiveness was emergent: it fell out of ADR-0003's Every Layout primitives where they happened to help, and failed silently where they could not. Two defects reached [#508](https://github.com/markgoho/doula-cloud/issues/508) and [#510](https://github.com/markgoho/doula-cloud/issues/510) and neither could have been caught by anything in the repo; both were found by a person opening a browser by hand. An expectation nobody measures decays, and one nobody states was never an expectation.

## 320px, and why it is not a phone number

320 CSS pixels is WCAG 2.2 [1.4.10 Reflow](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html)'s figure, and it is derived from a 1280px viewport at 400% browser zoom. It is a **low-vision number**, not a small-phone number. That is the reason it binds a desktop-only screen exactly as hard as a screen someone reads on a train, and the reason it is not negotiable upward to 360: choosing 360 would silently drop support for 400% zoom, which is an accessibility regression wearing a scope reduction's clothes.

`docs/design/govuk-alignment.md`'s fourth honesty rule already required a row to be walked at 320px. This ADR is now the single place that number is stated; the alignment table cites it rather than naming it independently, so the two cannot drift apart.

## What the commitment covers, and what it does not

It covers **works**: every route complete and usable, nothing unreachable, nothing lost, and the page never scrolling sideways. It does **not** cover **refined**: touch targets sized for thumbs, phone-first navigation, install prompts, camera or offline affordances. Those are interaction design rather than layout, they are a separate effort, and this ADR deliberately promises nothing about them. A reader who takes "works at 320px" to mean "designed for phones" has read it wrong.

## The mechanism, in order

1. **Reach for the query-free answer first.** `flex-wrap` with a real `flex-basis`, `repeat(auto-fit, minmax(<content floor>, 1fr))`, `clamp()`, and the intrinsic sizing keywords `min-content`, `max-content` and `fit-content` between them handle most adaptation with no query at all — Jen Simmons' several *"stages of squishiness"* in a single resize. This is the majority case, not the clever case.
2. **When the layout genuinely changes, use a container query.** A container query asks how much room *this component* was given. Because it never asks about the viewport, the same component is correct in a full page, a sidebar, and an embedded surface that owns no viewport of its own. This is the mechanism for a structural change — different markup existing, not the same markup rearranged.
3. **A viewport media query is for two things only.** User-preference features (`prefers-color-scheme`, `prefers-reduced-motion`, `print`), and the outermost application shell chrome — a top bar whose box *is* the viewport by construction, where establishing a containment context purely to be able to query it buys naming consistency and no behaviour. Each such instance carries a comment saying what stopped fitting. `StaffTopBar.svelte` is the template: *"One breakpoint, and it is where six nav items plus a Practice name stop fitting rather than a device width."*

Intrinsic Web Design is not a ban on queries and this ADR is not either. Simmons' own six characteristics end with *"media queries, as needed"* — kept deliberately, demoted from *the* mechanism to a finishing tool. What intrinsic sizing removes is the specific job breakpoints were invented for: rescuing percentage-based layouts from breaking. See `docs/research/intrinsic-web-design.md`.

## No breakpoint tokens, and no named width set

There is no `--breakpoint-*` token, no canonical set of named widths, and no rule about which values a query may use. Two reasons. The mechanical one: `var()` does not resolve inside a `@media` or `@container` *condition*, only inside declarations, so a breakpoint cannot be a custom property consumed where it is actually needed. The substantive one: a shared width name **is** a breakpoint, and naming a set is how a codebase drifts back to designing for device sizes.

A threshold is therefore derived from content — the width at which a named thing stopped fitting — and lives where it is used. Whether that needs governing further is **deliberately left open**, to be settled the first time a specific threshold is genuinely unavoidable and there is a real case to reason from.

## What a component does when its container cannot hold it

**It renders a different layout, not a smaller one.** The wrong answers are well documented and both are ruled out here: mangling a `<table>` with `display: block`/`flex`/`grid`/`contents` destroys table semantics in Safari and Firefox, so screen readers stop announcing headers entirely; and wrapping everything in a scrollbox treats a layout problem as a scrolling problem.

`DataTable` is the worked example and the general shape. It renders a `<table>` when its container can hold one, and one `<dl>` per record — each column's header as `<dt>`, its value as `<dd>` — when it cannot, **both generated from the same column configuration**. No route authors a second markup tree, no CSS mangles a table, and the container query is legitimate under the rule above because the markup genuinely differs.

A labelled scroll region remains the correct answer for genuinely tabular numeric data, where reading across matters more than reading a record — WCAG 1.4.10 explicitly exempts *"data tables (not individual cells)"* from its ban on two-dimensional scrolling, so this is conformant. When it is used it carries `tabindex="0"`, `role="region"` and an accessible name, without which a keyboard user cannot reach the scroll at all.

This departs from Adrian Roselli's user testing, which found a scroll wrapper beat a reflowed version. The departure is deliberate: his test case was a generic data table, and every table in this application is a list of **records with names** — a Client, an Invoice, a Staff member — which is the case a description list is good at and his was not.

GOV.UK has no pattern here at all; their own backlog issue has been open since 2018. The answer is read off the ONS Design System's responsive table rather than invented, per [#514](https://github.com/markgoho/doula-cloud/issues/514). See `docs/research/narrow-width-tables-and-testing.md`.

## Where this is enforced

A commitment nobody measures decays, so this is a gate rather than a convention — the same shape as `tokens.usage.spec.ts` and `motion.spec.ts`, and blocking a commit rather than warning about one.

**Two source checks**, in `app/src/lib/styles/layout.usage.spec.ts`, run by `scripts/hooks/pre-commit`. Both carry a `layout:ignore` reason marker scoped to the rule block, exactly as `tokens:ignore` and `motion:ignore` already work.

- **A viewport width media query outside the shell chrome, or without a reason comment, fails.** This is the check that keeps rule 3 above from drifting back, and it is the only one that can: a rendering test sees how a component behaves, never which mechanism its source used. The marker alone is not enough — the file must also be named shell chrome in the spec, so the exception cannot widen by accident.
- **The static `vw` unit fails.** It measures the window including the strip the scrollbar occupies, so an element sized with it is always wider than the space actually available and the page scrolls sideways by exactly the scrollbar's width. `100%`, `100dvw` and `100svw` all mean what the author intended; `100vw` never does.

Only these two, because only these two are things a source check can judge without also firing on correct code. Fixed pixel widths, bare `1fr` tracks and `white-space: nowrap` all have legitimate uses, and a gate that is routinely suppressed has stopped being a gate.

**A third check was designed and dropped**, and it is worth saying why so it is not proposed again. [CSS Flexbox §4.5](https://www.w3.org/TR/css-flexbox-1/#min-size-auto) makes a flex item's automatic minimum its *min-content* size, so one long unbreakable string — a double-barrelled surname — acts as a floor the row cannot shrink below and the row overflows by specification rather than by bug. `min-inline-size: 0` switches that floor off, and requiring it looked like an obvious source rule. But CSS cannot see a rule's children: the check can only be written as *every flex or grid container must declare it*, and the application has 42 such declarations against 3 that need the fix. Thirty-nine false positives is not a gate. **The rule stands and is enforced by rendering instead**, which works only because every fixture uses hostile data — that is what makes the matrix page able to catch a case a polite fixture would hide.

**A rendered width matrix.** The rendering half is a style-guide surface that renders every component at several *container* widths side by side on one page, with a Playwright pass asserting that inside each frame nothing needs more room than the frame has — exempting only a labelled scroll region, which means the marking that earns the exemption is the marking accessibility already required. Because the widths are container widths rather than viewport widths, one ordinary desktop page holds every component at every width it cares about and the browser is never resized.

The page is **generated from the component directory**, and a component with no fixture fails the build. Hand-authoring it would reproduce the exact defect this ADR exists to close: #508 stayed invisible because the style guide's Data table entry demoed two columns. And **every fixture uses the longest realistic value, not a representative one** — a hyphenated double-barrelled surname, a full practice name, a four-figure amount. A polite fixture is a test that passes while a real Practice's screen is broken.

Sampling is honest about its own limit: no tool tests a continuum, and WCAG's own 320 is a literal point test. That is precisely why the mechanism is gated from source, which holds at every width because it never measures one.

## Consequences

- `CLAUDE.md`'s **Cross-cutting expectations** gains a fifth entry, **Layout**. It is not folded into accessibility: reflow is one consequence of intrinsic layout, not the whole of it, and a component being correct wherever it is placed is a design-system property before it is an accessibility one.
- `PortalTopBar.svelte`'s existing `@media (min-width: 48rem)` is sanctioned by rule 3 but lacks the required comment, and this ADR's own gate fails it until it has one.
- #508 and #510 are the first two consumers and implement one half each — the record-list reflow and the inline-field reflow respectively.
- The style guide's per-component entries stay as they are. The matrix page sits alongside them: the entries document a component's API, the matrix proves its layout.
