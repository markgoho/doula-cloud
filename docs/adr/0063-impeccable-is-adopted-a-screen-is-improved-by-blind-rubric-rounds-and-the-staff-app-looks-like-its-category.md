# Impeccable is adopted, a screen is improved by blind rubric rounds, and the Staff app looks like its category

Status: accepted, 2026-11-08. Decided by the founder on 2026-11-08, at the end of the trial on the `experiment/impeccable` branch.

From 2026-11-07 to 2026-11-08 the founder tried [Impeccable](https://github.com/pbakaus/impeccable), a design skill for agents, on two screens: the signed-out landing, and the Staff Clients list, which took most of the trial. The skill read two context files written for it, `PRODUCT.md` and `DESIGN.md`; dealt a hand of visual directions as HTML sketches for the founder to choose from; and critiqued the screen. Partway through, the critique's single score was replaced by two written rubrics and three blind graders, because one agent grading its own work gave numbers that did not track what a person could do. Fourteen rubric rounds later the Clients list scored 40/40 on usability (median of three graders, round 14), up from the skill's first critique of 23/40, and 18/20 on craft, up from 16/20 when the craft rubric was first scored (round 2). Every round is in `.impeccable/critique/`.

The founder's words at the end: "We've already proven impeccable deserves a place in this repo." He asked that 40/40 mean a much better product, not a gamed score, and that is the reason the method below is written the way it is.

**Impeccable is adopted: the skill, its four agents and its project state. The engine is downloaded and checksum-verified, never committed. PRODUCT.md and DESIGN.md at the root are its context, and the canonical records win where they disagree. The Staff app's visual world is the category standard, at the craft of Linear, Stripe Dashboard and GOV.UK services, with at most one signature component: the activity ledger, on records. A screen is improved against a usability rubric (/40) and a craft rubric (/20), by three blind graders scored by the median per item, and a rubric changes only when a check is wrong about the job or contradicts a recorded decision. ADR-0061 is amended, not superseded: the rendered page stays the review, and a visual direction is now dealt by Impeccable as well as built as a prototype. The craft books stay evidence.**

## What is adopted

- **The skill**, `.claude/skills/impeccable/` (version 0.1.18 in `scripts/VERSION`): `SKILL.md`, its `reference/` commands, and the launcher scripts.
- **Its four agents**, `.claude/agents/impeccable-asset-producer.md`, `impeccable-documenter.md`, `impeccable-finish-reviewer.md` and `impeccable-manual-edit-applier.md`.
- **Its project state**, `.impeccable/`: `config.json` (one detector exception, for the decision-round sketches that use other faces on purpose), `design.json` (the design sidecar), the surface briefs under `surfaces/`, the direction-round sketches and payloads under `mocks/decision/`, every critique snapshot under `critique/`, and the rubrics under `rubrics/`.
- **The two context files at the root.** `PRODUCT.md` holds the product truth the skill designs to: users, purpose, positioning, operating context, brand commitments and principles. `DESIGN.md` records the built system, derived from what shipped (tokens, components, named rules). Both say where the truth is: `PRODUCT.md` that `GLOSSARY.md`, `docs/adr/`, `docs/personas/` and the brief's Voice section win where it disagrees with them; `DESIGN.md` that `tokens.css`, the components and those records win over it. Neither is a source a value is read from: a value is a token first, and `DESIGN.md` describes it.

## What is not adopted

- **The engine binary is not committed.** The launcher (`scripts/impeccable`, `scripts/impeccable.cmd`) downloads the pinned engine from the project's GitHub releases and checks it against `scripts/engine-checksums.txt`, one sha256 per platform. A download with no recorded checksum, an empty file, or no way to hash it is refused. `.gitignore` keeps the downloaded `scripts/bin/` and the per-run `.impeccable/hook.cache.json` out of the repo, and the question-session state stays local. Moving to a new engine version is a PR that changes `VERSION` and `engine-checksums.txt` together.
- **No book becomes a rule through it.** Impeccable's references (`reference/craft.md`, `craft-floor.md`, `typeset.md` and the rest) are a tool's guidance, not a source in [ADR-0021](0021-govuk-is-the-reference-for-service-patterns.md)'s order. The visual craft books stay evidence, as [ADR-0060](0060-the-visual-craft-books-stay-evidence-and-the-type-scale-is-one-ratio-from-16px.md) decided, and [ADR-0041](0041-a-book-enters-the-repo-as-evidence-and-becomes-a-rule-only-through-a-decision.md) holds: only the founder's recorded decision turns a claim into a rule. Where the skill and a recorded decision disagree, the decision wins: the 16px floor, intrinsic layout, the Voice rules and GOV.UK's patterns all held through the trial.
- **No vendored file is edited to suit the repo.** A change the repo needs goes in `PRODUCT.md`, `DESIGN.md` or `.impeccable/`, which are ours.

## The visual world

The Staff app's visual world is **the category standard, executed at the craft of Linear, Stripe Dashboard and GOV.UK services**. The founder chose it on 2026-11-08 over two hands of dealt directions (`.impeccable/mocks/decision/`, each the Clients list's first screen in one world, built from `sketch-brief.md`), for its consistency: every other world showed "glimpses of something interesting", and none held together. `PRODUCT.md`'s Brand Commitments records it.

- **Distinctiveness comes from craft and at most one signature component**, never from a costume world. The signature component is the activity ledger (`PRODUCT.md`, Positioning item 4), shown on records. A list is not a record, so the Clients list carries none. The one list-level candidate built, a due strip in the Date column, was judged by all three round-7 graders to hurt the list and was reverted (60e08e41); `DESIGN.md` records why.
- **It supersedes the brief's direction, *Plum Dusk, evolved*** ([#409](https://github.com/markgoho/doula-cloud/issues/409)), for color. The palette is now a cool blue-gray ground with one blue for links, the primary action, the current place and the focus ring (58772733, 107811ae, 1dae132a), night-blue top bars in both themes (7afedb47), and quiet filled badges with one meaning per hue (33a9a233). `tokens.css` holds the values and `DESIGN.md` names them. Type, spacing, depth, shape and the Voice section of the brief are unchanged: the 16px floor and the 1.25 ratio are ADR-0060's, and the brief's rules on one accent, no shadow and one radius still hold.
- **The lamp** (`--color-lamp`, `--color-lamp-surface`) is a warm second color on the signed-out landing only, lighting the door this device used last (ddea25ff). It is not part of the Staff working ground. It is the one place the brief's "no second accent" does not hold, and that is recorded here rather than left for a reader to find.
- **The pilot banner keeps its copy**, "Pilot" and "Tell us what you think". Its teal pill clashes with the blue world; that is left for a later round on another screen ([#2735](https://github.com/markgoho/doula-cloud/issues/2735)).

A separate ADR for the world was considered and not written: the choice was made by the same means and on the same day as the method, and it has no consequences of its own beyond the palette, which `tokens.css` and `DESIGN.md` hold.

## How a screen is improved

The method is `.impeccable/rubrics/grading-round.md`, and it is adopted as the way any screen is hill-climbed.

1. **Two rubrics.** A usability rubric for the screen, ten heuristics scored 0 to 4 (/40), each 4 earned only when every check under it passes with evidence a grader can point to, plus task checks written from the personas' jobs (`clients-list.md` is the first). A craft rubric (/20, `craft.md`) for the finish, graded against the chosen world's bar, with `reference/craft-floor.md` as its floor.
2. **Three blind graders.** Each gets the rubrics, the screen's source and its URL on the running stack (`grader-brief.md`), and none gets an earlier score, the branch's history, another grader's output or a word on what changed. The score per item is the median of the three. A check two of three fail is a finding; one grader's lone fail is a note.
3. **A fix names the check it makes pass**, and is verified the way the check is graded. A change that moves a score without changing what a person can do is reverted.
4. **A rubric changes only when a check is wrong about the job or contradicts a recorded decision**, never because it is hard to pass, and the change is dated at the rubric's foot with its reason. The trial changed three checks this way: H3 stopped asking for the Name in the address, because [#1829](https://github.com/markgoho/doula-cloud/issues/1829) forbids a Client's name in any URL (after round 5); C5 stopped requiring exactly one signature component, because `PRODUCT.md` allows at most one (after round 7); and H8's equal-rhythm check moved from record cards to table rows, because equal-height cards can only be made by drawing empty lines, which the check beside it fails (after round 13).
5. **Each round is recorded** with `impeccable critique-storage write`, with the medians, the failed checks and the SHA graded.

**What a rubric does not replace.** Its widths (1440, 1280, 768, 375px, 200% zoom) are a grader's sample of what a person sees. [ADR-0025](0025-layout-is-verified-across-the-continuum.md)'s continuum sweep stays the gate on layout, and [ADR-0024](0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md)'s 320px commitment, the 100% coverage gate, the copy specs and WCAG 2.2 AA hold as they did. A screen at 40/40 that fails one of them is not done.

### The Clients list, round by round

| Round | SHA | Usability /40 | Craft /20 |
|---|---|---|---|
| First critique (skill's own score, 2026-11-07) | — | 23 | — |
| 1 | ee8cb550 | 31 | — |
| 2 | d2e55fe4 | 34 | 16 |
| 3 | ccc0d3c6 | 30 | 16 |
| 4 | 2a3d0368 | 36 | 17 |
| 5 | 06b73fc0 | 38 | 17 |
| 6 | 09ca4c2d | 38 | 17 |
| 7 (due strip under test) | a1fde0a4 | 38 | 13 |
| 8 | 00d5e8c0 | 39 | 18 |
| 9 | a1e3ff2b | 40 | 17 |
| 10 | 80ab68eb | 39 | 19 |
| 11 | 5d60ef85 | 40 | 19 |
| 12 | eca84a52 | 39 | 18 |
| 13 | eb9bf3dd | 38 | 19 |
| 14 | 86d5a70d | 40 | 18 |

Round 1's walkers were general agents on Claude in Chrome, blind by instruction, because `persona-walker`'s browser tools were not connected. Round 3 fell because round 2's fixes broke what they did not aim at; round 7 fell on the due strip; rounds 12 and 13 each failed the screen on a check the next round's evidence corrected. Round 14's two lost craft points are on C4, mostly the two-line rows at mid widths ([#2736](https://github.com/markgoho/doula-cloud/issues/2736)).

What changed for a person, by the rubric's jobs: an Owner sees who owes money, overdue first and counted, and who has no Contract, each in two actions; any reader narrows to one Doula and to a name without losing the filter (the name never in a URL); every applied filter is a removable tag; a phone's first screen holds the list; a screen reader hears each row as name, then each line's kind, calls and facts; the keyboard reaches row 1 past the filter bar in six stops or fewer; a refused load says so and "Try again" recovers it; and a live birth reads as "3 weeks postpartum", a loss never so.

## ADR-0061 is amended, not superseded

[ADR-0061](0061-pen-dev-is-left-and-a-screen-is-designed-on-a-prototype-and-the-rendered-page.md) said a screen is designed on a throwaway prototype when a visual decision has to be made, and on the rendered page for review. The trial did both halves, by other means:

- **The rendered page stayed the review, and became stricter.** Every round was graded on the running stack, at named widths and in both themes, by graders who could not see what changed. That holds and is sharpened: the review is the rubric round.
- **A visual direction was dealt, not prototyped.** The world was chosen on Impeccable's direction round: several self-contained HTML sketches of one real screen, each in one world, rendered and compared. They are committed under `.impeccable/mocks/decision/` as the record of what was compared, the job ADR-0061 gives a prototype branch. They are spent: nobody builds from them and nobody keeps them in step with the code, so they are not the "second drawing" ADR-0061 left pen.dev to avoid. A question the direction round does not answer (a state model, one visual choice on a real screen) still goes to `/prototype` as an artifact.
- **`tokens.css` and the components stay the only source.** `DESIGN.md` describes them and is corrected when they move.
- **ADR-0061's "Design System artifact synced read-only from the code"**, not built then for want of a second reader, is in effect answered by `DESIGN.md`, which a design agent reads.

ADR-0061 carries a dated note saying so.

## What the trial changed in the shared components

The Clients list's changes reached components other screens use. Each is recorded where its rule lives:

- **`DataTable`** gained `columnWidths="fill"` with one `grows` column, `recordLayout="compact"`, `caption`, `describedBy`, `hasRowHeader` (the first column is a row header by default, on every table) and `isBusy`; the header row is tinted, rows tint on hover, every cell takes tabular figures, and the first column starts at the table's edge. The Clients list sets its own two floors (55rem, 75.2rem) on its own container, through the route-owned floor properties #1945 and #2063 already provided ([ADR-0024](0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md), dated note).
- **`FilterBar`** gained `switches`, `foldsFieldsOnly` (Clients, Invoices), `applied` (MOJ's selected-filter tags), `statusRuler`, `statusId` and a "Skip to results" link, and keeps Apply and the list in place when a filter is applied (`govuk-alignment.md`, the filter-bar row).
- **`Badge`** is quiet and filled, with one meaning per hue: amber needs someone to act (an owed Invoice, now labeled "Owed" on Staff screens), red is late or failed (an overdue or uncollectible Invoice, an expired Offer). That reaches Invoices, `InvoiceSection`, Offers and every other badge.
- **Templates**: one page title size, every Staff page at one left edge, a record's contents as one "Jump to" list ([ADR-0018](0018-templates-are-a-design-system-layer-with-two-named-exits.md), 2026-11-07 amendments).

## Consequences

- **[ADR-0058](0058-atomic-design-names-the-tiers-and-two-consumers-that-should-change-together-earn-a-variant.md)'s bar is not yet met by every new option.** "Two consumers that should change together earn a variant." `foldsFieldsOnly` and `switches` have two (Clients, Invoices). `applied` and `statusRuler` have one until Schedule and On call move to the same arrangement ([#2734](https://github.com/markgoho/doula-cloud/issues/2734)). `columnWidths="fill"`, `grows` and `recordLayout="compact"` have one, the Clients list, and no second is planned; the Clients list also overrides `fill`'s width half from its own stylesheet ([#2739](https://github.com/markgoho/doula-cloud/issues/2739)). ADR-0018's "deviation is never anonymous" is why this is written down: the three are held to the bar in the pre-launch interface inventory ([#1908](https://github.com/markgoho/doula-cloud/issues/1908)), merged back or kept with a second consumer named.
- **The brief's Color section no longer describes the build.** It carries a dated note at its top pointing here, to `tokens.css` and to `DESIGN.md`; its rules on the accent, status color and dark derivation still hold, and its hue tables are history.
- **`docs/design/workflow.md`** now sends a screen through Impeccable: context, a direction round where the look is open, and the rubric rounds.
- **A rubric is written per screen.** A screen that is hill-climbed gets its own usability rubric under `.impeccable/rubrics/`, written from the personas' jobs, before its first round; the craft rubric is shared.
- **Graders cost tokens.** A round is three graders and the walks; the trial ran fourteen on one screen. That is the price of a score that means something, and a screen is hill-climbed when it is worth it, not by default.
- **The spelling sweep does not read `.impeccable/`, `.claude/skills/` or the two root context files.** `PRODUCT.md` and `DESIGN.md` are ours and are written in American English by hand; the vendored skill is not edited.

**Amended, 2026-11-08, later the same day ([#2744](https://github.com/markgoho/doula-cloud/issues/2744)):** Impeccable is a user-level install, not committed to the repo. The founder moved the skill and its four agents out of the repo after #2740 landed them, so *What is adopted*'s first two bullets, *What is not adopted*'s first bullet and the word "vendored" above no longer describe it. In their place:

- **A user-level install, pinned to a known version, that this repo depends on.** Each machine installs it for its user before any design work: `bunx impeccable install --user`, which puts the skill under `~/.claude/skills/impeccable/` and its agents under `~/.claude/agents/`. The trial ran 0.1.18; 0.1.19 is the version installed at this amendment. The repo holds no copy of the skill, its agents or its engine, and its launcher verifies its own engine download.
- **Updating is three steps, in order:** `bunx impeccable check` says whether a newer version exists, `bunx impeccable update --user` installs it, and `/impeccable doctor` run in this repo checks the new version against `.impeccable/`, `PRODUCT.md` and `DESIGN.md`. A version `doctor` finds a problem with is not used for design work here until our files are fixed.
- **The project state stays in the repo:** `.impeccable/`, `PRODUCT.md` and `DESIGN.md`, as above. `.gitignore` keeps only `.impeccable/hook.cache.json` out. The installed skill is not edited to suit the repo; a change the repo needs goes in those files.
- The grader brief's floor is the installed skill's `reference/craft-floor.md`, and the spelling sweep reads none of `.impeccable/` or the two root context files.
