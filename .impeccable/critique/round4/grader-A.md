# Grader A, round 4, Engagement record

## Expected and missing (written before reading the surface brief)

Having read the grader brief and the rubric's jobs and looked at Camille's record (a11y snapshot at 1280×800; screenshots of a background tab time out), things I expected on a record screen of this kind and could not find:

1. Previous/next record navigation from inside the record (Linear J/K on an issue, Gmail's older/newer, HubSpot's record pager). Not present; only the back link to Clients.
2. Keyboard shortcuts / command menu to jump to a section or act (Linear Cmd-K, GitHub `?`). Not present ("Jump to" links only).
3. Copy the record's link or ID (Stripe Dashboard customer "Copy ID", Linear "Copy issue URL"). Not present.
4. A free internal note / pinned note on the record (HubSpot contact Notes, Stripe customer "Notes" / metadata, Salesforce case comments). Care Plan answers partly serve; no free note.
5. Files / attachments on the record (HubSpot, Salesforce Files, Zendesk). Not present (Contract PDF only).
6. Tasks or reminders tied to the record ("Create task" on HubSpot, follow-up on Salesforce). Not present.
7. Status change from the header (Linear's status pill, Jira's status button). Care phase "Change" is in Details only, not at the phase in the header.
8. Print the whole record (GOV.UK caseworker summaries, Stripe invoice print). Only the Birth Plan prints.
9. Comment / @mention a colleague on the Activity (Linear, GitHub timeline). Not present.
10. Search within the Activity or Messages (Front, Help Scout conversation search). Not present.

(Header email/phone, Activity filters, Back to top and an in-place compose were expected and are present.)

## Method note

Live on my own page (pageId 13, marked `__grader='A'`), Renata's session, no throttling. Every screenshot of my background tab timed out (retried after `emulate`), so evidence is measured values and the accessibility snapshot; no image files. Coordinates are page y (top + scrollY).

## 1. Usability (/40)

| # | Score | Failed checks | Evidence for passes |
|---|---|---|---|
| 1 Visibility | 4 | none (1a unverified: Slow 4G skeleton per brief; no-shift half not measured) | 1b header "Care phase Active · Due Dec 6, 2026, 37 weeks · Doulas Priya Raman, Bethany Kroll" y=333, before Jump to (382). 1c fact "On call tonight: Nobody". 1c' `span.warning` "Nobody on call tonight" in header status line, amber. 1d in-page BFF reads: visits `next.scheduledAt 2026-11-19T11:00-05:00` = "Thu Nov 19, 11:00am"; invoices `owed {owedCents:75000, nextDueOn:2026-11-30}` = "$750.00, due Nov 30". 1e no console error/warn on load. |
| 2 Real world | 4 | none (2b Tamsin/Imani/Mira/Imogen unverified) | 2a 0 matches /engagement/i in innerText and aria-labels. 2b Camille "37 weeks", Grace "28 weeks". 2c "7 minutes ago", "Wednesday, 8:30am"; older "Oct 26, 2026, 10:30am", every row a `<time datetime>` with offset. 2e Grace "Sent, waiting for the Client to sign"; Camille "Signed". |
| 3 Control | 4 | none (3a from On call unverified) | 3a/3e Contracts?status=signed → Imani: back link "Contracts" href keeps `?status=signed`, `aria-current` on Contracts; from Clients the same with "Clients". 3b Cancel changes nothing (Priya 9, 11, 14; Dee 5). 3c Birth Plan page back link "Camille" → `#birth-plan`. 3d 1440: page 5181px, no section > 900 (max 619), "Back to top" at end → `#jump-to` (ul tabindex=-1). |
| 4 Consistency | 3 | 4f (T8) | 4a 1440: Visits/Invoices/Activity tables 1024 wide (max-content 1912/1160/1213), one right edge 1060 with section dl rows (36–1060); no p/dd/form/status wider than 65ch (663px). 4b facts column top 382 = Jump to top 382, x 975–1238 beside main 32–935 at 1280; beside at 1440. 4c `facts?: Snippet` on RecordDetail; EngagementScreen's grids are inner rows only. 4d Imani owed `.fact.error` red; Camille owed neutral; nobody-on-call amber. 4g T9 passes. 320×640 scrollWidth 320, nothing past the edge, no target < 24px. |
| 5 Error prevention | 4 | none (T7 address half unverified) | 5a header holds only "Message the Client"; Void has a confirm step ("Void this Contract?"). 5b Priya reads money (ADR-0008, employee); no Record payment/Void/Write off in her log (`isOwnerOrAdmin` passed to InvoiceSection). 5c Lena: only $450.00, Invoice dates/status only. |
| 6 Recognition | 4 | none (6d Jamie unverified) | 6a 1280: eight facts labelled, y 333–~730 (< 800); links on call→#on-call, next Visit→#visits, Contract→#contract, Owed→#invoices, Messages→#messages; header facts unlinked. 6b header "Message the Client"; "Add a Visit" Visits' first control. 6c Birth Plan "4 of 6 questions answered · last changed Nov 2, 2026 by Priya Raman · Camille has not read it yet"; Contract "Signed on…"; Invoices "$750.00 owed, due Nov 30." 6d Camille, Grace, Imani (filled Visits, Invoices, Birth outcome, Contract; then empty Care Plan, Messages, Birth Plan, Offers, On call; Activity last). |
| 7 Efficiency | 4 | none | T1–T5 below. |
| 8 Aesthetic | 4 | none (8f "secondary" styling not measured; placement verified) | 8a On call h2 483, state line, first row 687–715 ≤ 800. 8b 375: header 317–371, on-call fact 560, Birth Plan link 417–442. 8c 5181px. 8d answers `.capped` max-height 512 (472 used) at 375; Care Plan "Show the whole Care Plan". 8e Offers 151px: heading, line, button. 8f WarningText then "Void this Contract", last in Contract. 8h 3 decisions (PR-G10 count). 8i 2 lines at 1280, 4 at 375; 11 links, all sections. |
| 9 Recovery | 4 | none (9b unverified, saving forbidden) | 9c offline "Load more": 30 rows kept, "We could not reach the service. Check your connection and try again." + "Try again" beside it. T10 from source/spec: each fact "Could not be read" on its section's error (`EngagementScreen.svelte:1441,1482,1498,1540,1547`), spec `engagement-detail.svelte.spec.ts:5145` Visits Try again; Engagement read failure → `FailedRead` (`+page.svelte:77`). |
| 10 Help | 4 | none | T6 passes. 10b Grace "Invoices start once Grace signs the Contract."; Imani "No Birth Plan. This is postpartum care." 10c void Warning says what voiding does before confirm. 10d no walker asked a term. |

**Total 39/40.**

## 2. Tasks

| Task | Result | Actions | Evidence |
|---|---|---|---|
| T1 | pass | 1 | 375: Jump to "Birth Plan" → "No epidural unless I ask twice…" y 625–675 of 740 (mine; Priya act 3). Link on first screen at 375 (417) and 320×640 (442–467). Full plan, Print, PDF 1 action away. |
| T2 | pass | 0 | 6a; Imani "$950.00 overdue since Nov 4" red; Grace "Sent, waiting…", "Nothing owed". |
| T3 | pass | read 2, write 3, add 2 | Priya 6–7, 10. Save in place: orchestrator evidence + source (re-read sets `next`/`last`, `:1648`). |
| T4 | pass | 1 | "Unanswered since Friday, 9:22am" before opening; "Message the Client" opens compose, focus in Message (orchestrator); Priya via "Write a message" (act 12). |
| T5 | pass | 0 / 2 | Dee 2–4; Void per 8f, asks first. |
| T6 | pass | 1 each | Contract "Sent on Sep 14, 2026 by Renata Alvarez"; Activity "Put on this work as the Doula: Bethany Kroll · Renata Alvarez · 7 minutes ago"; "Gap in on-call cover added for the Client · Renata Alvarez". Filters are BFF reads (`?activity=money`, `aria-current` on chosen); Load more appends in place. |
| T7 | pass (part unverified) | 2 | Lena 5–8: own fee $450.00 and terms, no Practice money; refused-address half unverified. |
| T8 | **fail** | — | h1 name, header facts follow labelled, fact links name fact + section, Phone "Call Camille Boyd, (585) 555-0142", Change described by its row. Fails drawn order: at 1280 h2 sequence is Details (drawn y=858, right column) → On call (drawn y=483) → Birth Plan. |
| T9 | pass | 4 after skip | Skip → Clients → Message the Client → On call → Birth Plan (budget 5). Ring 2px solid oklch(0.4 0.11 258) offset 2 on every main control; only sticky elements are 40px table headers. Forms take focus, Cancel returns it (Dee 4–8, Priya 7–11). |
| T10 | pass (source/spec) | — | Could not refuse one read alone with Offline emulation. |

## 3. Craft (/20)

| Dim | Score | Failed | Evidence |
|---|---|---|---|
| C1 | 4 | none | Domain-only: "Nobody on call tonight: Priya Raman cannot be reached until…", weeks, Birth Plan answers, on-call windows "from 37 weeks". No border-left > 1px in main, no card grid, hero metric, eyebrow. |
| C2 | 4 | none (sameness across 3 screens unverified) | Hanken Grotesk via @font-face; h1 35px/600, h2 27.8px; money and Activity times `tabular-nums`; prose ≤ 65ch; nothing overflows at 320. |
| C3 | 4 | none (dark-theme contrast unverified) | `::selection`, `caret-color`, `scrollbar-color`, `text-underline-offset` in the sheets; tinted bg oklch(0.99 0.003 255), ink oklch(0.23 0.025 260); amber = act, red = overdue. |
| C4 | 4 | none (other screens unverified) | heading→state 20, state→body 20, section→section 41; one left edge 32 at 1280; job fills first screen at 1280 and 375. |
| C5 | 4 | none (DESIGN.md naming, motion unverified) | Activity ledger with BFF filters is the one signature; no ornament. |

**Total 20/20.**

## 4. Findings, ranked

1. **4f / T8: heading order departs from drawn order at desk widths.** At 1280 a screen-reader user meets "Details" (contact, created, Change acts) before "On call", the first section sighted readers meet. Fix: put the Details region after the sections in source order (as the one-column layout already does with its second Details) and place it in the facts column by grid area/row; leave the facts `dl` where it is.

Observed, not rubric failures: Escape does not close Record a payment (Dee 7); its Continue/Cancel below the fold at 1280×800 (Dee 4); Cancel drops typed notes/messages with no guard (Priya 9, 14); Phone fact 3px below the 375×740 fold; Activity shows 5 rows before Load more.

## 5. Expected and missing vs the Category baseline

| Expectation (source) | Baseline |
|---|---|
| Previous/next record (Linear, Gmail, HubSpot) | Declined row 17, "no job on record; Back returns to the list". Holds weakly: a Linear-trained owner triaging a list reaches for it. |
| Shortcuts / command menu (Linear, GitHub) | Later row 16 (#2956). Holds. |
| Copy link or ID (Stripe, Linear) | Declined row 15, "the address bar holds it". Holds. |
| Internal note (HubSpot, Stripe, Salesforce) | Built row 29 (Care Plan, "Camille never sees the Care Plan"). |
| Files on the record (HubSpot, Salesforce) | Not in the baseline. |
| Tasks / reminders (HubSpot, Salesforce) | Not in the baseline. |
| Status change from the header (Linear, Jira) | Row 11 Have it (Change beside the property in Details). Holds for GOV.UK readers. |
| Print the record (GOV.UK caseworker) | Declined row 18; Birth Plan prints. Holds. |
| Comment / @mention on Activity (Linear, GitHub) | Not in the baseline. |
| Search in Activity/Messages (Front, Help Scout) | Not in the baseline (9b kind filter nearest). |
