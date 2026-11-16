# Grader B, round 4: Engagement record

## Expected and missing (written before reading the surface brief)

Having read only the grader brief and the rubric's jobs, and having looked at Camille's record at 1280x800 (snapshot + `B-camille-1280.png`):

1. **Overflow "More" menu of record-level actions in the header** (Stripe Dashboard customer page "Actions" menu; Linear issue "..." menu). Only "Message the Client" is in the header; record-level acts (change phase, due date, Doulas) sit as buttons in the Details aside.
2. **Previous/next record without going back to the list** (Linear issue view up/down; Gmail newer/older). None.
3. **Keyboard shortcuts / command menu** (Linear Cmd-K, GitHub `?`). None.
4. **Copy the record's link or ID** (Stripe "Copy ID"; Linear "Copy issue URL"). None.
5. **Client's home address with a map link** for an on-call doula leaving for a labor (Jane App / HubSpot contact card address). Details shows email, portal invite, created; no address, no partner/emergency contact on the record itself.
6. **Upload / attach a file** (HubSpot attachments, HoneyBook files). None.
7. **Tasks / follow-up reminders on the record** (HubSpot tasks, Salesforce activities). None.
8. **A free-form internal note** (HubSpot notes, Linear comments). The Care Plan's fixed questions stand in; no timestamped note.
9. **Create an Invoice from the record** (Stripe customer page "Create invoice"). Not offered on Camille's (fully billed; may be correct).
10. **Activity search or filter by person** (Stripe events filter, Linear history). Kind filter only.

## Method

Own page (pageId 11, `window.__grader='B'` checked before each read), Renata's session, dev origin :5273. Widths 1440x900, 1280x800, 640x900 (stands in for 200% zoom at 1280), 375x740, 320x640; dark theme at 1280. T10 and 9c: a page-local `fetch` wrapper (initScript / evaluate) answered 500 for one read (Invoices, Visits, the Engagement, Activity "Load more"); nothing on the server changed. Other records read through same-origin iframes at 1280. Screenshots: `B-camille-1280.png`, `B-camille-375.png`, `B-camille-1280-dark.png` (full-page captures timed out). Act logs for T1, T3, T4 (Priya), T5 (Dee), T7 (Lena).

## 1. Usability

| # | Score | Failed | Evidence for passes |
| --- | --- | --- | --- |
| 1 | 4 | none (1a unverified: skeleton timing per brief; no-shift half not measured by me) | b: header "Care phase Active / Due Dec 6, 2026, 37 weeks / Doulas Priya Raman, Bethany Kroll"; Tamsin "No Doula yet", Imogen no due line. c/c': header warning Badge "Nobody on call tonight", fact "On call tonight: Nobody", On call line "Nobody on call tonight: Priya Raman cannot be reached until Mon Nov 16, 10:00am." d: `/visits` serves `next` 2026-11-19T11:00 and `last`; `/contract/invoices` serves `owed.owedCents 75000, nextDueOn 2026-11-30` = facts. e: no console error/warn on load. |
| 2 | 4 | none | a: `/engagement/i` false on main innerText and aria-labels on 9 records. b: Camille 37, Tamsin 36, Grace 28; Imani, Mira, Imogen none. c: "31 minutes ago", "Wednesday, 8:30am", then "Nov 2, 2026, 8:30pm"; `<time datetime="2026-11-16T09:27:18-05:00">`. e: "Draft, not sent yet", "Sent, waiting for the Client to sign", "Signed", "Voided, no longer active". |
| 3 | 4 | none (3a On call/Contracts origins and 3c Birth Plan page's way back unverified live; 3a from `listOrigin.svelte.ts` + `engagement-back-link.svelte.spec.ts`) | b: Cancel changed nothing (Priya acts 9, 11, 14; Dee act 5). d: page 5181px at 1440; no section > 900px; one "Back to top" at the end, `#jump-to` is a `ul tabindex=-1`. e: top bar `aria-current="page"` on Clients; back link "Clients". |
| 4 | 4 | none (4d's red-for-overdue hue and Jamie Bishop's state unverified) | a at 1440: Visits/Invoices/Activity tables 1024px = main column, max-content 1912/1160/1213, shared right edge 1060; prose 663px at `--measure` 65ch. b: facts column top 382 = Jump to top 382 at 1280; `@container record-body (min-width: 73rem)` in RecordDetail, no `@media`. c: `facts?: Snippet` in RecordDetail; EngagementScreen sets no grid-template. f: h1 "Camille Boyd", labelled facts follow, h2s per section, Phone link "Call Camille Boyd, (585) 555-0142", Change buttons described by row. g: skip link → Enter → 4 Tabs to "Birth Plan" (Clients, Message the Client, On call, Birth Plan); 2px solid focus outline, offset 2px; no sticky page chrome. Cross-cutting: no horizontal scroll at 320, 375, 640; no target < 24px; dark contrast 7.3–13.0. |
| 5 | 4 | none | a: header holds only "Message the Client"; void asks first (hidden h2 "Void this Contract?"). b: Record payment/Void/Write off gated on `isOwnerOrAdmin` (`InvoiceSection.svelte:720`, `:829`); employee reads money per ADR-0008. c: T7 passes. |
| 6 | 4 | none (Jamie Bishop unverified) | a: all eight T2 facts above 800px at 1280 with labels, links to the named sections; header facts carry none. b: header "Message the Client"; "Add a Visit" Visits' first control. c: Birth Plan "4 of 6 questions answered · last changed Nov 2, 2026 by Priya Raman · Camille has not read it yet"; Contract "Signed on Sep 21, 2026"; Invoices "$750.00 owed, due Nov 30." d: Grace, Tamsin, Imani, Mira, Talia, Willa, Ingrid orders read: no empty section above a filled one outside the two exceptions; Activity last. |
| 7 | 4 | none | T1–T5 within budget (below). |
| 8 | 4 | none | a: On call h2 483, first row bottom 723 at 1280. b: 375 shot: header facts, Badge, Jump to with Birth Plan, "On call tonight Nobody". c: 5181px ≤ 5400. d: Birth Plan section 727px at 375 (≤ 740); Care Plan behind "Show the whole Care Plan". e: Offers 151px (heading, line, button). f: "Void this Contract" class `secondary`, transparent fill, after WarningText in its h3 subsection. g: no repeats beyond the exempt on-call alert. h: 3 decisions at 375 (Message the Client, Jump to, the fact links). i: Jump to 2 lines at 1280, 4 at 375; every link a section. |
| 9 | 3 | c | a: T10 passes. b: unverified (saving forbidden). |
| 10 | 4 | none | a: T6 passes. b: "Invoices start once Grace signs the Contract."; "The Birth Plan is not started yet."; "No Birth Plan. This is postpartum care."; "No Birth Plan. The birth outcome is recorded." c: "Voiding ends this Contract for good. Nothing more can be invoiced under it..." d: no walker asked a term's meaning. |

**Total 39/40.**

## 2. Tasks

| Task | Result | Actions | Evidence |
| --- | --- | --- | --- |
| T1 | pass | 1 | Priya act 3: Jump to "Birth Plan" → heading at 76px, "No epidural unless I ask twice." on screen; full plan link "Read or print the whole Birth Plan". Birth Plan link bottom 467 at 320x640, ~470 at 375x740. |
| T2 | pass | 0 | 1280 shot: all eight facts with labels; Imani "$950.00 overdue since Nov 4"; Grace "Sent, waiting for the Client to sign", "Nothing owed". |
| T3 | pass (save half from orchestrator evidence and spec, earlier build) | read 2, write 3, add 2 | Priya acts 6–7 ("Last Visit: Mon Nov 2" → notes; Change → caret in Notes), act 10 (Add a Visit → form, field visible). |
| T4 | pass (send half from orchestrator evidence) | 1–2 | Fact "Unanswered since Friday, 9:22am" before opening; "Message the Client" opens compose with focus in Message; act 12: newest message and box on one screen. |
| T5 | pass | 0 / 2 | Dee act 2 (owed, due 0 actions), acts 3–4 (Owed link → Record payment); void secondary and asks first. |
| T6 | pass | 1 each | Contract section "Sent on Sep 14, 2026 by Renata Alvarez."; Activity first rows "Put on this work as the Doula: Bethany Kroll / Renata Alvarez", "Gap in on-call cover added for the Client / Renata Alvarez"; `activity?kind=money` served by BFF (Dee act 10); Load more appended 25 rows in place. |
| T7 | pass ("address tells her nothing" unverified live; ADR-0008 404 rule) | 2 | Lena acts 6–8: "Your fee $450.00", terms, Invoices with dates and status only; no other money; controls shown all on ADR-0008's write table (Visit date/notes, gaps are AttachingWrite in `oncall/mount.go`). |
| T8 | pass | — | Snapshot at 1280 (see 4f). Note: at wide widths heading order is Details (right column, top 858) before On call (top 483). |
| T9 | pass | 4 tabs | See 4g; Dee acts 4–8: form takes focus on its heading, Cancel returns to "Record payment". |
| T10 | pass | — | Invoices refused: "We could not load the Invoices." + "Try again", Owed fact "Could not be read", others stand. Visits refused: "We could not load the Visits." + "Try again", Next Visit "Could not be read". Engagement refused: "Sorry, there is a problem / Something went wrong on our end, not because of anything you did... / Go to your Practice overview". |

## 3. Craft

| Dim | Score | Failed | Evidence |
| --- | --- | --- | --- |
| C1 | 4 | none | On call tonight, weeks, Birth Plan read status, Doula gap: domain-only elements; no refuse-list scaffolds. |
| C2 | 4 | none (cross-screen scale unverified) | Hanken Grotesk self-hosted, h1 31.4/600, h2 25.0/600; money and dates `tabular-nums`; prose 663px = 65ch; nothing overflows at 320. |
| C3 | 4 | none | Committed blue/amber in both themes; selection, caret, scrollbar-color, underline offset 3.2px set; dark contrast 7.3–13.0. |
| C4 | 3 | alignment/spacing | Activity filter: "All" left 95, next link 159 (≈42px gap) vs 20–21px between the others (300, 485): an uneven row of one control. |
| C5 | 4 | none (DESIGN.md signature record not re-read) | Activity ledger the only signature; no motion; nothing ornamental. |

**Total 19/20.**

## 4. Findings (ranked by harm to a person's job)

1. **9c fail: a refused "Load more" prints the BFF's own message.** With the Activity's next page refused, the rows stay (30 → 30) but beside "Try again" the text is the response's `message` verbatim; a real 500 carries `apierr.MsgInternalError` = "internal error", lowercase BFF words, not Voice. Cause: `paginatedList.svelte.ts:215` prefers `error_.message` (from `apiErrorMessage`) over `failureMessage`. Fix: show `failureMessage` ("We could not load more Activity. Try again.") for a further page's failure, as the first page already does; apply to Visits and Messages.
2. **C4: uneven gap after "All" in the Activity filter** (≈42px vs 20px). Fix: give the current filter the same box as the others (no extra padding/margin on the `aria-current` item).
3. Not scored, worth fixing: Phone fact at 743px, 3px below the 375x740 fold (Priya act 2); Escape does not close "Record a payment" and its Continue/Cancel sit below the fold at 1280x800 (Dee acts 4, 7); "Add a Visit" asks only a date, while Priya reached for Type and Notes (act 10); Cancel discards typed notes/messages without a word; every Activity line says "for the Client" on one Client's record; at 1280 heading navigation meets "Details" before "On call".

## 5. Expected and missing, against the Category baseline

| My expectation | Baseline | Verdict |
| --- | --- | --- |
| 1. Overflow "Actions" menu (Stripe, Linear) | #28 Declined | Holds: acts sit beside the fact they change, GOV.UK has no overflow pattern; a Stripe-trained reader finds each act one place over. |
| 2. Previous/next record (Linear, Gmail) | #17 Declined, "no job on record" | Holds weakly: Renata working the Overview's waiting Clients would use it; Back returns to the list. |
| 3. Keyboard shortcuts (Linear, GitHub) | #16 Later (#2956) | Deferred to a founder decision; fine. |
| 4. Copy link/ID (Stripe, Linear) | #15 Declined | Holds: address bar. |
| 5. Client's address with a map link (Jane App, HubSpot) | Not in the baseline | Missing. Camille's thread says "I will come to you first"; an on-call Doula leaving for a home labor needs it. It exists on the Client page ("Where Morgan lives", Lena act 4), one link away via "View Client". Add it to Details or beside Phone. |
| 6. Attach a file (HubSpot, HoneyBook) | Not in the baseline | Messages compose has "Attachment" (Priya act 12); partial. |
| 7. Tasks/reminders (HubSpot) | Not in the baseline | Missing; no job on record that I found. |
| 8. Internal note (HubSpot, Linear) | #29 Build | Built as the Care Plan ("Camille never sees the Care Plan") and Visit notes. |
| 9. Create an Invoice (Stripe) | Not in the baseline | Correctly absent on Camille's (all billed). |
| 10. Activity filter by person (Stripe, HubSpot) | #9b Build (by kind only) | Kind built and BFF-served; by person not in the baseline. |
