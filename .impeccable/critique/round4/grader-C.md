# Grader C, round 4: the Engagement record

## 5 (written first). Expected and missing, before reading the surface brief

Written after the grader brief, the rubric's jobs, and one look at Camille's record at 1280x800 (grader-C-1280.png). Each: what I expected, the product that taught it, what I saw.

1. Previous/next record without going back to the list (j/k, arrows) — Linear issue view, Gmail, Stripe Dashboard detail pages. Not on the screen.
2. Copy a value in one action (email, phone, ID) — Stripe Dashboard customer page (copy buttons on ID/email). Not seen; email/phone are links only.
3. A command palette / keyboard shortcuts / global search from the record — Linear (Cmd+K), Stripe (/ search). Top bar has no search.
4. Breadcrumb trail (Practice > Clients > Camille) — GOV.UK breadcrumbs, Stripe. A single back link "Clients" instead.
5. Overflow "Actions" menu in the header for record-level acts (edit, archive) — Stripe customer "Actions", HubSpot record. Record-level changes sit in the Details column instead.
6. Status shown as a pill beside the title — Stripe (customer/invoice status), Linear status chip, GOV.UK tag. Care phase is plain text "Care phase Active".
7. Internal pinned note on the person — HubSpot / Practice Better client notes. Care Plan partly plays this role.
8. Files/attachments on the record — Practice Better, Jane App client chart. Not seen.
9. "View as the Client sees it" / portal preview — Practice Better, Jane App. Not seen.
10. Add a Visit to my calendar / ICS — Jane App, Calendly. Not seen.
11. Lifetime money summary (paid to date, total) — Stripe customer page "Total spend". Partly: Invoices line says "all is billed"; no paid-to-date total.
12. One timeline merging messages, visits and changes — HubSpot / Stripe event timeline. Activity is separate from Messages (Activity present, filterable).

Live work: own page (pageId 12, marked `__grader='C'`, re-marked after each navigation via initScript), signed in as Renata, never landed on /login. Shots: grader-C-1280.png, grader-C-375.png, grader-C-1280-dark-invoices.png. T10 simulated by an initScript that returned 500 for one read on my page only (Invoices, then Visits, then the Engagement read); nothing saved, sent or deleted.

## 1. Usability (/40)

| # | Score | Failed checks | Evidence for passes |
|---|---|---|---|
| 1 Visibility | 4 | none (a unverified: skeleton timing per brief; no-shift not measured by me. d unverified: no network diff taken) | b: Camille header "Care phase Active / Due Dec 6, 2026, 37 weeks / Doulas Priya Raman, Bethany Kroll" before Jump to; Tamsin "Doula No Doula yet"; Imani no Due line. c: fact "On call tonight: Nobody". c': header warning Badge "Nobody on call tonight" (amber), fact plain "Nobody", On call line "Nobody on call tonight: Priya Raman cannot be reached until Mon Nov 16, 10:00am." e: no error/warn in console after reload. |
| 2 Real world | 4 | none (b for Mira/Imogen unverified) | a: innerText has 0 "engagement" matches, no aria-label/title with it. b: Camille 37, Tamsin 36, Grace 28 weeks, Imani none. c: every Activity row a `<time datetime="2026-11-16T09:27:18…-05:00">`, "29 minutes ago", "Wednesday, 8:30am"; older "Oct 26, 2026, 10:30am" (Dee act 10). e: "Signed", Grace "Sent, waiting for the Client to sign". |
| 3 Control | 4 | none (a's filter-kept and the On call/Contracts origins unverified; d's focus move unverified) | b: Cancel discards and closes (Priya acts 9, 11, 14; Dee act 5; my compose Cancel). c: birth-plan `backHref` = record `#birth-plan`, label the Client's name (birth-plan/+page.svelte:59). d: page 5181px at 1440, no section over 900px (max Visits 619), one "Back to top" at the end → `#jump-to`. e: top bar "Clients" drawn current, back link "Clients" → /clients. |
| 4 Consistency | 3 | f (T8: heading order) | a: at 1440 Visits/Invoices/Activity tables 1024px, their max-content 1912/1160/1213 (so ≤ max-content+32), all right edges 1060 = main column; state line, Notice, thread 663px = 65ch. b: facts column top 390 = Jump-to top 390 at 1440, beside at 1280 (shot); `@container record-body (min-width: 73rem)` in RecordDetail.svelte:830, no @media. c: `facts?: Snippet` in RecordDetail; EngagementScreen sets no grid. d: Imani "$950.00 overdue since Nov 4" red (oklch 0.506 0.193 28), Camille Owed neutral badge, amber for unanswered and nobody on call. e: baseline rows built or declined soundly (see 5). g: skip link → Clients → Message the Client → On call → Birth Plan = 4 stops (budget 5); focus ring 2px solid oklch(0.4 0.11 258) offset 2px; payment form takes focus on its heading and Cancel returns it to "Record payment" (Dee acts 4–8); compose Cancel returns to "Write a message". Cross: 320px scrollWidth 310, no control under 24px. |
| 5 Error prevention | 4 | none (b partly unverified: Priya's log does not say whether Record payment/Void/Write off were drawn for her; T7's "address tells her nothing" half unverified) | a: header holds only "Message the Client"; "Void this Contract" is a secondary Button after WarningText, under h3 "Void this Contract". c: Lena saw only her own $450.00, Invoices with dates/status and no amounts, "Request void" (hers). |
| 6 Recognition | 4 | none (d on Jamie Bishop unverified) | a: all eight T2 facts on the 1280×800 first screen with labels (Messages fact bottom ≈752); links name fact and section; phase/due/Doula unlinked. b: header "Message the Client" → compose, focus in textarea "Message"; "Add a Visit" is Visits' first control. c: Birth Plan "4 of 6 questions answered · last changed Nov 2, 2026 by Priya Raman · Camille has not read it yet"; Contract "Signed on Sep 21, 2026"; Invoices "$750.00 owed, due Nov 30." d: Camille, Grace (Contract, Visits, then empties; On call before its days counts empty), Imani (Visits, Invoices, Birth outcome, Contract, then empties) all hold. |
| 7 Efficiency | 4 | none (save-refresh halves of T3/T4/T5 unverified by me; 200% zoom unverified) | T1–T5 pass, see table 2. 320px: no horizontal scroll. |
| 8 Aesthetic | 4 | none (d's 375 height not measured; Priya act 3 had the answer on screen) | a: On call heading, state line and Priya's row all above 800 at 1280 (shot). b: 375 first screen holds header facts, badge, Jump to with Birth Plan, On call tonight fact (shot). c: 5181 ≤ 5400 at 1440. d: Birth Plan section 530px, Care Plan 395px with "Show the whole Care Plan". e: Offers/Birth outcome 151px = heading, line, button. f: secondary outline button (dark shot). h: Message, Jump to, facts group = 3. i: Jump to 2 lines at 1280, 4 at 375. |
| 9 Error recovery | 4 | none (b, c unverified: would need a refused save / Load more) | a: T10 pass. |
| 10 Help | 4 | none | a: T6 pass. b: Grace "Invoices start once Grace signs the Contract."; "The Birth Plan is not started yet. Start the Birth Plan"; Imani "No Birth Plan. This is postpartum care." c: WarningText says what voiding does before the button. d: no walker asked what a term meant. |

**Total: 39/40**

## 2. Tasks

| Task | Result | Actions | Evidence |
|---|---|---|---|
| T1 | pass | 1 | Priya act 3: Jump to "Birth Plan" → heading at 76px, "No epidural unless I ask twice." on screen; link bottom at 467px at 320×640 (measured), on first screen at 375 (shot); "Read or print the whole Birth Plan" 1 action on. |
| T2 | pass | 0 | Camille 1280 shot + snapshot: all eight facts; Imani "$950.00 overdue since Nov 4"; Grace "Sent, waiting for the Client to sign", "Nothing owed". |
| T3 | pass | read 2, write 3, add 2 | Jump to Visits → "Last Visit: Mon Nov 2" → notes; Change → caret in Notes (Priya acts 6–7). Add a Visit opens with focus on its heading, the one field directly below (act 10). Save-refresh: unverified by me (orchestrator's earlier-build evidence only). Note: Add a Visit has no Notes/Type field, which Priya reached for. |
| T4 | pass | 1 | Measured at 375: header "Message the Client" → focus in textarea "Message" (398–498px) with Camille's "move Thursday's visit" (242–292px) on the same screen; "Unanswered since Friday, 9:22am" read at a glance first. Send half unverified. |
| T5 | pass | 0 / 2 | Dee acts 2–4: owed and due read on arrival; Owed fact → Record payment. Void sits after its warning, confirm step separate. Save half unverified. |
| T6 | pass | 1 each | Contract section "Sent on Sep 14, 2026 by Renata Alvarez" (Contract fact link); Activity first rows "Put on this work as the Doula: Bethany Kroll — Renata Alvarez", "Gap in on-call cover added for the Client — Renata Alvarez"; filters are links (`?activity=money`) served by GET activity?kind=money (Dee act 10). Load more in place unverified. |
| T7 | pass (one half unverified) | 2 | Lena acts 5–8: own fee $450.00 and terms in Contract; Invoices with no amounts; no other money. Another Engagement's address not tried. |
| T8 | **fail** | — | h1 "Camille Boyd", facts follow with labels, links "Nobody, On call tonight, in On call", "Call Camille Boyd, (585) 555-0142", Changes described by row. But heading order is Camille Boyd → **Details** → On call → Birth Plan…, while at 1440 Details is drawn at top 858px in the right column, below On call (460) and beside Birth Plan (1089). |
| T9 | pass | 4 stops | Measured: skip → Clients → Message the Client → On call → Birth Plan; ring 2px solid. Escape does not close the payment form (Dee act 7), not a rubric check. |
| T10 | pass | — | Invoices 500: "We could not load the Invoices. Try again", Owed fact "Could not be read", Contract still drawn. Visits 500: "We could not load the Visits. Try again", Next Visit "Could not be read". Engagement 500: "Sorry, there is a problem / Something went wrong on our end… / Go to your Practice overview". |

## 3. Craft (/20)

| C | Score | Failed | Evidence |
|---|---|---|---|
| C1 Specificity | 4 | none | Weeks beside due, "Nobody on call tonight: Priya Raman cannot be reached until…", Birth Plan answer on the record, contractor's "Your fee… the fee you accepted": no generic admin has these. No card grid, hero metric, gradient, side border. |
| C2 Typography | 3 | the face | One face, Hanken Grotesk, for h1 (35px/600) and body (17.9px): a neutral grotesque that reads as a platform sans; no display voice. Steps clear (35/27.8/22.2/17.9, ~1.25), tabular-nums on money, prose 663px = 65ch. Same-role-on-3-screens unverified. |
| C3 Color | 4 | none (full contrast pass unverified; tokens not computed) | One blue for links, amber act, red overdue, neutral owed; ::selection oklch(0.376 0.042 256), caret tinted, scrollbar-color set, underline offset 3.58px; dark theme coherent (shot). |
| C4 Rhythm | 3 | alignment/density in Invoices table | Dark shot: INV-0001's row runs 3 lines ("Deposit, / bank / transfer" in a ~70px Note column) while Amount/Status have spare room; the Status cell repeats "by bank transfer" that Note also says. Facts top = Jump-to top (390); first screen holds the job at 1280 and 375. |
| C5 Signature | 3 | signature not recognizable | DESIGN.md:171 names the activity ledger; on this page it is a plain When/What/Who table indistinguishable from any audit log, last on the page. No motion seen; chrome quiet. |

**Total: 17/20**

## 4. Findings (ranked by harm to a person's job)

1. **T8 / 4f — Details heading precedes On call for a screen reader.** At wide widths the facts column's Details (`complementary` h2) comes before every section in heading order but is drawn below On call. A screen-reader user pressing H to get to On call hits Details first. Fix: in `RecordDetail`, put the Details block after the sections in DOM and place it in the facts column by grid area (it already is the last block in one column), or move Details out of the column-one slot so only the unlabeled at-a-glance list precedes the sections.
2. **(not a check) Add a Visit has only a date field**; Priya reached for Type and Notes (act 10). Fix: add Type and Notes to the add form, focus the first field.
3. **(not a check) Cancel throws away typed text with no warning** (Priya acts 9, 14). Fix: keep the draft or ask before discarding, per GOV.UK's "check before you lose work".
4. **C4 — Invoices table cells wrap to 3 lines** while columns have slack, and payment method is said twice (Status sub-line and Note). Fix: drop "by bank transfer" from the Status sub-line or the Note, and give Note `grows` so it takes the slack.
5. **(not a check) Record payment form**: Continue/Cancel below the fold at 1280×800, Escape does nothing (Dee acts 4, 7). Fix: make Escape cancel; tighten the form so its buttons land on screen.
6. **C2 — no display voice**: one neutral grotesque for everything. Fix: the direction's call — a display cut or second face for h1/h2, recorded in DESIGN.md.
7. **C5 — ledger not recognizable as a signature.** Fix: give the ledger one authored visual trait (e.g., actor and time set as a consistent leading column with the kind tag), or record in DESIGN.md that the ledger is distinguished by content only.
8. **(not a check) Tamsin's "No Doula yet" appears twice** (header and On call tonight fact). Fix: On call tonight could say "Nobody, no Doula yet" or drop to "Nobody".

## 5. Expected and missing, against the Category baseline

| My expectation (source) | Baseline | Verdict |
|---|---|---|
| 1. Previous/next record (Linear, Gmail, Stripe) | row 17, Declined | "no job on record; Back returns to the list." Holds weakly: Renata at a 14-Doula agency walks Clients one after another; Linear readers will press j/k. Not unsound enough to fail. |
| 2. Copy a value (Stripe) | row 15 / 14, Declined | "no job on record; the address bar holds it." Holds for IDs; email/phone are mailto/tel links. Holds. |
| 3. Command palette / shortcuts / search (Linear, Stripe) | row 16, Later (#2956) | Deferred to a whole-app decision. Holds for this screen. |
| 4. Breadcrumbs (GOV.UK, Stripe) | row 21b, Declined | Back link names the list, top bar marks it. Holds. |
| 5. Overflow Actions menu (Stripe, HubSpot) | row 28, Declined | Acts beside their facts, GOV.UK has no menu. Holds. |
| 6. Status pill by title (Stripe, Linear, GOV.UK tag) | row 1, Build | Built as labeled text, plus warning Badge for nobody on call. Holds. |
| 7. Internal pinned note (HubSpot, Practice Better) | row 29, Build | Built: Care Plan, "Camille never sees the Care Plan". |
| 8. Files/attachments on the record (Practice Better, Jane) | not in baseline | Missing; message compose has Attachment, the record has no file list. Nominated. |
| 9. View as the Client sees it (Practice Better, Jane) | not in baseline | Missing. Nominated. |
| 10. Add a Visit to my calendar (Jane, Calendly) | not in baseline | Missing. |
| 11. Paid-to-date total (Stripe customer "spend") | not in baseline (row 4 is owed only) | Partly: Invoices line says "all is billed"; no paid total. |
| 12. One merged timeline (HubSpot, Stripe) | rows 9, 20 | Activity has money events; Messages separate. Holds. |
