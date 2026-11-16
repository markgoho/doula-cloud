# Grader C, round 1, Engagement record

Live on my own page (marked `__grader='C'`), Renata's session, dev origin :5473. Act logs (Priya, Dee, Lena) used for T1, T3–T5, T7. Screenshots: `grader-C-1280-first.png`, `grader-C-375-first.png`, `grader-C-1280-dark.png`, `grader-C-loadmore-offline.png` (the offline Engagement-read shot timed out; text captured instead). Unreached checks are "unverified", not fails.

## Expected and missing (written before reading the surface brief)

1. Client's phone, tap-to-call, on the record (HubSpot/Salesforce contact). Camille's Details: email only.
2. Sticky/condensed header with name and main moves on a 6600px page (Linear issue, Stripe customer).
3. Overflow "Actions"/"More" menu grouping the record's moves (Stripe customer "Actions").
4. "Create invoice" from the record (Stripe customer). Not testable on Camille (all billed).
5. Previous/next record (Linear j/k, Gmail older/newer).
6. Internal note/comment on the Activity timeline (Linear comments, HubSpot timeline notes).
7. Attachments/files on the record (HubSpot, Linear).
8. Copy link to the record (Linear Cmd+L, Stripe copy ID).
9. Breadcrumb trail (Stripe, many GOV.UK services).

## 1. Usability (/40)

| # | Score | Failed | Evidence for passes |
|---|---|---|---|
| 1 | 4 | none (a, d unverified) | b: under h1 "Care phase Active · Due Dec 5, 2026, 37 weeks · Doulas Priya Raman, Bethany Kroll", above Jump to; Imani no due line. c/c': facts "On call tonight ⚠ Nobody" + header Badge "Nobody on call tonight". e: console has no errors/warnings on load. |
| 2 | 3 | cross-cutting Voice: Activity "Load more" refused shows raw "Failed to fetch" | a: 0 matches for /engagement/i in innerText and aria-labels. b: Camille 37, Grace 28 weeks; Imani none (Tamsin/Mira/Imogen unverified). c: "1 minute ago", "Tuesday, 8:30am", "Nov 1, 2026, 8:30pm"; 8/8 rows with `<time datetime>`. e: "Signed", Grace "Sent, waiting for the Client to sign". |
| 3 | 3 | e: no element in the top bar carries `aria-current` (only Activity's "All" does) on arrival from Clients or Contracts | a: back link "Clients" from Clients, "Back to Contracts" from Contracts (filter kept unverified). b: Add a Visit and Visit Change both Cancel cleanly, focus returns to the opener. c: Birth Plan page "Back to Camille" → `#birth-plan`. d: page 6605px at 1440; no section >900px (max 710); "Back to top" at the end → `#jump-to`. |
| 4 | 4 | none (4d overdue red hue unverified) | a at 1440: Visits table 1024 ≤ max-content 1213, Invoices 1024 ≤ 1116, Activity 692 = 692; thread 657, Notices 657 (= 65ch). b: facts column top 382 = Jump to top 382 at 1280 (x 975), 390 at 1440; `@container record-body (min-width:73rem)` in RecordDetail, no @media. c: `facts?: Snippet` in RecordDetail, EngagementScreen sets no grid. d: Owed Badge neutral, nobody-on-call amber. f T8, g T9 pass (below). |
| 5 | 4 | none (T7, 5a asks-first unverified) | a: header holds Message, Add a Visit only. b: ADR-0008 gives an employed Doula the Contract's money, so Priya's "Owed $750" is not a refused read. |
| 6 | 3 | d: empty above filled — Grace: Messages ("No Messages yet", 136px) above Invoices; Camille: empty Offers and Birth outcome (before due date) above Details and Activity | a: all eight T2 facts on first screen at 1280, labelled, linked (names "…, Owed, in Invoices" etc.); header facts unlinked. b: both moves, 1 action. c: Birth Plan "4 of 6 questions answered · last changed Nov 1, 2026 by Priya Raman · Camille has not read it yet"; Contract "Signed on Sep 20, 2026"; Invoices "$750.00 owed, due Nov 29."; Visits "Next: Wed Nov 18…". |
| 7 | 4 | none | T1–T5 within budgets (below). |
| 8 | 1 | a: On call's first row ("Priya Raman … Nobody covering") bottom 872px > 800 at 1280. c: 6605px at 1440 = 7.3 screens (cap 3600). h: 4 decisions on 375 first screen (Message the Client, Add a Visit, Jump to, facts group) vs budget 3 | b: 375 first screen: header facts (Due 346), status Badge 404, Birth Plan link 445–470, On call tonight fact 574. d: per Priya log answer wholly on screen after one tap. e: Offers 149px, Birth outcome 151px (heading + line + button). f: "Void this Contract" `secondary`, transparent fill, after WarningText. g: see note on 1c'. i: Jump to 2 lines at 1280 (60px), 4 at 375; all 11 targets exist. |
| 9 | 3 | c: refused "Load more" keeps 30 rows but says "Failed to fetch" (not Voice words) | a: T10 section-level unverified; Engagement read refused (offline) → "Sorry, there is a problem … Go to your Practice overview" (but says "on our end", wrong offline). b unverified. |
| 10 | 4 | none | a T6 pass. b: "Recorded after the birth.", "Nobody has been offered this work yet." c: "Voiding ends this Contract for good. Nothing more can be invoiced under it…". d: no walker asked a term's meaning. |

**Total 33/40.**

Note 8g vs 1c': "Nobody on call tonight" is in the header status line and in the facts column. 1c' requires the header line, so I did not fail 8g; the two checks collide and the rubric should say which wins.

## 2. Tasks

| T | Result | Actions | Evidence |
|---|---|---|---|
| T1 | pass | 1 | Priya act 3: Jump to → Birth Plan, pain-relief answer wholly on screen; Birth Plan link at 445px (375×740) and 494–519px (320×640); full page has Print + PDF and "Back to Camille". |
| T2 | pass | 0 | 1280 screenshot: phase, due + 37 weeks, Doulas, nobody tonight, Wed Nov 18 11:00am, Signed, $750.00 due Nov 29, Unanswered since Thursday 9:22am. Imani "$950.00 overdue since Nov 3"; Grace "Sent, waiting…", "Nothing owed". |
| T3 | pass (save unverified) | read notes 2 (Jump to + scroll), write 3, Add a Visit form 1 | Change on Sun Nov 1 now opens directly under that row (row bottom 386, form heading 419), focus to form heading; Add a Visit focuses its form heading, Cancel returns focus to header link. |
| T4 | pass (send unverified) | 2 | Facts "Unanswered since Thursday, 9:22am" before opening; Messages section "Latest: …" + "Write a message". |
| T5 | pass (save unverified) | 0 to read, 2 to Record payment | Dee acts 2–4. Void secondary after WarningText. Dee missed a Cancel on the payment check step. |
| T6 | pass | ≤2 each | Contract: "Sent on Sep 13, 2026 by Renata Alvarez." (1). Bethany: Activity "Put on this work as the Doula: Bethany Kroll — Renata Alvarez" (1–2). Gap: "Gap in on-call cover added… Renata Alvarez". Filters hit `activity?kind=` (Dee act 7–8). |
| T7 | unverified | — | Lena reads "Your fee $450.00" and terms; Invoices with no amounts. She also saw Change on Marisol's Visits, "Record a time a Doula cannot be reached", "Start the Birth Plan", "Add Morgan's details"; whether the BFF refuses those was not checked. |
| T8 | pass | — | h1 "Camille Boyd", labelled facts after it, 11 h2 in drawn order, fact links "Signed, Contract, in Contract", Change described "the gap for Priya Raman", states in words. |
| T9 | pass | 5 stops | After skip link: Clients, Message the Client, Add a Visit, On call, Birth Plan; 2px solid outline oklch(0.4 0.11 258) on each; forms take focus, Cancel returns it. |
| T10 | unverified | — | Only Offline available: Engagement read → error page with a way back, wrong cause stated; per-section refusal not producible. |

## 3. Craft (/20)

| C | Score | Failed | Evidence |
|---|---|---|---|
| C1 | 4 | none | Unbranded, it reads as doula care: "Nobody covering — At a wedding four hours away", "37 weeks", Birth Plan, on-call days. No card grid, hero metric or eyebrow. |
| C2 | 3 | scale step: h1 35.6px/600 vs h2 28.2px/600 (1.26×, same weight); the title barely outranks "On call" | Hanken Grotesk, self-hosted, one family; tabular-nums on Invoice amounts, facts and Visit times; 65ch measure; h2 `text-wrap: balance`; no overflow at 320. Cross-screen role check unverified. |
| C3 | 4 | none | Amber act, neutral owed, red overdue; badge text tinted from its hue; `::selection`, caret-color, scrollbar-color, underline offset 3.6px in base.css; contrast: light warning L45 on L99, dark labels L73 on L21.5. |
| C4 | 3 | density: at 1280 the first section's first row ends at 872px; On call puts "When a Doula cannot be reached" before the on-call days it qualifies | One left edge at x=32; facts column level with Jump to. |
| C5 | 4 | none | DESIGN.md:171 names the activity ledger as the signature; it is quiet; no scattered motion seen. |

**Total 18/20.**

## 4. Findings (ranked by harm to a person's job)

1. **8c/8a/C4: the page is 7.3 screens at 1440, and at 1280 Renata's first section row is cut off (872px).** Fix: put On call's own days first and the gap row second; cap the Visits "Earlier" group and the Messages thread like the Birth Plan, so the page fits 4 screens.
2. **6d: empty sections above filled ones** (Grace: empty Messages above Invoices; Camille: empty Offers, pre-due Birth outcome above Details/Activity). Fix: write the per-phase order table, and sink every empty section below the filled ones as heading plus one line.
3. **Offline Engagement read says "Something went wrong on our end, not because of anything you did."** It is the device that is offline, and ADR-0059 wants a doula to read her Clients with no signal. Fix: tell offline apart from a server failure ("You're offline. This record will load when you're back online."), and serve the cached record.
4. **9c/Voice: refused "Load more" shows "Failed to fetch".** Fix: "Could not load more Activity. Check your connection, then try again." beside "Try again".
5. **8h: 4 decisions on the phone's first screen.** Fix: on a narrow container, put Add a Visit inside Visits and keep Message the Client in the header (or make the two moves one group the budget counts once). The fix must still meet 6b.
6. **3e: no top-bar `aria-current`.** This also undoes the breadcrumb decline's premise. Fix: set `aria-current="page"` on the list the record was reached from (Clients by default).
7. **C2: the h1 and h2 sizes are too close.** Fix: raise h1 a step (about 2.5rem, 700) or drop h2 to about 1.5rem.
8. **Back-link wording varies:** "← Clients", "Back to Contracts", "Back to Camille". Fix: one pattern everywhere.
9. **Dee:** no Cancel on the payment check step, and no Due column on the Invoice row. Fix: add Cancel to the check step and a Due column.

## 5. Expected and missing vs Category baseline

| Mine | Source | Baseline | Verdict |
|---|---|---|---|
| 1 Phone tel: | HubSpot, Salesforce | #27 Build | Built (`EngagementScreen.svelte:1295`, tel row when a phone exists); Camille has none, so not seen. |
| 2 Sticky header | Linear, Stripe | #7 Declined ("GOV.UK's sticky-element test dropped it for zoom faults and focus hidden under it") | The decline is about in-page nav and holds for that. Linear and Stripe readers on a 7-screen page still lose the name and moves, and the only Back to top is at the page's end. It is sound only if 8c is fixed. |
| 3 Actions menu | Stripe | not in baseline | — |
| 4 Create invoice | Stripe | not in baseline | — |
| 5 Prev/next | Linear, Gmail | #17 Declined ("no job on record; Back returns to the list") | Holds, but weakly: Renata working down "Needs cover" will go back and forth. |
| 6 Notes on Activity | Linear, HubSpot | not in baseline | — |
| 7 Attachments | HubSpot, Linear | not in baseline | — |
| 8 Copy link | Linear, Stripe | #15 Declined ("no job on record; the address bar holds it") | Holds. |
| 9 Breadcrumbs | Stripe, GOV.UK | #21b Declined ("back link names the list… top bar marks that list as the current place") | Unsound today. The top bar marks nothing current (3e fails), so half its premise is missing. It holds once 3e is fixed. |
