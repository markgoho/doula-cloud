# Inclusive Components stays a rank-three source, the other accessibility books stay evidence, and a control that acts at once is a switch

`CLAUDE.md` makes accessibility an expectation every feature carries: "What is built is usable by everyone who has to use it." [ADR-0021](0021-govuk-is-the-reference-for-service-patterns.md) made GOV.UK the default reference for a screen that asks for something or reports a failure, and its 2026-09-01 amendment set the order of sources to walk where GOV.UK has no pattern, with *Inclusive Components* and Adrian Roselli's writing tied at rank 3. That amendment came nine days before [ADR-0041](0041-a-book-enters-the-repo-as-evidence-and-becomes-a-rule-only-through-a-decision.md), under which a book is evidence and only the founder's recorded decision turns it into a rule. So the rank stood on a record that no adopt-or-not decision had confirmed, and two more books on the reference track, *Inclusive Design Patterns* and *Giving a Damn About Accessibility*, had a position on how a control is built and no decision at all.

On 2026-10-24 the founder worked through the theme synthesis at [`docs/research/books/themes/inclusive-components.md`](../research/books/themes/inclusive-components.md) in the inclusive components grilling session, [#1720](https://github.com/markgoho/doula-cloud/issues/1720): one round of eight questions, each with a recommended answer, and a positions record of eight positions, confirmed the same day ("confirmed"). This ADR records those decisions. Each reason below is the founder's, given in his own words where he gave them and otherwise as the recommendation he accepted on #1720; a book appears only as evidence weighed.

**One book keeps its rank 3, now confirmed. Two are declined as sources and stay as evidence. An on/off control that acts at once is announced as a switch. Three standing decisions hold, and a person who uses assistive technology is watched before launch.**

## The decision

### *Inclusive Components* stays at rank 3, now confirmed

*Inclusive Components* (Heydon Pickering, 2018; [read-back](../research/books/inclusive-components.md)) keeps its place in ADR-0021's order of sources at rank 3, tied with Adrian Roselli's writing, after GOV.UK (rank 1) and NHS.UK and ONS (rank 2). This confirms the 2026-09-01 amendment as this theme's adopt decision. As with GOV.UK, the decision is taken and never the markup. The book's browser facts are checked against today's support before they are used. Where it and GOV.UK overlap, as on tabs and notifications, GOV.UK wins at rank 1.

The founder's reason (#1720, Q1 and position 1): after the order of sources was explained, the founder chose to keep rank 3, as the fallback where GOV.UK, NHS.UK and ONS have no pattern. GOV.UK has no pattern for most of what the app builds past forms (menus, toggles, sortable tables, toggletips), and this source fills those gaps in GOV.UK's direction. It is one author's book, which keeps it below the design systems that are maintained and tested with users.

Weighed for: the native element first and ARIA only for a missing state (Toggle buttons, p. 15); focus never removed (Toggle buttons, pp. 20–21); `aria-expanded` on a disclosure's button (Menus & Menu Buttons, pp. 73–78); a `<dl>` per record for a narrow table (Data Tables, pp. 237–256); and a modal that returns focus (Modal Dialogs, pp. 267–281). The app already matches it in `MenuButton`, `Details`, `DataTable` and `Dialog`, with no book cited.

Weighed against: the evidence is the author's own testing, reported without participant counts, and one tweet carries the case against the tab pattern (read-back section 8); several browser facts it states as present, `inert` in Chrome only and `-ms-high-contrast` among them, have changed since (read-back section 7); and the author limits the book himself, "no one way" to build a component (Toggle buttons, p. 8). The second point is why its browser facts are checked before use, and the first is why it stays below rank 2.

Rejected: rank 2, beside NHS.UK and ONS, because that rank is for design systems maintained and tested with users; and declining it, because menus, toggles and sortable tables would then have Roselli's writing alone between GOV.UK's silence and original work.

### *Inclusive Design Patterns* is declined as a source and stays evidence

*Inclusive Design Patterns* (Heydon Pickering, 2016; [read-back](../research/books/inclusive-design-patterns.md)) is not a source. [ADR-0055](0055-form-design-patterns-is-a-rank-three-source-for-forms.md) declined it for forms; this ADR declines it for components too. The founder's answer (#1720, Q2 and position 2): "decline". The reason is the recommendation he accepted: where it agrees with *Inclusive Components*, the same author's later book already holds rank 3, and where it differs, the later book or GOV.UK has the newer answer. Its idea of an audit by pattern is already how this repo works, since a fix goes into the component and not into each screen.

Weighed for: HTML before ARIA (A Filter Widget, pp. 226–232); a menu button that is a disclosure (A Menu Button, pp. 169–175); an audit by pattern, not by WCAG principle (Evaluation By Pattern, pp. 109–115); and focus to the first new item after a load (A Filter Widget, pp. 239–245).

Weighed against: informal polls as its evidence (read-back section 8); vendor facts since reversed (read-back section 7); the forms decline (ADR-0055, from [#1719](https://github.com/markgoho/doula-cloud/issues/1719)); and the same author's later, fuller book, which already holds rank 3.

Rejected: rank 3 beside *Inclusive Components*.

### *Giving a Damn About Accessibility* is declined as a source and stays evidence

*Giving a Damn About Accessibility* (Sheri Byrne-Haber, 2021; [read-back](../research/books/giving-a-damn-about-accessibility.md)) is not a source. The founder's answer (#1720, Q3 and position 3): "decline". The reason is the recommendation he accepted: a rank in ADR-0021 answers what a control does, and this book never answers that, since its subject is how an organization decides. Its one process claim that the repo did not already meet, testing with disabled people, is a decision of its own, recorded below.

Weighed for: AA as the start, and AAA touch targets and contrast checks on focus indicators as the step to great (Ch. 4, pp. 41–42); testing with disabled people, because a simulated disability is "never sufficient" (Ch. 4, pp. 43–44); the design system as the place where one defect reaches every instance (Ch. 4, p. 45); and accessibility as a release gate with an exceptions process (Ch. 4, p. 46). Most of this the repo already does: axe fails the build at WCAG 2.2 AA (`docs/testing/e2e.md`), and Staff screens are held to the same 320px bar as the portal ([ADR-0024](0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md)).

Weighed against: it does not say how a control is built; and most of its figures carry no source (read-back section 8).

Rejected: rank 3, for process only (how accessibility is checked and gated) and not for components.

### The other books on the track

*Form Design Patterns* is a rank-3 source for forms by ADR-0055. Here it is evidence only, on the on/off control, focus after an action, and JavaScript, below. *Expressive Design Systems* (on the on/off control), *Refactoring UI* and *Designing User Interfaces* (on the bar), and *Going Offline* (on the on/off control) appear below as evidence; each one's standing as a source is its own theme's decision, on [#1721](https://github.com/markgoho/doula-cloud/issues/1721) (the design system), [#1722](https://github.com/markgoho/doula-cloud/issues/1722) (visual craft and type) and [#1723](https://github.com/markgoho/doula-cloud/issues/1723) (offline and resilience). The synthesis gives *Laying the Foundations*, *Flexible Typesetting*, *Design Systems* and *Atomic Design* a narrow position or none on this theme, and this ADR adds no rank for any of them.

### An on/off control that acts at once is announced as a switch

`Checkbox` with `variant="toggle"` is a native checkbox drawn as a switch. Its use in the app is the Clients list's "See everyone", which filters the list the moment it changes, and a screen reader announces it as a checkbox, which suggests data that waits for a submit. The toggle variant gets `role="switch"`, so it is announced as "switch, on" or "switch, off"; the native element keeps its focus, its Space key and its `<label>`, and its look does not change. The plain variant stays a checkbox, for data a form submits, such as the Contract agreement.

The founder's reason (#1720, Q4 and position 4): "shouldn't this be a toggle in that case?" It was explained that `role="switch"` is the toggle's role, that the control already looks like a toggle, and that the role changes only what a screen reader announces; the founder then chose `role="switch"`. The control is a toggle, so it says that it is one. It is the smallest change that makes the control say what it does.

Weighed for: *Inclusive Components* on a setting that takes effect at once (Toggle buttons, pp. 13–15, 27–33); *Form Design Patterns* on "Select all" (Ch. 5, pp. 253–255); and Atlassian's toggle in *Expressive Design Systems*, which never waits for a submit (Ch. 3, pp. 53–54). Weighed against: *Inclusive Design Patterns*' checkbox for the password reveal (A Registration Form, pp. 276–278), from a book declined above; and *Going Offline*'s checkbox named as an alternative to a button (Ch. 8, p. 110), from a book with no position on this theme.

Rejected: holding the checkbox; a `<button aria-pressed>` with a fixed label, a larger rebuild for the same announcement; and GOV.UK's filter with a submit button, an extra press for a one-click change of view, which the brief's Hick's Law and Fitts's Law rows want short. GOV.UK has no toggle component, so this is an answer where rank 1 is silent, not a departure from it. Built on [#1881](https://github.com/markgoho/doula-cloud/issues/1881).

### After an action succeeds, focus moves to the result (hold)

The brief's smoothness list holds: "submitting a form moves focus to the result", and a success message takes focus where the control that caused it is gone ([#1598](https://github.com/markgoho/doula-cloud/issues/1598)). The founder's answer (#1720, Q5 and position 5): "go with your rec". The reason is the recommendation he accepted: one predictable behavior across the app is what the brief's smoothness list asks for, and no screen in the app adds many items in a row, which is the case the opposite rule is for.

Weighed for holding: *Inclusive Design Patterns* moves focus to the first newly loaded item (A Filter Widget, pp. 240–245), and *Form Design Patterns* moves it to the first new field (Ch. 9, pp. 354–369). Weighed against: *Inclusive Components* announces an added item in a live region and leaves focus where it is, after testers thought an announcement had moved their focus (A Todo List, pp. 50–53).

Rejected: the live region as a general rule. A screen that adds many items in a row records a departure, with that reason.

### The signed-in app needs JavaScript, and the public site does not (hold)

The app renders nothing without JavaScript: `app/src/routes/+layout.ts` sets `ssr = false` so that adapter-static's `200.html` fallback can serve every route. The marketing site (`site/`) is prerendered and needs no JavaScript to read. The founder's answer (#1720, Q6 and position 6): "hold". The reason is the recommendation he accepted: the app is signed-in software for a Practice's Staff and Clients, and the public pages a stranger meets first already render without JavaScript. Server rendering would change the hosting ([ADR-0056](0056-the-hosting-rewrite-stays-and-a-google-run-caller-on-the-direct-path-is-an-accepted-residual.md)) for a failure, JavaScript that does not load, that the axe and keyboard gates do not show to be real for this audience.

Weighed for holding: *Inclusive Components*, which enhances progressively but calls a JavaScript-free build at the cost of usability "vanity" (Menus & Menu Buttons, p. 94). Weighed against: *Inclusive Design Patterns* (The Document, pp. 32–34; A Blog Post, pp. 87–88) and *Form Design Patterns* (Ch. 1, pp. 35–38), which build every pattern as HTML first.

Rejected: server rendering for the app.

### The bar is WCAG 2.2 AA, with the 44px step, and there is no contrast ceiling (hold)

The gate stays WCAG 2.2 AA, as `docs/testing/e2e.md` runs it with axe, and the brief's one step past it stays: "Hit targets never fall below 44px". The brief's Motion rule, "Nothing moves that the user did not cause", also stays as it is. No contrast ceiling is added; no token today is pure black or pure white. The founder's answer (#1720, Q7 and position 7): "wcag AA". The reason is the recommendation he accepted: a 7:1 floor would push every muted text token toward the body color and flatten the hierarchy the brief draws, and a ceiling has no measure that the arithmetic in `tokens.spec.ts` can check.

Weighed for: *Refactoring UI* (Working with Color, p. 142) and *Designing User Interfaces* (Colors, p. 98), which stop at AA. Weighed against: Byrne-Haber's AAA step (Ch. 4, pp. 41–42); *Inclusive Components*' rule that WCAG's pause control for a moving carousel is "not good enough" (A Content Slider, pp. 182–183), which the brief's Motion rule already meets; and *Inclusive Design Patterns*' tempered contrast, not stark black on white (A Paragraph, pp. 56–57), which no token breaks today.

Rejected: a 7:1 floor for text; and a ceiling rule beside the brief's floors.

### A person who uses assistive technology is watched before launch

No check the repo runs can hear what a screen reader says (`docs/testing/e2e.md`), and the five usability sessions on [#1685](https://github.com/markgoho/doula-cloud/issues/1685) are with the pilot agency's people. The founder's words (#1720, Q8 and position 8): "not that I'm aware of, we'll probably need to figure that out eventually". No one in the pilot is known to use assistive technology, so at least one session with such a person is run before launch, and how that person is found is still open: it is the first box of [#1882](https://github.com/markgoho/doula-cloud/issues/1882). The rule is the brief's Aesthetic-Usability row, where evidence comes from "watching somebody use it".

Weighed for: Byrne-Haber's "never sufficient" for a simulated disability (Ch. 4, pp. 43–44); and GDS's advice to include disabled participants, as *Inclusive Design Patterns* quotes it (A Filter Widget, pp. 238–239). Both books are declined above as sources, so both are evidence here and not the reason.

Rejected: recruiting from the pilot only, which may include no one; and leaving this testing until after launch.

### Where the other conflicts rest

The synthesis names twelve conflicts. Six are decided above: what an on/off control is (the one that changes the product), focus after an item is added, how much depends on JavaScript, how far past the minimum, how much contrast, and the evidence each book rests on (the ground of the three book decisions). The other six hold on records that predate this ADR, as the first round on #1720 sets out:

- **When `role="menu"` is right.** `MenuButton.svelte` already declines the role, because it is a promise of arrow-key behavior over buttons that Tab already reaches, and GOV.UK has no menu component. Evidence that agrees: *Inclusive Components* (Menus & Menu Buttons, pp. 65–67, 79–85) and *Inclusive Design Patterns* (A Menu Button, pp. 169–175). *Form Design Patterns*' true menu (Ch. 5, pp. 246–253) is for an overflow of actions that no screen has.
- **How tabs take focus.** GOV.UK's Tabs component (rank 1) and the alignment table's *Tabs* row, rebuilt on [#866](https://github.com/markgoho/doula-cloud/issues/866). *Inclusive Components*' tested alternative (Tabbed Interfaces, pp. 141–143) cannot outrank it.
- **Whether a message can be dismissed or leave on its own.** GOV.UK's Notification banner (rank 1) and `Notice`, which stays in the page, with #1598 for a success message. *Inclusive Components*' timer (Notifications, pp. 230–232) loses at rank 3.
- **Shadow DOM.** [ADR-0003](0003-css-layout-primitives-as-native-custom-elements.md): light DOM, no Shadow DOM.
- **A label that is not seen.** The forms decisions (ADR-0055) and the brief's Voice section keep the label visible; *Refactoring UI*'s position is the visual craft theme's (#1722).
- **Why an organization does it.** `CLAUDE.md` makes accessibility an expectation that every feature carries, with no business case asked.

## What was considered instead

**Adopt nothing.** A legitimate outcome under [#1684](https://github.com/markgoho/doula-cloud/issues/1684), and the decline option on each of #1720's book questions. Rejected for *Inclusive Components* only, for the reason under position 1; taken for the other two.

**Leave ADR-0021's amendment as it stands, with no adopt decision.** Rejected, because under ADR-0041 a rank that no decision confirmed cannot be the reason for anything, and the amendment is what an agent reads first when GOV.UK has no pattern.

**One decision for the three books.** Rejected, because the three do not stand in the same place: one fills GOV.UK's gaps on components, one is the same author's earlier and narrower statement, and one is about how an organization decides, not how a control is built.

## Consequences

- **The pattern table is `govuk-alignment.md`, and a rank-3 answer is a row in it.** ADR-0041's third reference-track stage asks for a table that an agent checks before building; for components, as for forms, that table already exists, and the `CLAUDE.md` Service patterns section already links it. A control that GOV.UK, NHS.UK and ONS have no pattern for, and that *Inclusive Components* answers, is a row in [`docs/design/govuk-alignment.md`](../design/govuk-alignment.md) beside the GOV.UK rows, with the outcome **Answered -- Inclusive Components**, the outcome ADR-0021's amendment already defines. No separate table is made: two tables would make the next builder check two places for one answer, and the rank order would then live in neither.
- **The product changes in one place.** The `Checkbox` toggle variant gets `role="switch"`: [#1881](https://github.com/markgoho/doula-cloud/issues/1881). Every other position holds a record that was already in force.
- **Other changes, each carried by its own ticket:**
  - ADR-0021's amendment and the alignment table's rank-3 line get a dated note that this ADR confirms the rank, with the two limits (the decision and never the markup; browser facts checked against today's support): [#1890](https://github.com/markgoho/doula-cloud/issues/1890). This is the same kind of change as [#1752](https://github.com/markgoho/doula-cloud/issues/1752) for ADR-0055.
  - The menu button, the app's clearest rank-3 answer, has no row in the alignment table; it gets one, and each other control that GOV.UK has no pattern for is checked for a row: [#1891](https://github.com/markgoho/doula-cloud/issues/1891). The toggle's row is #1881's.
  - At least one usability session with a person who uses assistive technology is run before launch, and the founder decides how that person is found: [#1882](https://github.com/markgoho/doula-cloud/issues/1882).
- The table rows that name a closed ticket as Open (*Exit this page*, *Skip link*, *Service navigation*, and more) are a conformance finding, not a decision, and stay with [#1703](https://github.com/markgoho/doula-cloud/issues/1703).
- `docs/manifesto.md` is unchanged, since it indexes value ADRs only, and the book table in [`docs/agents/literature.md`](../agents/literature.md) moves to **decided** in #1684's sweep after all five reference-track ADRs land.
