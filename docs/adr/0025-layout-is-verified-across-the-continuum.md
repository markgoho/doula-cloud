# Layout is verified across the continuum, not at a set of widths

The commitment and the mechanism are [ADR-0024](0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md)'s. This ADR is how that commitment is checked, and it is separate so that verification can change without reopening the mechanism — which is exactly what happened to its predecessor. [ADR-0023](0023-layout-is-intrinsic-and-320px-is-the-floor.md) argued at length that no named set of widths may exist and then specified a **rendered width matrix**: a page rendering every component at several chosen container widths, with an assertion per frame. That contradiction was found in the first hour of the map that produced this document, and it is the reason verification now lives on its own.

## Sampling is breakpoint thinking wearing a test's clothes

A set of widths in a test is the same object as a set of widths in a stylesheet: somebody picked them, they will be copied, and the layout is now correct at the sampled points and unexamined between them. The matrix is dropped. Nothing in this repo's verification names a width, except 320 — which is a conformance commitment and not a sample.

**The continuum check** replaces it: a pass asserting that nothing needs more room than it is given, at any available space, naming no width. It runs against **the drag surface**, the style-guide surface with a handle a person drags. Those two are one artifact seen two ways, defined that way in `CONTEXT.md` so they cannot drift into two implementations, and they are built by [#527](https://github.com/markgoho/doula-cloud/issues/527).

Two requirements come from the baseline capture on [#521](https://github.com/markgoho/doula-cloud/issues/521), where a real screen was swept from 1400px to 280px in 1px steps:

- **It must see a screen that contains no CSS of its own.** The one break found lived two components below the route being measured, in `DescriptionList`'s `auto 1fr` track pair. A check that only examines the file an author touched would have found nothing.
- **It must run on content a real Practice produces.** Every sweep that found nothing used content invented to be reasonable. The break needed a URL in a note to appear at all.

Sampling was honest about one thing and this document keeps it: no tool tests a continuum exactly, and a sweep is still finite. The difference is where the number comes from — a sweep's step size is a resolution, and a matrix's widths are a design.

## Fixtures

Both fixture rules were attached to the matrix. Both survive, re-attached to the continuum check.

- **Every fixture uses the longest realistic value, not a representative one** — a hyphenated double-barrelled surname, a full practice name, a four-figure amount, a URL in a free-text field. A polite fixture is a test that passes while a real Practice's screen is broken, and [#521](https://github.com/markgoho/doula-cloud/issues/521) is the evidence: the only genuine defect a full sweep found was invisible until the content was hostile.
- **A component with no fixture fails the build.** The check is generated from the component directory, so an absent fixture is not an untested component, it is a silent exemption. [#508](https://github.com/markgoho/doula-cloud/issues/508) stayed invisible because the style guide's Data table entry demoed two columns; hand-authored coverage reproduces the exact defect this is meant to close.

Hostile fixtures are also what enforces the rule a source check could not. [CSS Flexbox §4.5](https://www.w3.org/TR/css-flexbox-1/#min-size-auto) makes a flex item's automatic minimum its *min-content* size, so one long unbreakable string acts as a floor the row cannot shrink below, and the row overflows by specification rather than by bug. `min-inline-size: 0` switches that floor off. **A third source check requiring it was designed and dropped**, and it is recorded here so it is not proposed again: CSS cannot see a rule's children, so the check can only be written as *every flex or grid container must declare it*, and the application has 42 such declarations against 3 that need the fix. Thirty-nine false positives is not a gate. The rule stands and is enforced by rendering.

## The source gate

A rendering test sees how a component *behaves*; it can never see which mechanism the source reached for. That is the half this gate holds, and it is why a source rule is worth having at all: it holds at every available space precisely because it never measures one. It lives in `app/src/lib/styles/layout.usage.spec.ts`, runs from `scripts/hooks/pre-commit`, and blocks a commit rather than warning about one.

**Two rules, and deliberately only two.** Fixed pixel widths, bare `1fr` tracks and `white-space: nowrap` all have legitimate uses, so a check on them fires on correct code until somebody suppresses it, and a gate that is routinely suppressed has stopped being a gate.

1. **A media query that measures width fails, anywhere, with no exception.** ADR-0024 rule 3 leaves no legitimate one: a media query is for a stated user preference, so `prefers-color-scheme`, `prefers-reduced-motion`, `prefers-contrast` and `print` are untouched and a width condition is wrong every time it appears. ADR-0023's `SHELL_CHROME` allowlist is deleted along with the exception it encoded.

   **This rule has no `layout:ignore` escape hatch.** A marker here is a request to do the one thing the mechanism ADR forbids. If a real wall turns up, the answer is a decision recorded on the map, not a marker in a file.

2. **The static `vw` unit fails.** It measures the window including the strip the scrollbar occupies, so an element sized with it is always wider than the space actually available and the page scrolls sideways by exactly the scrollbar's width. `100%`, `100dvw` and `100svw` all mean what the author intended; `100vw` never does. This rule keeps `layout:ignore` with a reason, because a genuine case could exist.

**The gate scans every CSS-bearing file the app ships**, not only `src/lib/components` and `src/routes`. The predecessor scanned components alone, which is why the repo's last static `vw` sat unseen in `tokens.css` until it was found by hand ([#532](https://github.com/markgoho/doula-cloud/issues/532)). A gate with a blind spot the size of the token layer reports a clean repo that is not clean.

## Consequences

- The gate as it stands does not match this ADR: it carries the `SHELL_CHROME` allowlist, permits `layout:ignore` on a width query, and scans component files only. One execution ticket on [#518](https://github.com/markgoho/doula-cloud/issues/518) covers the shell's containment context, the gate rewrite and the widened scan, and absorbs [#532](https://github.com/markgoho/doula-cloud/issues/532).
- The continuum check and the drag surface do not exist yet. Until [#527](https://github.com/markgoho/doula-cloud/issues/527) lands, layout is verified by the source gate and by a person, and this ADR names the gap rather than implying coverage.
- The retrofit's stopping condition is this document: a surface is done when the continuum check passes on it with hostile content.
