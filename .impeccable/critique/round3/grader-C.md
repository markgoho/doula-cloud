# Grader C, round 3, Engagement record at 4b6f64aa

## Expected and missing (written before reading the surface brief)

From the grader brief, the rubric's jobs, and one look at Camille's record at 1280x800 (grader-C-1280.png) and its accessibility snapshot:

1. **Previous / next record without going back to the list** (Linear issue view J/K and arrows; Gmail newer/older; HubSpot record "Previous/Next" in a filtered view). Not on the screen: the only way out is "Clients".
2. **A sticky record header (name + primary action) once scrolled** (Stripe Dashboard customer page, HubSpot contact record, Linear issue header). h1 is `position: static`; on a 5109px page the name and "Message the Client" scroll away.
3. **An internal Staff note on the Client, separate from Visit notes** (HubSpot "Notes" tab, Healthie chart notes, Linear issue comments). No "Add a note" anywhere; Visit notes are the only free text Staff write.
4. **Files / attachments on the record** (HubSpot attachments, Healthie Documents, Stripe customer's invoices PDFs list). Only the signed Contract PDF; no place for a Client's uploaded doc or a hospital form.
5. **Tasks / follow-ups on the record** (HubSpot Tasks, Linear sub-issues). None; "Unanswered since" is the only to-do signal.
6. **Copy link / copy ID of the record** (Linear "Copy issue URL", Stripe "Copy ID"). Not present.
7. **Command menu / keyboard shortcuts on the record** (Linear Cmd-K, Stripe "/" search). Not present.
8. **Client's email and address at a glance beside the phone** (Stripe customer sidebar: email, billing address; HoneyBook client card). Phone is a fact; email is in the Details aside at y=809 (below the first screen at 1280x800); no address.
9. **One "More actions" overflow in the header** (Stripe customer "Actions" menu, Linear "..." menu). Actions are distributed into sections instead.
10. **Create Invoice from the record** (Stripe customer page "Create invoice"). Not drawn on Camille's because everything is billed; unverified on a record with unbilled balance.

### Against the brief's Category baseline (read after the list above)

| # | Mine | Baseline row | Verdict |
|---|---|---|---|
| 1 | Prev/next record | 17, Declined "no job on record" | Holds for now. A Linear/HubSpot reader expects it, but no walk reached for it |
| 2 | Sticky record header | **Not in the baseline.** Row 7 declines sticky in-page *navigation*, not a sticky name + primary move | Unsound to leave out. Priya (round 3, act 11/12) reached for "a short way to Add a Visit and Write a message from far down the page": Add a Visit was 951px above her, Write a message 2546px below. The page is 9640px at 375 and 5158px at 1440 |
| 3 | Internal Staff notes | 29, Build | Built: Care Plan, "Camille never sees the Care Plan" |
| 4 | Files/attachments | Not in baseline | No job on record that I found. Not a fail |
| 5 | Tasks | Not in baseline | Same |
| 6 | Copy link/ID | 15, Declined | Holds. The address bar does it |
| 7 | Shortcuts/command menu | 16, Later #2956 | Holds |
| 8 | Email/address at a glance | 27, Build | Partly built. Phone is a fact with `tel:`. Email sits in the Details aside at y=809 at 1280x800, below the first screen. No address |
| 9 | Overflow "Actions" | 28, Declined | Holds: GOV.UK has no such pattern, and it would add one action |
| 10 | Create Invoice | Not explicit | Unverified |

## Usability (/40)

| # | Score | Failed | Evidence for passes |
|---|---|---|---|
| 1 | 4 | none (1a skeleton unverified) | Per-rAF trace on reload at 1280, unthrottled: first h2 482→483px, `.facts` 381→382px (≤1px). Header shows phase, due with 37 weeks and Doulas, then the amber "Nobody on call tonight" badge, then Jump to. Fact reads "On call tonight: Nobody". BFF `/visits` `next.scheduledAt` 2026-11-19T11:00 matches "Thu Nov 19, 11:00am". `/contract/invoices` `owed` {75000, nextDueOn 2026-11-29} matches "$750.00, due Nov 29". Console: 0 errors or warnings |
| 2 | 4 | none | Find for "engagement" in body text and aria names: 0 hits. Weeks: Camille 37, Tamsin 36, Grace 28, Imani none. Mira and Imogen unverified. Activity: `<time datetime>` on every row, relative under 7 days ("24 minutes ago", "Tuesday, 8:30am"). Contract wording: "Sent, waiting for the Client to sign" (Grace), "Signed" |
| 3 | 4 | none (3a filter kept, 3d focus move, 3e aria-current unverified) | Compose Cancel closes it and returns focus to "Message the Client". Birth Plan page back link "Camille" goes to `…#birth-plan`. 5158px page with no section over 900px (max 630); "Back to top" at the page end. Back link "Clients"; top bar "Clients" underlined as current |
| 4 | 2 | **4a, 4e** | 4b: `.facts` beside the main column at 1280 and 1440 (left 1102, top 390). `@container record-body (min-width:73rem)` in RecordDetail. 4c: facts is a `Snippet` prop; EngagementScreen sets no grid. 4d: overdue red oklch(0.506 0.193 28) on Imani; amber warning; owed neutral. 4f/T8: h1 "Camille Boyd", labelled header facts, h2s in drawn order, fact links "Nobody, On call tonight, in On call", "(585) 555-0142, Phone, call", Change "on-call days for Priya Raman". 4g/T9: skip→Clients→Message→On call→Birth Plan (4 stops), 2px solid focus ring with 2px offset, compose takes focus and Cancel returns it. 320px: scrollWidth 310, nothing over the edge, no target under 24px |
| 5 | 4 | none (5c/T7 unverified on this build) | No destructive act in the header. Void is transparent/secondary after the WarningText, with a confirm step ("Void this Contract?"). Money writes are gated `isOwnerOrAdmin` (EngagementScreen.svelte:3908). ADR-0008 lets an employed Doula read money, so Priya seeing Owed is correct |
| 6 | 4 | none (Jamie unverified) | All eight facts on the 1280x800 first screen, untruncated (grader-C-1280.png). Links go to their sections; the header facts carry no link. "Message the Client" is in the header; "Add a Visit" is Visits' first control. State lines: BP "4 of 6 questions answered · last changed Nov 1, 2026 by Priya Raman · Camille has not read it yet", Contract "Signed on…", Invoices "$750.00 owed, due Nov 29.". Order checked on Camille, Grace, Imani and Tamsin; Activity last |
| 7 | 4 | none | T1–T5 pass, below |
| 8 | 4 | none (8d at 375 unverified) | 8a: On call h2 483, state line, Priya row ≈700-715 at 1280. 8b/8h at 375: header facts, badge, Jump to, facts start at 588, so three decisions. 8c: 5158 ≤ 5400. 8d: BP 527px and Care Plan 381px at 1440; "Read or print the whole Birth Plan". 8e: empty sections 149–153px (heading, line, control). 8f: Void is secondary. 8i: Jump to takes 2 lines at 1280 and 4 at 375 |
| 9 | 4 | none (9b, 9c unverified) | T10 with Invoices and Visits fetch patched to 500 on my page: "We could not load the Invoices. / Try again", the same for Visits, other sections stand, facts "Owed: Could not be read" and "Next Visit: Could not be read". Engagement read 500: "Sorry, there is a problem… Go to your Practice overview" |
| 10 | 4 | none | T6 passes. Empties say why: "Invoices start once Grace signs the Contract.", "No Birth Plan. This is postpartum care.", "The thread opens once Grace has accepted the portal invite." Void warning is in words before the confirm. No walker asked about a term |

**Total 38/40**

## Tasks

| T | Result | Actions | Evidence |
|---|---|---|---|
| T1 | pass | 1 | Priya r3 act 3: Jump to → "No epidural unless I ask twice." on screen. The link is on the first screen at 375 and at 320 (bottom 467px). Full plan 1 action away ("Read or print the whole Birth Plan"; Print and PDF on that page) |
| T2 | pass | 0 | grader-C-1280.png; Imani shows "$950.00 overdue since Nov 3"; Grace shows Sent / Nothing owed |
| T3 | pass | 2 / 3 / 2 | Priya r3 acts 6–8 and 11. In-place update is the orchestrator's evidence |
| T4 | pass | 1 | "Message the Client" → textarea "Message" focused, newest Message above it (grader-C-375-compose.png). "Unanswered since Thursday, 9:22am" is in facts before opening, though at 375 it sits at y=742, just below the first screen |
| T5 | pass | 0 / 2 | Dee r1 acts 2–5 (fact link → Record payment). Void is secondary and asks first |
| T6 | pass | 1 each | Contract line "Sent on Sep 13, 2026 by Renata Alvarez". Activity first page: "Put on this work as the Doula: Bethany Kroll — Renata Alvarez", "Gap in on-call cover added — Renata". Filters are BFF `?activity=` (Dee act 7–8 GET kind=…). Load more is present |
| T7 | unverified | — | Lena's log is round 1 (563c2f5c). Not walked on this build |
| T8 | pass | — | Snapshot at 1280 and 375 |
| T9 | pass | 4 tabs | see 4g |
| T10 | pass | — | see H9 |

## Craft (/20)

| C | Score | Failed | Evidence |
|---|---|---|---|
| C1 | 4 | none | Weeks, "Nobody on call tonight", Birth Plan answer and "cannot be reached" gaps are domain-only. No cards, eyebrows or hero metrics |
| C2 | 4 | none (same-role style across 3 screens unverified) | Hanken Grotesk, self-hosted. h1 31–600, h2 25–600, `text-wrap: balance`. `tabular-nums` on money cells and facts. Prose at 65ch (657px at 1440). No overflow at 320 |
| C3 | 3 | browser surfaces | Selection, caret and scrollbar are themed. Underline offset 3.2px. Dark contrast: badge 5.35, labels 7.33, text 13.03. **But** the compose's file input is the native unstyled "Choose File / No file chosen" (grader-C-375-compose.png) |
| C4 | 3 | alignment | Density fits Operate at 1280 and 375. **But** at 1440 the Activity table's right edge is 697 while every other row and table ends at 1060 |
| C5 | 3 | signature | DESIGN.md:171 names the activity ledger as the signature. On this page it is a plain When/What/Who table at the very end, not recognizable as DoulaCloud, and it shows two identical rows ("Doulas on call for the Client changed", same instant, same Who). One motion (value placeholder) |

**Total 17/20**

## Findings, ranked by harm to a real person's job

1. **4e: no way back to the header's moves from deep in the page.** Priya (r3) had to travel 951px up for Add a Visit and 2546px down for Write a message, and never used the header move. The page is 9640px at 375. Fix: a compact sticky record bar (name + "Message the Client") that appears only once the header scrolls out, with `scroll-padding-top` so focus is never under it. Or extend "Back to top" to every section taller than one screen *at the reader's width*, not only at 1440.
2. **Activity entries are not told apart.** "Doulas on call for the Client changed" appears twice at the same instant by the same person, and "the Client" is used in place of her name. T6's "who changed what" stays guessable, not readable. Fix: name the Doula and the days in the What ("Bethany Kroll on call Dec 6 to Dec 20"), as "Put on this work as the Doula: Bethany Kroll" already does.
3. **4a / C4: the Activity table's right edge (697) is off the shared edge (1060) at 1440.** Fix: let the ledger span the main column with What taking the slack, or have the rubric exempt a table whose max-content is under the column.
4. **Tamsin's fact reads "On call tonight: Starts Nov 20" with "No Doula yet".** It reads as if someone starts. Fix: "No Doula yet" in the fact, linking to On call.
5. **C3: native file input in the compose.** Fix: a visually styled label button plus a visually hidden `<input type=file>`, from the Button tokens.
6. **At 375 the Messages fact (y=742) and the Phone fact (799) fall below the first screen.** Priya missed the phone. Fix: order the facts so Messages and Phone come before Contract on a phone, or tighten the 8px/28px header rhythm (the button is 28px from h1 but 8px from the status rows, so it groups with the facts).

Unverified: 1a skeleton timing (orchestrator's note), 2b Mira/Imogen, 3a, 3d focus move, 3e aria-current, 5c/T7 on this build, 6d Jamie, 8d at 375, 9b, 9c, C2 cross-screen.
