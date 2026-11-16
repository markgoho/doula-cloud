# Grader B, round 2 (commit f4cacd20)

## Expected and missing (written before reading the surface brief)

From the brief, the rubric's jobs, and one look at Camille's record at 1280×800:

1. Client's phone number in the header, tap-to-call on a phone (Healthie, Practice Better, HubSpot contact record: phone and email beside the name). Only an email, down in Details.
2. A "More actions" / overflow menu on the record (Stripe Dashboard customer page "…", Linear issue "…"). Acts (due date, care phase, Doulas) are in Details instead.
3. Keyboard shortcuts / previous-next record (Linear j/k, Gmail). None.
4. A sticky or condensed header when scrolled, so the name stays in view (Linear, Stripe). None observed.
5. Files / attachments on the record (Healthie and Practice Better client documents). None.
6. Search or filter within Activity by text (Stripe events list filters). Kind filters only.
7. A breadcrumb (GOV.UK breadcrumbs, Stripe "Customers > name"). Back link only.
8. Internal note / pinned note on the record (HubSpot notes, Stripe customer "Notes"). Care Plan may stand in.

## 1. Usability (/40)

| # | Score | Failed | Evidence for passes |
|---|---|---|---|
| 1 | 3 | a (no-shift half) | a: rAF trace on load, no throttling: On call `<h2>` top 449px at 376ms → 483px at 419ms (34px; "Jump to" grows from 1 to 2 lines as late links land); facts top 381→382. Skeleton timing unverified (Slow 4G). b: header "Care phase Active · Due Dec 5, 2026, 37 weeks · Doulas Priya Raman, Bethany Kroll" above Jump to. c/c': amber Badge "Nobody on call tonight" in header, fact "On call tonight Nobody", On call line in words. d: `/visits` serves `next` (Nov 18 11:00), `/contract/invoices` serves `owed {owedCents 75000, nextDueOn 2026-11-29}`, matching the facts. e: console empty on load. |
| 2 | 4 | — | a: case-insensitive find of "engagement" in innerText + aria-label/title: none. b: Camille 37, Grace 28, Imani and Mira none (Tamsin, Imogen unverified). c: Activity "5 minutes ago"/"Tuesday, 8:30am", older "Oct 25, 2026, 10:30am", each `<time datetime="2026-10-25T10:30:00-04:00">`. e: Grace "Sent, waiting for the Client to sign". |
| 3 | 4 | — | a: from On call list, back link reads "On call" (filter kept unverified). b: compose Cancel closed it, nothing sent. c: Birth Plan page back link "Camille" → `#birth-plan`. d: 5512px page, "Back to top" at end → `#jump-to` (`tabindex=-1`); no section >900px. e: from On call, top bar `aria-current` on "On call"; from Clients on "Clients". |
| 4 | 4 | — | a (1440): Visits 1024 ≤ max-content 1208, Invoices 1024 ≤ 1116, Activity 661 = 661, Notice 657 and thread 657 ≤ 65ch (657px). b: facts column left 975 (1280)/1102 (1440), top 382 = Jump to top 382; `@container` in RecordDetail, no `@media` in route. c: facts a RecordDetail region; EngagementScreen style sets no grid for it. d: Imani overdue red `oklch(0.506 0.193 28)` + icon; Camille owed neutral; amber for nobody on call and unanswered. f/g: T8, T9 pass. Cross-cutting: 320×640 and 640×900 (standing for 200% at 1280): no h-scroll, nothing past the edge, no target <24px; dark contrast muted 7.33, link 9.13, warning 6.95. e: see section 5 (I name attachments; one grader cannot fail it). |
| 5 | 4 | — | a: header holds only "Message the Client"; "Void this Contract" in Contract, after WarningText, with a "Void this Contract?" confirm. b: Priya sees money, which ADR-0008 grants an employee. c: T7. |
| 6 | 4 | — | a: all T2 facts on first screen at 1280×800 with labels, lowest (Messages) bottom ~710. b: header move opens compose with focus in TEXTAREA "Message"; Add a Visit first control in Visits. c: Birth Plan "4 of 6 questions answered · last changed Nov 1, 2026 by Priya Raman · Camille has not read it yet"; Contract "Signed on…"; Invoices "$750.00 owed, due Nov 29." d: Camille, Grace, Imani hold (empties below filled); Jamie Bishop unverified. |
| 7 | 3 | T3 | T1, T2, T4, T5 pass; T3 fails (below). |
| 8 | 3 | c | a: On call heading 483, state line, Priya row ~700 ≤800. b: 375 first screen: header facts to 400, Birth Plan link 445–470, On call tonight fact 573–598. **c FAIL: 5512px at 1440×900 (cap 5400).** d: Birth Plan answers capped 471px (max 512) at 375; Care Plan capped 447px with "Show the whole Care Plan". e: Offers 149px, Birth outcome 151px. f: Void `secondary`, transparent fill. g: no header/fact repeat beyond the exempt on-call alert. h: 3 (Message the Client, Jump to, facts group; PR-G10's count). i: Jump to 2 lines at 1280, 4 at 375; every link a section. |
| 9 | 4 | — | a: T10 pass. b, c unverified (saving forbidden). |
| 10 | 4 | — | a: T6 pass. b: Grace "Invoices start once Grace signs the Contract.", "The thread opens once Grace has accepted the portal invite."; Imani "No Birth Plan. This is postpartum care."; Mira "No Birth Plan. The birth outcome is recorded." c: warning text before confirm. d: no term asked in acts-priya. |

**Total 37/40.**

## 2. Tasks

| T | Result | Actions | Evidence |
|---|---|---|---|
| T1 | pass | 1 | acts-priya act 3; measured after Jump to → Birth Plan at 375×740: answer at 623–673px. Link on first screen at 375 (bottom 470) and 320×640 (467). Full plan page has Print + PDF. |
| T2 | pass | 0 | B-camille-1280.png; Imani "$950.00 overdue since Nov 3"; Grace "Sent, waiting for the Client to sign", "Nothing owed". |
| T3 | **fail** | ≥3 to read | At 375×740, after Jump to → Visits the last logged Visit's notes ("Third prenatal visit…") sit at 1445–1569px, ~2 screens below the fold behind "Coming up" and "Not scheduled yet" cards; acts-priya act 4 scrolled and lists "Earlier Visits … far down the phone page". Add a Visit in 2 (button at 165px after the jump): passes. Save-in-place from orchestrator evidence. |
| T4 | pass | 1 | "Message the Client" → focus in TEXTAREA "Message"; Camille's Nov 12 Message visible above it (1280). Unanswered fact read before opening. Send-in-place from orchestrator evidence. |
| T5 | pass | 0 / 2 | acts-dee acts 2–4 (round 1 build); Void secondary after WarningText (measured on this build). |
| T6 | pass | ≤1 each | Contract section "Sent on Sep 13, 2026 by Renata Alvarez"; Activity → "Contract and money" (URL `?activity=money`, BFF-served) "Sep 13, 2026, 10:30am Contract sent to the Client — Renata Alvarez"; Bethany via "Doulas and on call" (acts-dee act 8); gap "5 minutes ago Gap in on-call cover added… Renata Alvarez" in All. |
| T7 | pass (partly unverified) | — | acts-lena (round 1) + orchestrator: own $450 fee only, Invoices without amounts. Whether "Change" on Marisol's Visits and "Record a time a Doula cannot be reached" are acts the BFF allows her: unverified. |
| T8 | pass | — | snapshot: h1 "Camille Boyd", labelled facts follow; h2s in drawn order; fact links "Signed, Contract, in Contract"; Change described "on-call days for Priya Raman"; states in words. |
| T9 | pass | 4 stops | Skip → Clients → Message the Client → On call → Birth Plan (budget 5); focus outline solid 2px; form focus and Cancel return per acts-priya 5–7. |
| T10 | pass | — | Refused via initScript fetch wrapper: Invoices → "We could not load the Invoices. Try again", Owed fact "Could not be read", others stand; Visits → same, Next Visit "Could not be read"; Engagement read 500 → "Sorry, there is a problem … Go to your Practice overview". |

## 3. Craft (/20)

| C | Score | Failed | Evidence |
|---|---|---|---|
| C1 | 4 | — | "Nobody on call tonight", "37 weeks", the Birth Plan's pain-relief answer, the on-call gap: a doula practice's record, no Refuse-list scaffold. |
| C2 | 3 | scale steps | Hanken Grotesk self-hosted, headings 35/22px, tabular-nums on td and dd. Body text at two sizes 0.7px apart (facts dd 17.9px, table td 17.17px): not an obvious step. Cross-screen roles unverified. |
| C3 | 4 | — | amber/red/blue one meaning each; `::selection`, caret, scrollbar-color, underline offset 3.58px set; dark contrast 6.95–9.13. |
| C4 | 3 | composition | At 1280 the main column is 903px but its prose and rows stop at 647–678px; facts start at 975px, a ~300px empty band through the first screen (B-camille-1280.png); three right edges in the main column (647 prose, 808 h3, 903 tables). Facts top aligns with Jump to (382/382). |
| C5 | 4 | — | Activity ledger the one signature; no motion seen; nothing ornamental. |

**Total 18/20.**

## 4. Findings (ranked by harm to a person's job)

1. **T3 (7): Priya cannot read the last Visit's notes without ~2 screens of scrolling on a phone.** Fix: put the last logged Visit's date and first line of notes in the Visits state line ("Last: Sun Nov 1, Third prenatal visit…"), or order Earlier above Not scheduled yet, so Jump to → Visits shows them.
2. **Mira Castellanos (after a loss): the empty Visits section ("No Visit scheduled.") stands first, above the filled Birth outcome and Contract.** Outside 6d's four named records so not scored, but it breaks 6d's rule and puts "Add a Visit" first on a loss record. Fix: apply empty-below-filled to the loss phase.
3. **1a: 34px shift at load** — On call heading 449→483px as the last reads land and Jump to wraps to a second line. Fix: draw all Jump to links from the skeleton (the section list is known from the Engagement read), or reserve two lines.
4. **8c: 5512px at 1440×900, cap 5400.** Fix: recover ~120px (shorter "Not scheduled yet" row, shorter void WarningText, tighter Contract section).
5. **C4: dead band at 1280** between measure-capped content and the facts column. Fix: cap the main column at the measure where it holds no table, or move the facts column to the measure's edge.
6. **C2: two body sizes 0.7px apart.** Fix: one body-size token for dd and td.
7. Phone (baseline row 27) is built only in Details, the page's last block on a phone (~9000px down at 375). For Priya calling from a corridor, put a present phone as a `tel:` link in the header line.

## 5. Expected and missing vs Category baseline

| My item | Source | Baseline verdict |
|---|---|---|
| Phone in header, tap-to-call | Healthie, Practice Better, HubSpot | Row 27 Build: built as a `tel:` row in Details when a phone is on file (Camille has none). Placement weak for the corridor reader (finding 7). |
| Overflow "More actions" menu | Stripe, Linear | Row 28 Declined: holds; acts sit beside their facts, GOV.UK has no overflow pattern. |
| Keyboard shortcuts / next-prev | Linear, Gmail | Row 16 Later (#2956), row 17 Declined: holds; Back returns to the list. |
| Sticky/condensed header | Linear, Stripe | Row 7 Declined: holds per GOV.UK's sticky-element findings; Back to top built. |
| Files / attachments | Healthie, Practice Better | **Not in the baseline.** Contract and Birth Plan PDFs exist; no Client documents. I name it missing. |
| Text search in Activity | Stripe events | Not in baseline (row 9b builds kind filters). Low harm at tens of rows. |
| Breadcrumb | GOV.UK, Stripe | Row 21b Declined: holds; back link names the list, top bar marks it (verified). |
| Internal note | HubSpot, Stripe | Row 29 Build: the Care Plan, "Camille never sees the Care Plan": built. |

Unverified: 1a skeleton timing; 3a filter kept; 2b Tamsin/Imogen; 6d Jamie Bishop; 9b; 9c; T7's Visit-Change/unreachable controls; C2 cross-screen roles; the 375 screenshot (capture timed out twice; measured by script).

Screenshots: B-camille-1280.png, B-camille-1280-dark.png.
