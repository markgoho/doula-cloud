# Grader B — Engagement record, round 1

Started 16:56 EST 2026-11-15, stopped gathering 17:18. Page id 14, marked window.__grader='B' (re-marked after each navigation). Live page judged at HEAD f416fb55 (Visit's Change form now under its own row: verified, form in the next `<tr>` after the Sun Nov 1 row, focus to its h3, Cancel returns focus to that row's Change).

Screenshots: B-1280-camille.png, B-375-camille.png, B-375-T1-birthplan.png, B-1280-visit-change-form.png, B-320-dark.png, B-slow4g-17s.png.

## Expected and missing (written before reading the surface brief; after the rubric + a look at Camille's record at 1280x800)

1. A status badge for the record's state beside the title (Stripe payment page, Linear status pill). Care phase is plain text "Care phase Active".
2. An overflow "More" menu for rarer acts on the record (Stripe "...", Linear "...", HubSpot "Actions").
3. Copy link / copy ID (Linear "Copy link", Stripe ID chip).
4. Previous / next record without going back (Linear j/k, Gmail, Zendesk).
5. Keyboard shortcuts (Linear, GitHub).
6. An internal note on the record that is not a Message to the Client (Linear comments, HubSpot notes, Zendesk internal note).
7. Tap-to-call the Client's phone (HubSpot mobile, any phone CRM). Only email in Camille's Details.
8. Sticky section nav / header on a long record (Stripe sticky header, GOV.UK contents + back to top).
9. Edit the Client's contact details in place (Stripe customer "Edit"). Goes to another page.
10. Follow/subscribe to the record (Linear subscribe, GitHub watch).

## 1. Usability (/40)

| # | Score | Failed | Evidence for passes |
|---|---|---|---|
| 1 | 3 | a | b: header "Care phase Active · Due Dec 5, 2026, 37 weeks · Doulas Priya Raman, Bethany Kroll" above Jump to. c/c': facts "On call tonight ⚠ Nobody" + header Badge "Nobody on call tonight". d: next Visit from the Visits read's `next` (`EngagementScreen.svelte:1505`), owed from Invoices (refusing Invoices turns it to "Could not be read"). e: console only vite debug lines. a FAIL: Slow 4G reload on the dev origin, page blank at 17s and still no `main` at 30s (B-slow4g-17s.png). Dev server's unbundled modules confound this; a preview build should be measured, but no skeleton was observed. |
| 2 | 3 | d | a: 0 matches for "engagement" in main text or aria-labels. b: Camille 37, Grace 28, Imani none (others unverified). c: "3 minutes ago", "Tuesday, 8:30am", older "Nov 1, 2026, 8:30pm"; 30/30 Activity rows carry `<time datetime>`. e: Grace "Sent, waiting for the Client to sign". d FAIL: Imani's facts say "No Messages yet", her Messages section says "No messages yet." — one record, two capitals for one domain noun. |
| 3 | 3 | e | b: Visit Change Cancel closes, no change. c: Birth Plan page "Back to Camille" → #birth-plan. d: page 6605px at 1440; one "Back to top" at the end (no section >900px at 1280/1440), moves focus to #jump-to. a: from Contracts the back link reads "Back to Contracts" (filter kept, On call: unverified; note "Back to Contracts" vs "Clients" — two styles). e FAIL: no `aria-current` in the top bar on the record, whether arrived from Clients or from Contracts (only Activity's "All" carries aria-current). |
| 4 | 4 | — (e: see 5) | a: at 1440 Visits table 1024 vs max-content 1194, Invoices 1024 vs 1098, Activity 681 = max-content; no other block >700 but Jump to (941, its links). b: facts column top 382 = Jump to top 382 at 1280; `@container record-body (min-width: 73rem)` in RecordDetail, no viewport query. c: `facts?: Snippet`; EngagementScreen sets no grid. d: overdue red oklch(0.506 0.193 28) on Imani, owed neutral, amber on act. e: baseline row 27's `tel:` is in source; my other expectations are declined or not in baseline (see 5). f T8 pass, g T9 pass. 320px: scrollWidth 310, no overflow, no target <24px. |
| 5 | 4 | — | a: only Message/Add a Visit in header; Void asks first with WarningText, red fill only on confirm. b: money writes gated (`InvoiceSection.svelte:142`), money reads are an employee's (ADR-0008:207). c: T7 pass. |
| 6 | 3 | d | a: every T2 fact on the first screen at 1280×800 (bottom ≈700), labeled, untruncated; links go to the right sections, header facts unlinked. b: both moves 1 action. c: Birth Plan "4 of 6 … last changed Nov 1, 2026 by Priya Raman · Camille has not read it yet"; Contract "Signed on…"; Invoices "$750.00 owed, due Nov 29." d FAIL: Camille: Offers and Birth outcome (empty, not yet due) above Details and Activity (filled); Imani: Care Plan "not started yet" and Messages (empty) above Details/Activity. |
| 7 | 4 | — | T1–T5 within budget (see Tasks). |
| 8 | 1 | a, c, h | b: 375 first screen holds header facts, "On call tonight Nobody" (y 573–598) and Birth Plan link (y 445). d: Birth Plan section 726px at 375 (cap 512px), full plan 1 action. e: Offers = h2 + one line + button. f: Void secondary (transparent bg), last subsection after WarningText. g: header facts and at-a-glance facts disjoint (the nobody-on-call Badge is 1c''s demand). i: Jump to 2 lines at 1280, 4 at 375; all links are sections. a FAIL: On call's first row (Priya's gap) is 687–872 at 1280×800. c FAIL: 6605px at 1440×900 = 7.34 screens (limit 3600). h FAIL: 375 first screen has 4 decisions by PR-G10's count: Message the Client, Add a Visit, Jump to, the at-a-glance link group. |
| 9 | 4 | — (b, c unverified) | a: Invoices refused → "We could not load the Invoices." + Try again, Owed fact "Could not be read", Visits intact; Visits refused → same, Next Visit "Could not be read", Invoices intact; Engagement refused → "Sorry, there is a problem … Go to your Practice overview". b: not tried (no save). c: my interception of Activity "Load more" did not match the URL; unverified. |
| 10 | 4 | — | a: T6 pass. b: "Nobody has been offered this work yet.", "Recorded after the birth.", Imani "No Birth Plan. This is postpartum care." c: WarningText before confirm. d: no walker asked a term's meaning. |

**Total 33/40** (3+3+3+4+4+3+4+1+4+4).

## 2. Tasks

| T | Result | Actions | Evidence |
|---|---|---|---|
| T1 | pass | 1 | Jump to → Birth Plan at 375×740: "No epidural…" at y 623–673, inside the cap (686), wholly on screen (B-375-T1-birthplan.png); Priya act 3. Link on first screen at 375 (y 445) and 320 (y 494–519). |
| T2 | pass | 0 | B-1280-camille.png: phase, due + 37 weeks, Doulas, Nobody on call, next Visit Wed Nov 18 11:00am, Signed, $750.00 due Nov 29, Unanswered since Thursday 9:22am. Imani "$950.00 overdue since Nov 3"; Grace "Sent, waiting…" and "Nothing owed". |
| T3 | pass (save unverified) | read 2 (Jump to Visits + scroll counted as reading), write 3, Add a Visit 1 | Priya acts 4–8; Change form now under its own row (verified live). Saving in place: not exercised. Priya's own note: the add form asks only a date, no type or notes. |
| T4 | pass (send unverified) | 2 | Unanswered + since read in facts before opening; fact link → Write a message → compose (Priya act 9–10). |
| T5 | pass (save unverified) | 0 read, 2 to Record payment | Dee acts 2–4. Void on signed Contract present per 8f and asks first (verified live). |
| T6 | pass | ≤2 each | Contract: "Sent on Sep 13, 2026 by Renata Alvarez" in Contract section; Bethany: "Doulas and on call" filter, BFF `activity?kind=doulas`; gap: "Gap in on-call cover added for the Client — Renata Alvarez, 3 minutes ago" in All. Load more appends in place (8→30 rows). |
| T7 | pass | — | Lena act 5–6: her fee and terms on Morgan's; Invoices dates/status with no money; gap writes for herself allowed (`oncall/gap.go:158`), Visit notes/date hers (ADR-0008:262). "Add Morgan's details" against the BFF: unverified. Terms vs Contract text disagreement is #2958, out of scope. |
| T8 | pass | — | Snapshot: h1 "Camille Boyd", then labeled header facts; h2s in drawn order; fact links "Nobody, On call tonight, in On call"; each Change described by its row ("the gap for Priya Raman", "Priya Raman, Sun Nov 1, 3:00pm"); states as words. 375 DOM order same (unchecked separately). |
| T9 | pass | 5 stops skip→Birth Plan (budget 6) | After skip link: Clients, Message, Add a Visit, On call, Birth Plan. Focus ring 2px solid oklch(0.4 0.11 258). Shift+Tab into a row under sticky `th`: element at hit-test is the focused button (not obscured). Change form takes focus (h3), Cancel returns it. |
| T10 | pass | — | see heuristic 9a. |

## 3. Craft (/20)

| C | Score | Failed | Evidence |
|---|---|---|---|
| C1 | 4 | — | On call tonight, weeks, Birth Plan's pain-relief answer, Coverage gap "Nobody covering" are this domain's; no card grid, hero metric, gradient, colored side border. |
| C2 | 3 | numbers line up | Hanken Grotesk self-hosted (not a platform sans); money `tabular-nums`. FAIL: Visits "When" column splits date and time with a justified gap — "Sun Nov 1,       3:00pm" vs "Wed Nov 18,  11:00am" — times do not line up and the gap reads as a glitch (B-1280-visit-change-form.png). Same-role-on-3-screens: unverified. |
| C3 | 4 | — | Tokens hue 258 tinted; `::selection` oklch(0.82 0.03 256), caret tinted, link underline offset 3.55px, focus ring colored; dark: labels 7.33:1, Badge 5.35:1, values 13:1. |
| C4 | 2 | alignment, density | Facts column top = Jump to top (382). FAIL alignment: the When column gap above. FAIL density: at 1280 the first section's first row ends at 872 > 800, and the page is 7.3 screens at 1440 — the record's job does not fill the first screen. |
| C5 | 4 | — (signature record unverified) | Nothing ornamental; the only colored element on first screen is the nobody-on-call warning, which is the job. No motion observed. |

**Total 17/20.**

## 4. Findings, ranked by harm to a real person's job

1. **8a / C4 density: the first section's first row is cut at 1280×800** (On call row 687–872). Renata at a desk sees "Nobody covering" half off-screen. Fix: tighten the header-to-section gap (Jump to → first h2 is ~100px) and put the gap row's reason/Change/Clear on one line beside the dates in a wide container; or cap the header's vertical rhythm so the first row ends ≤800.
2. **8c: page is 6605px at 1440 (7.3 screens, limit 3600).** Fix: cap Visits' Earlier group and Activity at fewer rows with Load more (e.g. 3 past Visits, 5 Activity rows), cap Care Plan like Birth Plan, collapse empty sections (Offers, Birth outcome) to one line each.
3. **8h: 4 decisions on the 375 first screen** (Message, Add a Visit, Jump to, facts links). Fix: put the two header moves in one group that reads as one decision only if PR-G10 counts it so; otherwise move "Add a Visit" into the Visits section's state line on narrow containers, or drop the facts' links on the phone's first screen.
4. **6d: empty sections above filled ones** (Camille: Offers, Birth outcome above Details, Activity; Imani: Care Plan, Messages above Details, Activity). Fix: sort empty sections after filled ones in `sectionOrder.ts` (keeping the On call / Birth outcome exceptions), or write the per-phase table into the surface brief.
5. **3e: no `aria-current` on the top bar** on the record (from Clients or Contracts). Fix: mark the list the record belongs to (`listEntryHref` already knows it) with `aria-current="page"` (or "true") in the nav.
6. **1a: no skeleton within 300ms under Slow 4G** (blank 30s on the dev origin). Fix/verify: measure on a preview build; ensure the SSR/app-shell paints header/facts skeletons before data.
7. **C2/C4: Visits When column date/time gap** ("Sun Nov 1,      3:00pm"). Fix: render date and time as one inline `<time>` with a normal space, or two left-aligned sub-columns with tabular nums.
8. **2d: "No Messages yet" (facts) vs "No messages yet." (section) on Imani.** Fix: one capital per the domain-noun rule in both strings.
9. Minor, not scored: back link style differs by origin ("Clients" vs "Back to Contracts"); "Not scheduled yet" group header over a row reading "Not yet scheduled"; the Void confirm repeats the WarningText already shown just above it; the a11y tree reads each Change's description text twice ("the gap for Priya Raman" as a separate node); Activity copy "added for the Client" where the Client's name is known.

## 5. Expected and missing, against the brief's Category baseline

| My expectation | Source | Baseline | Verdict |
|---|---|---|---|
| 1. Status badge beside title | Stripe, Linear | row 1 (status in header): Build | Built as labeled text, not a badge; nobody-on-call is a Badge. Holds. |
| 2. Overflow "More" menu | Stripe, Linear, HubSpot | not in baseline | Not in baseline; acts live in their sections (GOV.UK). Fine for this reader. |
| 3. Copy link / ID | Linear, Stripe | row 15 Declined: "no job on record; the address bar holds it" | Holds for a Stripe/Linear-trained reader on desktop; weaker on a phone where the address bar is hidden, but no job asks to share a record. |
| 4. Prev/next record | Linear, Gmail | row 17 Declined: "no job on record; Back returns to the list" | Holds weakly: Renata working down a Needs-cover list would use it; #2949 (return to the row) mitigates. |
| 5. Keyboard shortcuts | Linear, GitHub | row 16 Later (#2956) | Deferred to an app-wide decision; holds. |
| 6. Internal note (not to the Client) | Linear, HubSpot, Zendesk | not in baseline | Not in baseline. Visit notes cover care notes; a Staff-only note about the Client (e.g. "partner prefers texts") has no home. Worth a ticket, not a fail. |
| 7. Tap-to-call phone | HubSpot mobile | row 27 Build | Built in source (`tel:`); Camille's record shows only email (no phone saved), so not observed live. |
| 8. Sticky section nav | Stripe, GOV.UK | row 7 Declined: "GOV.UK's sticky-element test dropped it for zoom faults and focus hidden under it" + Back to top | Holds (Back to top built and moves focus). |
| 9. Edit contact details in place | Stripe | row 11 Have it (each summary row's Change) | Partly: Details' Change exists for care fields; Client contact goes to the Client page ("Add Camille's details"). Acceptable, Client record owns contact. |
| 10. Follow/subscribe | Linear, GitHub | not in baseline | Not in baseline; no job on record. |
