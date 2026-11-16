# Grader A: Engagement record, round 1

Live page, my own tab (marked `__grader='A'`, closed at the end), signed in as Renata. Widths 1440×900, 1280×800, 640×900 (standing in for 200% zoom at 1280), 375×740, 320×640; light and dark. Screenshots: `grader-A-1280-first.png`, `grader-A-375-first.png`, `grader-A-offline.png`.

Scoring rule I used: a lettered check that fails costs a point. A check I could not reach is "unverified". It does not count as a fail, but a heuristic holding one cannot score 4, because 4 needs evidence for every check. So it is capped at 3.

## Expected and missing (written before reading the surface brief)

1. Status as a pill beside the name (Stripe, Linear, HubSpot lifecycle stage).
2. A "More / Actions" overflow menu in the header (Stripe "⋯", Linear, HubSpot).
3. Previous and next record, without going back to the list (Linear J/K, Gmail, HubSpot).
4. Copy the record's link or ID (Stripe, Linear).
5. Internal Staff notes on the record, which the Client never sees (HubSpot Notes, Linear comments, Zendesk internal note).
6. The Client's phone near the name, tap to call (HubSpot contact sidebar, any CRM).
7. Keyboard shortcuts (Linear, Gmail).
8. A sticky header or section nav on a long record (Stripe Dashboard).
9. Files and attachments on the record (HubSpot, Linear).
10. Tasks and reminders tied to the record (HubSpot).
11. Search or filter within Activity (Stripe Events log).

## 1. Usability (/40)

| # | Score | Failed checks | Evidence for passes |
|---|---|---|---|
| 1 Visibility | 3 | a unverified (Slow 4G not run); d unverified (no network diff) | b: under the name, "Care phase Active · Due Dec 5, 2026, 37 weeks · Doulas Priya Raman, Bethany Kroll", above Jump to. c and c': the facts say "On call tonight ⚠ Nobody" and the status-line Badge says "⚠ Nobody on call tonight". e: console has only vite debug lines after a reload. |
| 2 Real world | 4 | none | a: searching body text and accessible names for "engagement" finds 0 hits; the title is "Camille Boyd — DoulaCloud". b: Camille 37 weeks, Grace 28 weeks, Imani no due line (Tamsin, Mira and Imogen not opened). c: every Activity row is a `<time datetime>` with offset -05:00; rows under 7 days read "8 minutes ago" or "Tuesday, 8:30am", older ones "Nov 1, 2026, 8:30pm". e: Grace "Sent, waiting for the Client to sign"; Camille "Signed". |
| 3 Control | 3 | **e**: no element in the top bar carries `aria-current`. The only `aria-current` on the page is the Activity filter "All". | a: from On call the back link reads "Back to On call" and goes to /on-call (Contracts and a kept filter unverified). b: the Visit's Change form → Cancel removes the form and focus returns to "Change" for that Visit. c: the Birth Plan page has "Back to Camille" → `#birth-plan`. d: page 6605px tall, no section over 900px (largest 710), "Back to top" at the end → `#jump-to`, a UL with tabindex -1. |
| 4 Consistency | 3 | **cross-cutting / standards**: the Activity filters are `<a href="#activity">` links with `aria-current="page"` used as toggles. They do not navigate, and the filter is not in the address (Dee act 7). GOV.UK filters use a query string. | a at 1440: Visits table 1024 (max-content 1213), Invoices 1024 (1116), Activity 692 (692). Every dl, p and status block is at most `--measure` (657px); only two h3 blocks are 822. b: facts beside the main column at 1280 and 1440, top 382 level with Jump to at 382; `@container record-body (min-width: 73rem)`; no `@media` in the route or RecordDetail. c: a `facts` Snippet prop, and the grid lives in RecordDetail. d: amber for nobody on call, red for overdue (Imani oklch(0.506 0.193 28)), neutral Owed Badge. e: see section 5. f and g: T8 and T9 pass. 320px and 640px: no horizontal scroll; no target under 24px at 320. Contrast: lowest is 5.2:1 light and 5.35:1 dark. |
| 5 Error prevention | 3 | **c / T7**: Lena sees Morgan's Invoice rows with their dates and states (Owed, Void). ADR-0008:344 refuses a contractor "Invoice and payment history" ("✗ her own agreed fee only"), and Lena read them as "Rooted's business". The second clause of T7 (an Engagement she is not attached to) is unverified. | a: nothing destructive in the header. "Void this Contract" asks first (a "Void this Contract?" dialog is in the DOM) after a WarningText that says what it does. b: Priya's money reads are allowed under ADR-0008:207 (amended #282), so showing Owed to her is right. |
| 6 Recognition | 4 | none (see the note on 6d) | a: all eight T2 facts are on the first screen at 1280×800 (facts bottom 714), each labeled and untruncated. Link names are "Nobody, On call tonight, in On call", "…Next Visit, in Visits", "Signed, Contract, in Contract", "$750.00, due Nov 29, Owed, in Invoices", "Unanswered since Thursday, 9:22am, Messages, in Messages". b: "Message the Client" and "Add a Visit" in the header. c: Birth Plan "4 of 6 questions answered · last changed Nov 1, 2026 by Priya Raman · Camille has not read it yet"; Contract "Signed on Sep 20, 2026"; Invoices "$750.00 owed, due Nov 29." d: Grace's order is Contract, Visits, On call, then the empty sections. Imani's starts Visits, Invoices, Birth outcome, Contract. Note: Details and Activity, always filled, sit below empty sections on every record. A strict reading of 6d fails that; I judged it the intended tail. |
| 7 Efficiency | 3 | Each job's "saving updates in place" clause is unverified, because the brief forbids saving. | T1–T5 pass their action budgets (section 2). |
| 8 Aesthetic | 1 | **a**: at 1280×800 the first section's first summary row, the gap "Priya Raman / Sun Nov 15… / Nobody covering / At a wedding four hours away", runs to about 807–872px and is cut at the fold. **c**: Camille at 1440×900 is 6605px, 7.3 screens (limit 3600px). **e**: Grace's empty Invoices section is a heading plus 4 lines ("Nothing owed.", the price line, "No Invoices yet.", "Invoicing is unavailable until…"). **h**: the first screen at 375×740 holds 4 decisions: Message the Client, Add a Visit, Jump to, and the facts' links. | b: the 375 shot shows the header facts, the on-call Badge, the Birth Plan link and the facts. d: Birth Plan section 675px at 1440; source cap 32rem on a phone; Care Plan 427px. f: Void is a transparent secondary Button under the last h3, after the WarningText. g: no header fact is repeated in the facts. i: Jump to is 2 lines at 1280 and 4 at 375, and every link targets a section on the page. |
| 9 Error recovery | 3 | a: source only (`loadFailure` snippet, "We could not load {what}." with "Try again"; facts read "Could not be read", EngagementScreen.svelte:1344–1439), not walked live. b and c unverified. Going offline on the dev server blanked the whole tab, so it could not stand in for a refusal. | — |
| 10 Help | 4 | none | a: T6 passes. b: Grace "Invoicing is unavailable until the Client signs this Contract."; "The Birth Plan is not started yet. Start the Birth Plan"; Imani "No Birth Plan. This is postpartum care." c: the void warning text. d: no walker asked what a term meant. |
| **Total** | **31/40** | | |

## 2. Tasks

| Task | Result | Actions | Evidence |
|---|---|---|---|
| T1 | Pass | 1 | Priya act 3: the "No epidural unless I ask twice." heading at 476 of 740, wholly on screen. Birth Plan link bottom at 519 at 320×640 and on the 375 first screen; the full plan with Print and PDF is 1 action away. |
| T2 | Pass | 0 | 1280 shot: all eight facts. Imani "$950.00 overdue since Nov 3" in red. Grace "Sent, waiting for the Client to sign", "Nothing owed". |
| T3 | Pass on budget; save unverified | read notes 2 (Jump to + scroll), write 3, add form 2 | Change opens under the Nov 1 row (form top 419 > row bottom 386) with focus on its h3. "Add a Visit" focuses the form's h3; its first field is the next Tab. Priya: the add form asks no notes or type. |
| T4 | Pass on budget; send unverified | 2 | The "Unanswered since Thursday, 9:22am" fact → Messages, "Latest: …" → "Write a message" opens the compose (Priya act 9). |
| T5 | Pass on budget; save unverified | 2 | Dee acts 3–4. Void asks first. |
| T6 | Pass | 1 each | Contract: "Sent on Sep 13, 2026 by Renata Alvarez". Activity "Doulas and on call" gives "Put on this work as the Doula: Bethany Kroll — Renata Alvarez" and "Gap in on-call cover added — Renata Alvarez". The filters fetch `?kind=` from the BFF (Dee: GET activity?kind=money 200). When, What, Who on every row. |
| T7 | Fail | — | Fee and terms are shown. Invoice states are shown to a contractor, against ADR-0008:344. The second clause is unverified. |
| T8 | Pass (1280; 375 unverified) | — | h1 "Camille Boyd", then the labeled facts; h2s in drawn order; each Change described by its row ("Priya Raman, Sun Nov 1, 3:00pm"); states are words. |
| T9 | Pass | 5 stops: skip → Clients → Message → Add a Visit → On call → Birth Plan | Focus 2px solid oklch(0.4 0.11 258), offset 2px. Forms take focus and Cancel returns it. |
| T10 | Unverified | — | Source supports it. Live offline blanked the dev tab. |

## 3. Craft (/20)

| C | Score | Failed checks | Evidence |
|---|---|---|---|
| C1 | 4 | — | Built for this domain: "When a Doula cannot be reached", "Nobody covering", "37 weeks", Birth Plan read state. No hero metrics, cards or eyebrows; the facts column is a plain dl. |
| C2 | 3 | One face (Hanken Grotesk) for display and body. A neutral grotesk with no display voice; it reads as a platform sans. | Tabular numbers (`tabular-nums` on td and dd); prose at 657px = 65ch; nothing overflows at 320. |
| C3 | 4 | — | Selection oklch(0.82 0.03 256), caret tinted, scrollbar oklch(0.57 0.03 258), underline offset 3.58px, focus ring in brand blue. Contrast at least 5.2:1 light and 5.35:1 dark. One meaning per hue holds. |
| C4 | 3 | In On call, the gap row starts about 66px under its h3 ("When a Doula cannot be reached" at 650, row at 716). The space looks larger than the group gaps elsewhere. Back links are worded three ways: "Clients", "Back to On call", "Back to Camille". | Facts top level with Jump to (382 = 382); the first screen carries the job at 1280 and 375. |
| C5 | 3 | Three amber "nobody on call" marks on one first screen: the header Badge, the facts' ⚠ Nobody, and the section line. The warning becomes ornament that competes with the rest. | The Activity ledger is quiet; there are no scattered effects. |
| **Total** | **17/20** | | |

## 4. Findings, ranked by harm to a person's job

1. **The page is 7.3 screens (8c).** Camille at 1440 is 6605px. Priya and Renata scroll past Offers, Birth outcome and Details to reach Activity. Fix: collapse empty sections into one "Not started yet: Offers, Birth outcome…" line, move Details into the facts column, and show Visits "Earlier" as 1 row plus "Show all".
2. **Lena sees Morgan's Invoice states (T7, ADR-0008:344).** Fix: the BFF drops Invoice rows for a contractor, and the section shows her own fee's paid state (#2271) instead.
3. **The first section's first row is cut at the fold at 1280 (8a).** Fix: put the gap row first in On call, above the "On call from … to …" line, or tighten the On call section's h3-to-row gap (about 66px).
4. **4 decisions on the phone's first screen (8h).** Fix: make the facts plain text on a phone, so the facts group is not a 4th link group, or fold the header's two moves into one group.
5. **Grace's empty Invoices is 4 lines (8e).** Fix: one line, "Nothing to invoice until Grace signs the Contract." Drop the $0.00 billed line and "No Invoices yet".
6. **No `aria-current` in the top bar (3e).** Fix: mark Clients, On call or Contracts current according to the list the back link names.
7. **Activity filters are fake links with `aria-current="page"` (4).** Fix: real `?kind=` links (GOV.UK filter) so the filter survives in the address and Back, or `aria-pressed` buttons.
8. **Three amber "nobody on call" marks on one screen (C5).** Fix: keep the header Badge and the facts row, and make the section line plain text.
9. **The back link is worded three ways (C4).** Fix: one form, "Back to X" or "X", on every screen.
10. **Hanken alone, with no display voice (C2).** Fix: a decision for DESIGN.md, not this screen.
11. **Unverified here:** 1a (skeletons on Slow 4G), 1d (facts against the network response), T10 live, and 9b/9c. Run them in a production build, where the service worker exists.

## 5. Expected and missing, against the Category baseline

| My expectation | Baseline |
|---|---|
| 1 Status pill by the name | Built (row 1) as labeled text. Sound. |
| 2 Overflow "Actions" menu | Not in the baseline. Row 2 builds header moves and row 26 places destructive acts last in their section. That covers the purpose, but a Stripe-trained reader will look for "⋯". |
| 3 Previous and next record | Declined (row 17: "no job on record; Back returns to the list"). Holds weakly: Renata at a desk reading 14 Doulas' Clients is the Linear/HubSpot reader this trains. |
| 4 Copy link or ID | Declined (row 15: "the address bar holds it"). Holds. |
| 5 Internal Staff notes | **Not in the baseline.** HubSpot and Linear readers expect a Staff-only note (Visit notes are per Visit). I name it missing. |
| 6 Phone near the name | Built (row 27). The phone is in Details at the bottom (about 5260px down), not near the name. Camille has no phone in the seed, so it is unseen. Priya's corridor call is far from it. |
| 7 Keyboard shortcuts | Later (#2956). Holds. |
| 8 Sticky nav | Declined (row 7, GOV.UK sticky-element test: zoom faults, hidden focus). Holds. Back to top is built. |
| 9 Files and attachments | Not in the baseline. Messages carry attachments; no record-level files. |
| 10 Tasks and reminders | Not in the baseline. |
| 11 Activity search or filter | Built (row 9b, by kind from the BFF). |

4e from me: one missing convention (internal Staff notes). It fails only if a second grader names it.
