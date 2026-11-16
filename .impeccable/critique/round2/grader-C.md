# Grader C, round 2, Engagement record (at f4cacd20)

## Expected and missing (written before reading the surface brief)

Seen: Camille's record at 1280x800 (snapshot + `grader-C-1280-camille.png`), the rubric's jobs only.

1. **Client's phone number, tap to call / text** (only email shown in Details). Taught by: HubSpot / Salesforce contact records (phone in header, click-to-call).
2. **Partner / support person and emergency contact** as data (Marcus is only prose in the Care Plan). Taught by: Jane App, SimplePractice client profile.
3. **Birth location / hospital as a fact at a glance** (only inside the Birth Plan). Taught by: Jane / SimplePractice location on profile.
4. **Previous / next record** without returning to the list. Taught by: Linear issue view (j/k), Gmail, Front.
5. **Copy link to this record.** Taught by: Linear, Stripe Dashboard (copy ID).
6. **Status / phase change from the header.** Taught by: Linear status dropdown, Stripe "Actions".
7. **Overflow "Actions" menu in the header.** Taught by: Stripe Dashboard customer page.
8. **Internal Staff notes** separate from the Client thread. Taught by: HubSpot Notes, Linear comments, Front internal comments.
9. **Tasks / reminders / follow-ups.** Taught by: HubSpot Tasks, Salesforce Activities.
10. **Files / attachments.** Taught by: HubSpot, SimplePractice documents.
11. **Keyboard shortcuts.** Taught by: Linear, Gmail.
12. **Print / export the whole record.** Taught by: SimplePractice chart export.

## 1. Usability (/40)

| # | Score | Failed checks | Evidence for passes |
|---|---|---|---|
| 1 Visibility | 4 | none (1a, 1d unverified) | 1b header "Care phase Active · Due Dec 5, 2026, 37 weeks · Doulas Priya Raman, Bethany Kroll" above Jump to (1280, 375 shots); Imani no due line. 1c fact "On call tonight: Nobody". 1c' amber Badge "Nobody on call tonight" in header; plain words in fact and On call line. 1e no console error/warn. 1d by source only (`first.next` EngagementScreen.svelte:1568, `owed` :1447); network not captured = unverified |
| 2 Match | 4 | none | 2a "engagement" in innerText + aria-labels = 0. 2b Camille "37 weeks" (others unverified). 2c "5 minutes ago", "Tuesday, 8:30am", older "Oct 25, 2026, 10:30am", each `<time datetime>` with offset. 2e "Signed" |
| 3 Control | 3 | 3b | 3b FAIL: Record a payment's check step ("Confirm and record") has no Cancel (RecordPaymentForm.svelte:176-216: Cancel only on step 'form'; Dee r1 act 5). Passes: compose Cancel closes and discards; 3d 5512px page, no section >900, "Back to top" at end (focus move unverified); 3e top bar "Clients" aria-current=page, back link "Clients". 3a, 3c unverified |
| 4 Consistency | 2 | 4e, 4f | 4e FAIL: baseline row 27 (Build) phone with `tel:` absent: 0 `a[href^=tel:]`, Details has Email/Portal invite/Created only. 4f FAIL (T8): Details `<aside>` h2 comes before "On call" h2 in DOM/heading order but is drawn at y=734 in the right column, below On call's heading (y=483) at 1280. Passes: 4a at 1440 Visits 1024 (max-content 1208), Invoices 1024 (1116), Activity 661 (661), Notice 657; 4b facts at x=975 beside main, top ~392 vs Jump to 396, no `@media` in EngagementScreen/StatusLine/RecordDetail; 4d overdue oklch(0.506 0.193 28) red (Imani), "Owed" neutral badge, amber warning; 4g T9 pass. 320px: scrollWidth 310, no target <24px. 4c unverified |
| 5 Error prevention | 4 | none | 5a header holds only "Message the Client"; void has a hidden "Void this Contract?" confirm. 5b ADR-0008:11 employee Doula reads money, so Priya's Owed is allowed. 5c Lena r1 log + orchestrator: no Owed, Invoices without amounts, fee $450 |
| 6 Recognition | 4 | none (6d Grace, Jamie unverified) | 6a five facts labeled in first 800px at 1280, links "X, Label, in Section". 6b. 6c Birth Plan "4 of 6 … last changed Nov 1 … Camille has not read it yet", Invoices "$750.00 owed, due Nov 29." 6d Camille, Imani: empties after filled, Activity last |
| 7 Flexibility | 3 | T3 | T1, T2, T4, T5 pass |
| 8 Aesthetic | 3 | 8c | 8c FAIL: 5512px at 1440×900 (>5400). Passes: 8a On call h2 483, line ~553, first row bottom ~715; 8b 375 first screen holds header facts, badge, Birth Plan link (bottom 470), on-call fact (596); 320 Birth Plan bottom 467; 8f Void outline secondary after WarningText; 8i Jump to 2 lines @1280, 4 @375; 8e Offers/Birth outcome line + button; 8d Care Plan behind "Show the whole Care Plan" |
| 9 Error recovery | 4 | none (9a, 9b unverified) | 9c offline "Load more": 30 rows kept, "We could not reach the service. Check your connection and try again." + "Try again" |
| 10 Help | 4 | none | T6 pass; Imani "No Birth Plan. This is postpartum care."; void warning in words before confirm |

**Total 35/40.**

## 2. Tasks

| T | Result | Actions | Evidence |
|---|---|---|---|
| T1 | pass | 1 | Priya r2 act 3: answer at y=624/740; Birth Plan link on first screen at 375 and 320 |
| T2 | pass | 0 | 1280 shot: all eight facts; Imani "$950.00 overdue since Nov 3" red. Grace unverified |
| T3 | fail | read 2, write 4 | After Jump to Visits at 375, last logged Visit's notes at 1445–1569px in a 740 viewport (Coming up + Not scheduled cards above; grader-C-375-jump-visits.png). Write = Jump + scroll + Change + tap Notes (focus lands on form heading, Priya act 5) = 4 > 3. Add a Visit first field in 2: pass |
| T4 | pass | 1 | "Message the Client" at 375: focus in TEXTAREA; newest Message on screen (243–293) |
| T5 | pass | 0 / 2 | Dee r1 acts 2–5; Void secondary + confirm now |
| T6 | pass | 1 each | Contract line "Sent on Sep 13, 2026 by Renata Alvarez"; ?activity=money server-loaded; Bethany put on and gap added rows, with Who |
| T7 | pass | – | Lena r1 log + orchestrator evidence |
| T8 | fail | – | heading order Details → On call vs drawn On call above Details |
| T9 | pass | 4 stops | Clients, Message the Client, On call, Birth Plan; ring 2px solid oklch(0.4 0.11 258) offset 2px. Note: compose opened from header link, Cancel returns focus to "Write a message", not the opener |
| T10 | unverified | – | dev server won't load offline; source shows per-fact "Could not be read" |

## 3. Craft (/20)

| C | Score | Failed | Evidence |
|---|---|---|---|
| C1 | 4 | – | weeks, on-call gap with reason, Birth Plan answers in the Client's words, read receipt; no refuse-list scaffold |
| C2 | 4 | – (3-screen sameness unverified) | Hanken Grotesk self-hosted, h1 35px/600, h2 22px; tabular-nums; prose 65ch |
| C3 | 4 | – (dark contrast unverified) | ::selection, caret, scrollbar-color, underline offset 3.55px, focus ring all set; hues one meaning each |
| C4 | 3 | alignment | ragged right edges (summaries ~678–693, Notice 657, tables 903/1024, facts at 975 leaving ~290px void at 1280); Invoice Actions wrap "Record payment Void / Write off" at 1440 with room left (grader-C-1440-invoices.png) |
| C5 | 4 | – (motion, DESIGN.md signature unverified) | one loud element (header Badge); nothing ornamental |

**Total 19/20.**

## 4. Findings (ranked)

1. T3: last Visit's notes 1.1 screens below Visits on a phone; Change focuses heading not Notes. Fix: put Earlier (last logged) before Not scheduled yet, or collapse unscheduled to one line; focus Notes on Change of a past Visit.
2. 4e: Client phone `tel:` (row 27, Build) absent. Fix: Phone row, `tel:` link, "Not given" + add when empty.
3. 4f/T8: Details aside before On call in heading order, drawn below. Fix: aside after main sections in DOM; grid places it.
4. 3b: payment check step has no Cancel. Fix: secondary Cancel, focus back to "Record payment".
5. 8c: 5512px > 5400. Fix: Visits default to next + last logged; merge Invoices notice into state line.
6. C4: ragged edges, wrapped Invoice actions. Fix: nowrap Actions cell; one measure for summaries/tables.

## 5. Expected and missing vs baseline

| Item | Source | Verdict |
|---|---|---|
| 1 Phone tap-to-call | HubSpot/Salesforce | row 27 Build — NOT built |
| 2 Partner/emergency contact | Jane/SimplePractice | not in baseline |
| 3 Birth location at a glance | Jane/SimplePractice | not in baseline |
| 4 Prev/next | Linear/Gmail | row 17 Declined "no job on record" — holds weakly |
| 5 Copy link | Linear/Stripe | row 15 Declined "address bar holds it" — holds |
| 6 Header status change | Linear/Stripe | row 11 (Change by the property) — holds for GOV.UK readers |
| 7 Overflow menu | Stripe | row 28 Declined — holds |
| 8 Internal notes | HubSpot/Linear | row 29 Build — built (Care Plan) |
| 9 Tasks | HubSpot | not in baseline |
| 10 Files | HubSpot/SimplePractice | not in baseline |
| 11 Shortcuts | Linear | row 16 Later (#2956) |
| 12 Print record | SimplePractice | row 18 Declined — holds |
