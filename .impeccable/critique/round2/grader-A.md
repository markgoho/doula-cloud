# Grader A, round 2, Engagement record (live at f4cacd20)

Signed in as Renata, own page (marked `__grader='A'`), chrome-devtools only. Budget 25 min. Screenshots: `A-1280-camille.png`, `A-375-camille.png`, `A-375-message.png`, `A-1440-dark.png`.

## Expected and missing (written before reading the surface brief)

From the rubric's jobs and one look at Camille's record (1280x800, snapshot):

1. Client phone number with tap-to-call beside the name (HubSpot and Salesforce contact records, Practice Better / Dubsado client profiles). Details shows Email only on Camille.
2. Previous / next record navigation without going back to the list (Gmail, Linear j/k).
3. Keyboard shortcuts / command palette on the record (Linear, Stripe Dashboard).
4. Copy link to this record / record ID (Linear, Stripe "copy ID").
5. Care phase changeable from the header itself (Linear status, HubSpot lifecycle stage); here it is in Details' "Change the care phase".
6. Tasks / reminders on the record (HubSpot tasks, Salesforce next steps).
7. Files / documents attached to the Client beyond the Contract PDF (HubSpot attachments, Practice Better documents).
8. Header overflow "More actions" menu (Stripe customer page, Linear issue menu).
9. A free-text internal note outside a Visit (HubSpot notes, Salesforce notes).

## 1. Usability (/40)

| # | Score | Failed checks | Evidence for passes |
|---|---|---|---|
| 1 Visibility | 4 | none (1a, 1d unverified) | 1b: header "Care phase Active / Due Dec 5, 2026, 37 weeks / Doulas Priya Raman, Bethany Kroll" under the h1, before Jump to (snapshot 59_36–42); Imani has no due line. 1c: fact "On call tonight: Nobody". 1c': amber "Nobody on call tonight" Badge in header status line; On call line in plain words. 1e: console has no errors/warnings on load. |
| 2 Real world | 4 | none (2b partly unverified: Tamsin, Mira, Imogen) | 2a: 0 matches for /engagement/i in innerText and aria-label/title. 2b: Camille 37, Grace 28, Imani none. 2c: Activity `<time datetime="2026-11-15T18:54:43-05:00">5 minutes ago`, "Tuesday, 8:30am", older "Oct 25, 2026, 10:30am". 2e: "Signed", "Sent, waiting for the Client to sign" (Grace). |
| 3 Control | 4 | none (3a, 3c unverified) | 3b: compose Cancel closed with nothing sent; Priya's Visit Cancel discarded (acts-priya act 7). 3d: 5512px at 1440, no section >900px (max On call 672, Birth Plan 675), one "Back to top" at the end to `#jump-to` (UL tabindex=-1). 3e: `aria-current=page` on top-bar "Clients"; back link "Clients". |
| 4 Consistency | 4 | none (4c unverified) | 4a at 1440: Visits table 1024 vs max-content 1208, Invoices 1024 vs 1116, Activity 661 = 661; prose/thread 647 (65ch). 4b: facts top 390 = Jump-to top 390 at 1440 (and level at 1280, screenshot), title at gutter. 4d: overdue red oklch(0.506 0.193 28) on Imani; owed-not-due neutral link; amber only on the must-act alerts. 4e: see section 5; phone row exists in source (`EngagementScreen.svelte:1318`), Camille has no phone in seed. 4f/4g pass. 320x640: scrollWidth 310 < 320, no target under 24x24. |
| 5 Error prevention | 4 | none | 5a: no destructive act in header; "Void this Contract" secondary Button after WarningText with dialog "Void this Contract?". 5b: ADR-0008:207 lets an employee Doula read money, so Priya's Owed fact is allowed; payment acts gated `isOwnerOrAdmin` (`InvoiceSection.svelte:708,740,814`). 5c: orchestrator's evidence (Lena on Morgan sees only her $450 fee, no Owed). |
| 6 Recognition | 4 | none (6d on Jamie unverified) | 6a: at 1280 all eight facts on the first screen, labeled, untruncated; links named "Nobody, On call tonight, in On call" etc.; header facts unlinked. 6b: one header move, Add a Visit first in Visits. 6c: Birth Plan "4 of 6 questions answered · last changed Nov 1, 2026 by Priya Raman · Camille has not read it yet"; Contract "Signed on Sep 20"; Invoices "$750.00 owed, due Nov 29." 6d: Camille, Grace, Imani: no empty section above a filled one, Activity last. |
| 7 Efficiency | 4 | none | T1–T5 pass (below). |
| 8 Aesthetic | 3 | 8c | 8a: On call h2 top 502, state line 553, first row bottom ~720 at 1280. 8b: 375 first screen holds header facts, badge, Birth Plan link (bottom 470), On call tonight fact (top 574). 8d: Birth Plan section 659px at 1440, 726px at 375 (≤740); Care Plan 394px capped. 8e: Offers 149px heading+line+button. 8f pass. 8g pass (on-call exemption). 8h: 3 decisions (Message, Jump to, facts). 8i: 2 lines at 1280, 4 at 375. **8c fails: scrollHeight 5512px at 1440x900 > 5400.** |
| 9 Recovery | 4 | none (9a/T10, 9b unverified) | 9c: offline "Load more" kept 60 rows and said "We could not reach the service. Check your connection and try again." with "Try again" beside it. |
| 10 Help | 4 | none | 10a: T6 pass. 10b: Grace "Invoices start once Grace signs the Contract."; Imani "No Birth Plan. This is postpartum care." 10c: warning text before confirm. 10d: no term queried in acts-priya. |

**Total 39/40.**

Unverified: 1a (dev server), 1d (network response not captured), 2b for Tamsin/Mira/Imogen, 3a, 3c, 4c, 6d on Jamie, 9a (T10), 9b.

## 2. Tasks

| Task | Result | Actions | Evidence |
|---|---|---|---|
| T1 | pass | 1 | acts-priya act 3: Jump to "Birth Plan" puts "No epidural unless I ask twice." at y=624 of 740. Link on first screen at 375 (bottom 470) and 320 (467). |
| T2 | pass | 0 | 1280 screenshot: phase, due+37 weeks, Doulas, Nobody on call, Wed Nov 18 11:00am, Signed, $750.00 due Nov 29, Unanswered since Thursday 9:22am. Imani "$950.00 overdue since Nov 3" (red). Grace "Sent, waiting for the Client to sign", "Nothing owed". |
| T3 | pass | read 2, write 3, Add 2 | Jump to Visits, scroll to Earlier; Change opens under the Visit (act 5), Notes tap = 3; "Add a Visit" first control (act 8). In-place update from orchestrator evidence. |
| T4 | pass | 1 | Live at 375: "Message the Client" → focus in TEXTAREA, Camille's "Could we move Thursday's visit…" on screen at y 243–293; unanswered fact read first. Answered-in-place from orchestrator evidence. |
| T5 | pass | 2 | Jump to Invoices → row "Record payment" (Dee's r1 log not re-read; Renata's page). Void placement 8f verified. In-place from orchestrator evidence. |
| T6 | pass | 0–1 each | Contract: "Sent on Sep 13, 2026 by Renata Alvarez" in section; Bethany: Activity "Put on this work as the Doula: Bethany Kroll / Renata Alvarez"; gap: "Gap in on-call cover added / Renata Alvarez". Filter "Contract and money" is `?kind=money` to the BFF (`activityLedger.ts:368`, `activity.go:169`); Load more appends in place. |
| T7 | pass (from evidence) | – | Orchestrator's note; r1 lena log not re-walked. |
| T8 | pass | – | h1 "Camille Boyd", header facts labeled after it; h2 per section; fact links name fact+section; Change buttons described by their row. Note: at wide widths the "Details" h2 (complementary) precedes "On call" in heading order. |
| T9 | pass | 4 stops | Skip→Clients, Message the Client, On call, Birth Plan. Focus ring 2px solid outline, offset 2px. Compose takes focus; Cancel puts focus on "Write a message" (not the header link that opened it). |
| T10 | unverified | – | Offline reload lost the dev server (chrome error page), so neither the per-section nor the whole-page refusal could be seen; source carries "Could not be read" facts (`EngagementScreen.svelte:1393–1502`) and a per-section "Try again" (`:3101`). |

## 3. Craft (/20)

| C | Score | Failed | Evidence |
|---|---|---|---|
| C1 Specificity | 4 | none | On-call gap ("cannot be reached until Mon Nov 16, 10:00am", "At a wedding four hours away"), weeks, Birth Plan read-state: unmistakably a doula practice. No refuse-list scaffolds. |
| C2 Typography | 4 | none (same-role on 3 screens unverified) | Hanken Grotesk, h1 35.6px/600; money tabular-nums in table and facts; prose 65ch. |
| C3 Color/surface | 4 | none (contrast measured by eye only) | ::selection oklch(0.376 0.042 256), caret tinted, scrollbar-color tinted, underline-offset 3.6px; dark theme coherent (A-1440-dark.png). |
| C4 Rhythm | 3 | alignment/composition | At 1440 the main content stops at ~693px and the facts column starts at 1102: a ~400px empty middle. Details in the facts column puts "Care" and "Birth" on separate lines with its "Change" alone at the far right edge, off the column's left edge (A-1440-dark.png, y≈820–895). |
| C5 Signature | 4 | none | Activity ledger is the one signature; no motion seen; nothing ornamental. |

**Total 19/20.**

## 4. Findings, ranked by harm to a real job

1. **8c, page 5512px at 1440x900 (cap 5400).** Renata scrolls past six-plus screens. Fix: collapse Offers and Birth outcome (both empty, 149/151px) into one "Not started" line group, or cut On call's prose duplication ("Nobody on call tonight: …" restates the header) — ~120px is enough.
2. **C4, Details layout in the facts column.** Label/value stacked unevenly and the Change link detached at the far edge; a reader cannot tell which row it changes at a glance. Fix: use SummaryList's row layout (key, value, action on one row) at the facts column's width, actions aligned to its own right edge, not the page's.
3. **C4, dead middle at 1440.** Fix: let the facts column sit at the main column's measure edge plus a gutter (container-driven), not pinned to the far right.
4. (not a failed check, noted) Compose Cancel returns focus to "Write a message", not to the header link that opened it; Priya's "Write a message" focuses the heading, not the textbox (acts-priya act 9). Fix: focus the textarea on open from either opener; return focus to the opener.
5. (noted) Activity "Doulas on call for the Client changed" says nothing about what changed (T6 asks "who last changed the gap"). #2945 covers part.
6. (noted) Add a Visit has only a date field; Priya reached for type, Doula and notes (act 8).

## 5. Expected and missing vs the Category baseline

| My item (source) | Baseline | Verdict |
|---|---|---|
| 1 Phone, tel: (HubSpot, Salesforce, Practice Better) | row 27 Build | built: `EngagementScreen.svelte:1318` renders a Phone row with tel: when the Client has one; Camille has none in the seed, so not seen live. |
| 2 Prev/next record (Gmail, Linear) | row 17 Declined, "no job on record; Back returns to the list" | holds weakly: a desk reader working down a list (Renata's morning) is trained by Linear/Gmail to expect it; Back to the row (#2949) is a fair substitute. |
| 3 Keyboard shortcuts (Linear, Stripe) | row 16 Later (#2956) | deferred to an app-wide decision; holds. |
| 4 Copy link/ID (Linear, Stripe) | row 15 Declined, "the address bar holds it" | holds. |
| 5 Care phase changed from the header (Linear, HubSpot) | not in the baseline (row 11 covers editing from the property) | the Change lives in Details, not on the header fact; acceptable under row 11 but not in the baseline as such. |
| 6 Tasks/reminders (HubSpot) | not in the baseline | – |
| 7 Attached files (HubSpot, Practice Better) | not in the baseline | – |
| 8 Overflow actions menu (Stripe, Linear) | row 28 Declined, "a menu puts each act behind a disclosure, two actions where there is one" | holds. |
| 9 Internal notes (HubSpot, Salesforce) | row 29 Build | built: Care Plan, state line "Camille never sees the Care Plan". |
