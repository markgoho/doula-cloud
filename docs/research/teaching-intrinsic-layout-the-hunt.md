# The teaching hunt: does anything actually teach intrinsic layout?

Research for [#526](https://github.com/markgoho/doula-cloud/issues/526), part of the [#518](https://github.com/markgoho/doula-cloud/issues/518) map. #519 surveyed three named sources and found eight of eleven design systems with zero container queries in source, and no measured evidence anywhere that any teaching format changes what an author writes. #520 read Every Layout firsthand and found a reference organised by primitive, not a curriculum. This ticket is a hunt: find the best teaching artifact for this material, wherever it lives, and judge it honestly against what this repo already has.

**A framing note on #519's design-system finding, from the repo owner:** the earlier zero-container-query result is not evidence the approach is unwise. Large systems carry legacy surface and no incentive to re-platform; their absence means there is no reference implementation to copy, not that the idea is premature. This research went looking specifically for systems that *have* adopted it, smaller and newer especially.

**Verification method.** Where a source's actual mechanism (interactive widget, exercise structure) is the finding being made, this research checked the raw source directly — `curl` plus `grep` against the fetched HTML, or the GitHub API against a course's own repository — rather than trusting a fetch tool's summarizing model. Those raw checks are cited inline as "verified firsthand" below, distinct from claims sourced only from a search snippet or a summarized fetch.

## The leads, judged on method

### Ahmad Shadeed — the strongest mechanism found

[An Interactive Guide to CSS Container Queries](https://ishadeed.com/article/css-container-query-guide/) was fetched and its raw HTML inspected directly. It contains **26 `<input type="range">` sliders** and `react-resizable` drag-handle components distributed through the article — confirmed by grepping the fetched HTML, not taken from a summary. The reader drags a container's edge or a slider and watches a live component (a card, a nav, a dashboard widget) re-lay-out in real time, no page reload, no separate demo page.

Structurally the article is a **wrong-then-right comparison**: it opens by building a card component with `@media`, shows where that breaks when the same card is dropped into a narrower sidebar (the exact "same component, different context" failure this repo's ADR-0023 names), then rebuilds it with `@container` and lets the reader resize both side by side. A dedicated "Common pitfalls" section states rules in checkable form rather than prose caveats — "A container can't be sized by its contents" and "It's not possible to query a container against itself" are the same circularity constraint this repo's own `intrinsic-web-design.md` §5 sources from MDN and the WICG wiki, arrived at independently by Shadeed and stated as a rule a reader can test against their own code. Fifteen-plus worked use cases follow (news cards, GitHub-style headers, dashboard widgets, a social feed), each with its own live, resizable demo.

**Judgment: this is a teaching mechanism, not just a well-written explanation.** The reader drives the demo; the failure mode and the fix are shown in the same interaction, at the reader's own pace and inputs, not the author's chosen breakpoints.

### Josh Comeau — a different mechanism, aimed at the wrong material

[An Interactive Guide to CSS Grid](https://www.joshwcomeau.com/css/interactive-guide-to-grid/) was fetched and checked the same way. It has no range-slider widgets, but nine `<textarea>` elements inside repeated "Code Playground" blocks (confirmed by grep) — live, editable CSS/HTML sandboxes embedded in the prose. The reader edits the actual grid-template-columns value, or the item's placement, and sees the rendered layout update immediately, rather than reading about the effect.

This is a genuine mechanism, and matches what #519 was missing: something demonstrated to make a reader *do* something rather than only read. But it teaches **CSS Grid and Flexbox as layout systems** — the substrate ADR-0023 already assumes — not container queries or intrinsic sizing specifically. [An Interactive Guide to Flexbox](https://www.joshwcomeau.com/css/interactive-guide-to-flexbox/) is the same format, same scope limit. Neither guide was found to mention container queries or intrinsic/space-based layout as its own topic.

**Judgment: strong mechanism, wrong altitude.** It is the best evidence found that live-editable playgrounds work as a mechanism; it is not itself a candidate for teaching this repo's actual subject matter.

### Andy Bell — a frame, and a paid worked-example course

The 2022 talk "[Be the browser's mentor, not its micromanager](https://buildexcellentwebsit.es/)" gives the philosophy its own name, and its companion site states the principle directly: *"Give the browser some solid rules and hints, then let it make the right decisions for the people that visit it."* This is closer to a teaching **frame** — a memorable name for the whole idea — than a mechanism: no exercises, no interactive demo, no checkable rule. It points outward to Every Layout and Utopia rather than teaching container queries or intrinsic sizing on the page itself.

Bell's paid [Complete CSS](https://piccalil.li/course/complete-css/) course (via [CSS-Tricks' coverage](https://css-tricks.com/complete-css-course/)) builds a full banking dashboard from bare HTML using CUBE CSS, and covers container queries along the way — a worked-example, project-build structure similar in kind to Wes Bos's courses below. It sits behind a paywall, so its exercise mechanics (starter files, checkable solutions, or pure video) could not be verified firsthand in this session.

**Judgment: the "mentor, not micromanager" line is a genuinely sticky frame worth borrowing, but it is not itself a teaching mechanism.** The course is a plausible mechanism, unverified.

### Wes Bos & Scott Tolinski / Syntax — exercise-driven, but predates the actual subject

Two structural claims were verified directly against Wes Bos's own GitHub repositories via the GitHub API, not a course-marketing page:

- `wesbos/css-grid` (backing [cssgrid.io](https://cssgrid.io/)) has 25 numbered lesson folders, e.g. `13 - Using minmax() for Responsive Grids`, and that folder contains exactly two files: `minmax-START.html` and `minmax-FINISHED.html`.
- `wesbos/What-The-Flexbox` (backing [flexbox.io](https://flexbox.io/)) has the same per-topic folder structure (`flex-wrapping-and-columns`, `mobile-reordering`, etc.).

This is a real, different mechanism from everything above: the learner opens a START file with no solution, has to produce working CSS against a stated task (delivered by video, not in-file comments — checked and confirmed absent from the raw `minmax-START.html`), and only then opens FINISHED to check the answer. It is the "exercise with a checkable answer" the ticket names explicitly, and it is a mechanism this repo could adopt for its own onboarding, independent of whether Bos's specific CSS content is what's needed (see the ticketable mechanism below).

Content coverage does not hold up, though. Neither course's landing page mentions container queries or intrinsic layout, and `cssgrid.io`'s own page was checked directly with no such reference found. These are Grid- and Flexbox-fundamentals courses, older than the material this hunt is chasing.

Syntax was checked at the level the coordinator asked for: raw transcripts, not summaries. Both were `curl`'d and grepped directly for `breakpoint`, `intrinsic`, `Jen Simmons`, and `every layout`:

- **[Syntax #566, "Container Queries Explained"](https://syntax.fm/show/566/container-queries-explained/transcript)** — zero occurrences of "breakpoint" anywhere in the transcript. Zero occurrences of "intrinsic," "Jen Simmons," or "Every Layout." The episode is a syntax-level walkthrough (container types, container names, units, style queries) with a timestamped show-notes structure, not a discussion of why a team habitually reaches for breakpoints.
- **[Syntax #725, "Safari is the new Chrome — Jen Simmons of Apple"](https://syntax.fm/show/725/safari-is-the-new-chrome-jen-simmons-of-apple/transcript)** — an interview with Simmons herself, but about Safari's release cadence, wide-gamut color, and CSS Masonry, not intrinsic layout or breakpoint habits. Zero occurrences of "breakpoint" or "intrinsic." Container queries surface once, as a single aside from Wes Bos about the difficulty of a container whose size depends on contents that themselves depend on the container's size — the same circularity constraint named above, but as a passing remark, not an explanation.

**Judgment: a named lead that delivers a real mechanism (START/FINISHED exercise pairs) but not on this material.** Syntax specifically does not contain the "why do teams keep reaching for breakpoints" conversation the coordinator was hoping to find — this is a plain negative result, checked against transcripts rather than assumed.

### Kevin Powell

Judged from catalogue and video-description evidence only (his site, [kevinpowell.co](https://www.kevinpowell.co/courses/), and course listings on Class Central) — no page was found offering an interactive, reader-driven artifact or a checkable exercise distinct from watching a video. He has individual videos on container queries and style queries, and is credited across multiple sources with high volume and clarity of explanation. **Judgment: volume and clarity, no mechanism found beyond video tutorial.** This is weaker evidence than the firsthand checks above and is stated as such.

### Miriam Suzanne

[Container Queries Explainer & Proposal](https://www.miriamsuzanne.com/2021/05/02/container-queries/) is spec-adjacent writing by one of the feature's co-authors, aimed at implementers and early adopters rather than learners — it references the CSS Working Group's approval process and links out to MDN and CodePen demos rather than building its own worked example. **Judgment: valuable as primary-source reasoning about why the feature is shaped the way it is (the containment/circularity requirement), not as a teaching artifact.**

### Rachel Andrew, web.dev/learn/css, and MDN — the institutional baseline

Rachel Andrew's [auto-fill vs auto-fill/minmax](https://rachelandrew.co.uk/archives/2016/04/12/flexible-sized-grids-with-auto-fill-and-minmax/) writing (already the primary source for §4 of this repo's `intrinsic-web-design.md`) is prose-and-diagram explanation, no interactive or checkable component.

[web.dev/learn/css](https://web.dev/learn/css) does carry a real mechanism: a course-ending **quiz with a badge** — *"Take a quiz, earn a badge. Correctly answer CSS questions to earn your Learn CSS badge."* That is a checkable-answer mechanism in the ticket's own sense. Its curriculum does include a dedicated container-queries module, but no explicit module on intrinsic sizing keywords.

[MDN's Learn web development: CSS layout](https://developer.mozilla.org/en-US/docs/Learn_web_development/Core/CSS_layout) module has "Test Your Skills" checkable exercises embedded between lessons and end-of-module challenges — also a genuine mechanism. But its content coverage has a real gap: this session found **no mention of container queries or intrinsic sizing anywhere in this module** — it stops at Flexbox, Grid, and classic media-query responsive design.

**Judgment: both institutional sources have real checkable-exercise mechanisms, and neither has caught up to container queries and intrinsic sizing as first-class curriculum content.** They are the baseline the rest of the field should be compared to, and the rest of the field's advantage over them is topical currency, not method.

## Sources found beyond the named leads

### Frontend Masters (Master.dev): "Building a UI Without Breakpoints"

[This April 2026 article by Amit Sheen](https://blog.master.dev/building-a-ui-without-breakpoints/) is the closest thing found anywhere in this hunt to writing specifically about **why teams keep reaching for breakpoints**, which #519 and the coordinator's brief both asked for directly. It states the habit's appeal before arguing against it: *"The model was simple... That simplicity is exactly why breakpoints became standard."* It contrasts breakpoint-driven CSS against container-query and intrinsic alternatives with before/after code, links out to CodePen demos, and states its target mechanism plainly: *"Instead of saying, 'at width X, force N columns,' define constraints and let the browser derive the layout continuously."* It does not cite Jen Simmons or Every Layout. Its mechanism is prose plus external interactive links, not an embedded interactive demo of its own, and it carries no measured evidence of changing behavior — consistent with #519's finding, not an exception to it.

### Utopia.fyi — a calculator, not an article

[Utopia](https://utopia.fyi/) is an interactive **calculator**, not a lesson: the reader inputs a minimum and maximum viewport and a type/space scale, and the tool outputs `clamp()`-based CSS values live, with a visual graph of the interpolation. The reader does not read a rule and apply it by hand; the tool computes the checkable output for the reader's own numbers. This is the same class of mechanism as Shadeed's resizable demos — reader-driven, immediate feedback — applied to fluid type/space scales rather than container queries.

### Design systems: extending #519's sweep, and the one positive finding worth naming

Per the coordinator's framing correction, this session searched specifically for systems that have adopted container queries, not only re-confirming the negative. `gh search code container-type` was run against Shopify Polaris, AWS Cloudscape, Radix Themes, Chakra UI, Atlassian's design system, PatternFly, Pinterest Gestalt, Skyscanner Backpack, and Zendesk Garden — all returned zero hits, extending #519's list of eight-of-eleven zeros by roughly ten more systems, large and mid-sized alike. Adobe's `spectrum-web-components` repeats the one hit #519 already found, in the same experimental `2nd-gen` package.

The genuine positive finding is one layer down from component libraries: **Tailwind CSS shipped container queries as first-class, no-plugin core API in v4.0**, verified against Tailwind's own announcement: *"We've brought container query support into core for v4.0, so you don't need the `@tailwindcss/container-queries` plugin anymore,"* listed among the release's headline features as *"first-class APIs for styling elements based on their container size, no plugins required"* ([tailwindcss.com/blog/tailwindcss-v4](https://tailwindcss.com/blog/tailwindcss-v4)). Before that, it shipped as an official first-party plugin from v3.2 (October 2022): *"Today we're releasing `@tailwindcss/container-queries` which is a new first-party plugin that adds container query support to the framework"* ([tailwindcss.com/blog/tailwindcss-v3-2](https://tailwindcss.com/blog/tailwindcss-v3-2)).

This is worth stating precisely, not overstating: the adoption happened in the **utility layer**, where any project using Tailwind gets `@container` support as a first-class primitive, not in the **component-system layer** the earlier design-system sweep checked. It is not a reference implementation of a component library built on container queries; it is evidence that the tooling underneath such a library now treats container queries as unremarkable, default infrastructure — which is itself a stronger positive signal than another system's silence.

## Teaching versus explaining: is #519's finding confirmed or overturned?

**Confirmed, with a sharper taxonomy.** No source found anywhere in this hunt — including the strongest mechanisms found, Shadeed's resizable demos and Comeau's live playgrounds — carries measured evidence (an A/B result, a before/after study, a retrospective) that its specific format changed what a reader subsequently wrote. That is #519's finding exactly, and it survives a substantially wider search.

What this hunt adds is a taxonomy of the mechanisms that exist without evidence of efficacy, so "no measured evidence" does not collapse into "nothing but prose exists":

- **Reader-driven manipulation of a live demo** — Shadeed's sliders and resizable panels; Utopia's calculator.
- **Live-editable code playground** — Comeau's in-page textareas.
- **Exercise with a separately-checkable answer** — Bos's START/FINISHED file pairs; MDN's Test Your Skills; web.dev's quiz-and-badge.
- **Named frame, no built-in mechanism** — Bell's "mentor, not micromanager."
- **Prose with before/after code, no embedded interaction** — the Master.dev article, Rachel Andrew's writing.

None of the first three categories has been shown, anywhere found in this research, to be *evidenced* as more effective than the last two — only argued to be, by the people who built them.

## The best artifact, and whether it beats what this repo has

**Best artifact: Ahmad Shadeed's [An Interactive Guide to CSS Container Queries](https://ishadeed.com/article/css-container-query-guide/).** It is the only source found in this entire hunt whose primary mechanism is squarely aimed at this repo's actual subject matter — container-driven, context-aware layout, the same idea ADR-0023 exists to state — and whose interactivity was independently verified against the raw page rather than taken on a summarizer's word.

**Is it better than ADR-0023 plus `docs/research/intrinsic-web-design.md` at making the idea stick? Split by what "stick" means, and the honest answer differs by half.**

- **As a teaching artifact for a person being onboarded to the idea: yes.** The repo's own documents are argued prose with tables and a worked component example (`DataTable`), and they say so themselves — nothing in either document lets a reader manipulate a demo and watch a container query resolve in real time. Shadeed's page has a mechanism the repo's documents deliberately don't: the reader controls the input, not the author.
- **At changing what an author actually writes, going forward: no evidence either way — for either source.** This is #519's finding, reaffirmed: no measured evidence exists that reading Shadeed's article, or reading ADR-0023, changes subsequent code. The repo's own prior state is the only *behavioral* data point available anywhere in this whole research effort (cited in #519's resolution), and it is negative for documentation of any kind: two real layout defects (#508, #510) reached production before the repo's prior 288-line doc-plus-ADR combination caught them, and both were caught by a person opening a browser by hand, not by anything a reader had internalized from prose. ADR-0023's own bet — enforcement via `layout.usage.spec.ts`'s source checks over documentation — is unrefuted by anything found in this hunt, and Shadeed's article, however good, is still something a reader has to read.

So: better at making a person understand it, on the day they read it. Not shown to be better, or worse, at making a person write correct code six weeks later — nothing found anywhere is.

## The adoptable mechanism

**Primary: a drag-to-resize container harness on the rendered width matrix, styled after Shadeed's demos.** ADR-0023's own matrix page currently samples fixed container widths; per its resolution comment on #519, the map has since retired that fixed-sample matrix specifically because "sampling a set of widths is breakpoint thinking." A continuous, reader-driven resize handle — the exact mechanism verified firsthand on Shadeed's page, `react-resizable` plus a live-updating render — is the non-sampling answer to precisely that objection: a person drags a component's frame continuously and watches it respond at every width, not a chosen set of them. This is concrete enough to be its own ticket: add a draggable resize handle to each component's frame on the style-guide/matrix surface, so a human reviewing layout work can manipulate container width directly instead of reading a fixed set of screenshots.

**Secondary: an in-repo onboarding exercise using Bos's START/FINISHED mechanism.** A small fixture component, deliberately built to fail `layout.usage.spec.ts`'s gate (a media query outside shell chrome, or a `100vw`), as a START state; a task ("make this pass the gate using rule 2 of ADR-0023"); a FINISHED file as the checkable answer. This reuses a verified, working mechanism from a named lead and applies it to this repo's own enforcement gate rather than to generic CSS — closer to what actually needs to stick here than reading either Shadeed's or this repo's own prose.

## Sources consulted

- Ahmad Shadeed, [An Interactive Guide to CSS Container Queries](https://ishadeed.com/article/css-container-query-guide/) — verified firsthand (raw HTML fetch, 26 `range` inputs + `react-resizable` grepped directly)
- Josh Comeau, [Interactive Guide to CSS Grid](https://www.joshwcomeau.com/css/interactive-guide-to-grid/) and [Interactive Guide to Flexbox](https://www.joshwcomeau.com/css/interactive-guide-to-flexbox/) — verified firsthand (9 `<textarea>` playground widgets grepped directly)
- Andy Bell, [buildexcellentwebsit.es](https://buildexcellentwebsit.es/) / ["Be the browser's mentor, not its micromanager"](https://bell.bz/be-the-browsers-mentor-not-its-micromanager/); [Complete CSS](https://piccalil.li/course/complete-css/) via [CSS-Tricks](https://css-tricks.com/complete-css-course/)
- Wes Bos, [cssgrid.io](https://cssgrid.io/) / [github.com/wesbos/css-grid](https://github.com/wesbos/css-grid) — verified firsthand via GitHub API (25 numbered lesson folders, START/FINISHED file pairs); [flexbox.io](https://flexbox.io/) / [github.com/wesbos/What-The-Flexbox](https://github.com/wesbos/What-The-Flexbox)
- Syntax, [#566 "Container Queries Explained"](https://syntax.fm/show/566/container-queries-explained/transcript) and [#725 "Safari is the new Chrome — Jen Simmons of Apple"](https://syntax.fm/show/725/safari-is-the-new-chrome-jen-simmons-of-apple/transcript) — both verified firsthand against raw transcript text (grepped for "breakpoint," "intrinsic," "Jen Simmons," "Every Layout")
- Kevin Powell, [kevinpowell.co/courses](https://www.kevinpowell.co/courses/) (catalogue-level only, not independently verified for mechanism)
- Miriam Suzanne, [Container Queries Explainer & Proposal](https://www.miriamsuzanne.com/2021/05/02/container-queries/)
- Rachel Andrew, [Flexible Sized Grids with auto-fill and minmax](https://rachelandrew.co.uk/archives/2016/04/12/flexible-sized-grids-with-auto-fill-and-minmax/) (already this repo's source in `intrinsic-web-design.md`)
- [web.dev/learn/css](https://web.dev/learn/css)
- MDN, [Learn web development: CSS layout](https://developer.mozilla.org/en-US/docs/Learn_web_development/Core/CSS_layout)
- Amit Sheen, ["Building a UI Without Breakpoints"](https://blog.master.dev/building-a-ui-without-breakpoints/), Frontend Masters/Master.dev, April 2026
- [Utopia](https://utopia.fyi/)
- Tailwind Labs, [Tailwind CSS v3.2 announcement](https://tailwindcss.com/blog/tailwindcss-v3-2) and [v4.0 announcement](https://tailwindcss.com/blog/tailwindcss-v4)
- Design-system source sweep via `gh search code container-type`: Shopify Polaris, AWS Cloudscape, Radix Themes, Chakra UI, Atlassian Design System, PatternFly, Pinterest Gestalt, Skyscanner Backpack, Zendesk Garden (all zero), Adobe `spectrum-web-components` (one hit, `2nd-gen` experimental package — same finding as #519)
