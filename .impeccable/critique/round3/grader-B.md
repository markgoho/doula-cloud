# Grader B — Engagement record, round 3 (live at 4b6f64aa)

## 5. Expected and missing (written BEFORE reading the surface brief)

Looked at Camille's record at 1280×800 and 375×740 (page 30, `__grader='B'`), listed every visible control.

1. **Create a new Invoice from the record** — Stripe Dashboard customer page ("Create invoice" in the customer header), Practice Better / Jane client profile ("New invoice"). Invoices section has row acts (Record payment, Void, Write off, Refund, Reverse) but no "Create/Send an Invoice".
2. **Next / previous Client without going back to the list** — Linear issue view (J/K, up/down arrows in header), Gmail, HubSpot record pager. Absent.
3. **Overflow "more actions" menu in the header** (copy link, archive, etc.) — Linear "…" menu, Stripe "Actions" menu, GitHub issue. Absent (header holds only "Message the Client"; by design per 5a/6b).
4. **Client's email at a glance beside the phone** — Stripe customer sidebar (email first), HubSpot contact left rail (email, phone). Phone is at a glance; email only in Details at the page's end (~8900px on phone).
5. **Persistent section nav when scrolled** (sticky tabs/anchor rail) — HubSpot record tabs, GOV.UK "contents" pattern on long pages + Stripe's sticky page header. "Jump to" scrolls away; "Back to top" links substitute.
6. **Files / documents attached to the Client** (intake forms, uploads) — Practice Better, Jane App chart "Files", HubSpot attachments. No section.
7. **Keyboard shortcuts** — Linear (C, E, ⌘K). None found.
8. **Copy the record's link / ID** — Stripe (copy ID), Linear (⌘⇧, copy URL). Absent.

### Against the brief's Category baseline (read after the list above)

| # | Mine | Baseline | Verdict |
|---|---|---|---|
| 1 | Create an Invoice from the record | not in baseline | On Camille's, "all is billed, so there is nothing left to invoice", so absence is right there; whether a Contract with unbilled money shows a raise control not checked. |
| 2 | Next/previous Client | row 17 Declined: "no job on record; Back returns to the list" | Holds, weakly: a Linear-trained owner working the "waiting on a reply" queue would reach for it, but no job asks. |
| 3 | Overflow "Actions" menu | row 28 Declined: "a menu puts each act behind a disclosure, two actions where there is one" | Holds; GOV.UK has no such pattern and every act sits beside its fact. |
| 4 | Email at a glance | row 27 Build | Built: phone is a `tel:` fact; email a `mailto:` in Details (facts column at desk; page end on phone). |
| 5 | Sticky section nav | row 7 Declined (GOV.UK sticky-element zoom/focus faults) | Holds; "Back to top" → Jump to substitutes (1 at 1440, 5 at 375). |
| 6 | Files/documents | not in baseline | Practice Better/Jane readers expect it; no job names it. |
| 7 | Keyboard shortcuts | row 16 Later (#2956) | Holds (app-wide decision). |
| 8 | Copy link/ID | row 15 Declined: "the address bar holds it" | Holds. |

No decline unsound; no baseline Build row missing → 4e passes from this grader.

## 1. Usability (/40)

| # | Score | Failed | Evidence for passes |
|---|---|---|---|
| 1 | 4 | — (a unverified per brief; d unverified, network not inspected) | b: header dl "Care phase Active / Due Dec 6, 2026, 37 weeks / Doulas Priya Raman, Bethany Kroll" before Jump to; c/c': amber Badge "Nobody on call tonight" in header, fact "On call tonight Nobody", On call line in words; e: console empty on load |
| 2 | 4 | — (b: only Camille 37 / Imani none verified) | a: innerText + aria-label find /engagement/i → 0; c: "4 minutes ago", "Tuesday, 8:30am", 5/5 Activity rows in `<time datetime="…-05:00">`; e: "Signed on Sep 20, 2026. Sent on Sep 13, 2026 by Renata Alvarez." |
| 3 | 4 | — (a filter-kept, c, e aria-current unverified) | a: back link "Clients"; b: compose, Record payment, Void confirm Cancel change nothing; d: 5158px at 1440, no section >900px, "Back to top" at page end → #jump-to |
| 4 | 1 | a, f, g (c unverified) | b: facts top 390 = Jump to top 390 at 1440, beside at 1280, stacks at 640 (facts 438, On call 708); d: Imani "$950.00 overdue since Nov 3" red oklch(0.506 0.193 28), Camille owed-not-due neutral, amber Badge; 320/640 scrollWidth ≤ innerWidth, no target <24px |
| 5 | 4 | — (b, c unverified live; Lena log is round-1 build) | a: no destructive act in header; Void opens "Void this Contract?" with WarningText, focus on Cancel |
| 6 | 3 | b | a: 8 T2 facts + Phone on first screen at 1280×800, labelled, each linked to its section, header facts unlinked; c: Birth Plan "4 of 6 questions answered · last changed Nov 1, 2026 by Priya Raman · Camille has not read it yet", Invoices "$750.00 owed, due Nov 29."; d: Camille passes (Imani's order seen, not settled) |
| 7 | 4 | — | T1–T5 within budget; 320 and 640 no horizontal scroll |
| 8 | 4 | — | a: On call h2 ~500, state 553, first row ~700 at 1280×800; b: 375 first screen holds header facts, Jump to, On call tonight; c: 5158 ≤ 5400; d: Birth Plan 726px at 375, Care Plan 447px; e: Offers/Birth outcome heading+line+button; f: Void transparent secondary after Warning, red only on confirm; g: no repeats bar the on-call alert; h: 3 decisions; i: Jump to 2 lines at 1280, ~4 at 375 |
| 9 | 4 | — (b, c unverified) | a: offline Activity filter → "We could not load the Activity." + "Try again" in place, others stand; facts return "Could not be read" per refused read (EngagementScreen.svelte:1420–1545); unknown Engagement → "Page not found … Go to your Practice overview" |
| 10 | 4 | — | a: T6; b: "Nobody has been offered this work yet." + Offer this work, "Recorded after the birth."; c: void warning before confirm; d: no walker asked a term |

**Total 36/40.**

## 2. Tasks

| T | Result | Actions | Evidence |
|---|---|---|---|
| T1 | pass | 1 | Priya r3 act 3; Birth Plan link bottom 467 at 320×640 |
| T2 | pass | 0 | grader-B-1280.png; Imani overdue live |
| T3 | pass | 2 / 3 / 2 | Priya r3 acts 6–8, 11; save-in-place from orchestrator evidence |
| T4 | pass | 1 | header "Message the Client" → TEXTAREA "Message" focused; "Unanswered since Thursday, 9:22am" at a glance |
| T5 | pass | 1–2 | Dee r1 acts 3–5; Void secondary + asks (live) |
| T6 | pass | ≤1 each | Dee r1 acts 6–8 (`GET activity?kind=…`); gap change by Renata on All at 0 actions |
| T7 | unverified | — | only round-1 Lena log |
| T8 | **fail** | — | at 1280 Details is `<aside>` with `<p class="aside-title">Details</p>`, no h2 (it is a Jump to target, an h2 in one column); Phone link "(585) 555-0142, Phone, call" does not say it calls the Client |
| T9 | **fail** | — | skip→Clients→Message the Client→On call→Birth Plan = 4 stops ✓; 2px solid ring ✓; compose takes focus, Cancel returns it ✓; **Record payment opens its form and focus stays on the button** (keyboard Enter, twice) |
| T10 | pass | — | as heuristic 9 |

## 3. Craft (/20)

| C | Score | Failed | Evidence |
|---|---|---|---|
| C1 | 4 | — | On call tonight, weeks, Birth Plan answers, "Camille never sees the Care Plan"; no cards/eyebrows/hero metrics |
| C2 | 4 | — (3-screen sameness unverified) | Hanken Grotesk 600, h1 35px / h2 27.8px; tabular-nums in facts and Invoices; prose at 65ch |
| C3 | 4 | — | ::selection, caret, scrollbar-color (base.css:155–163); underline offset 3.55px; amber act / red overdue / blue link; dark at 640 legible (grader-B-640-dark.png) |
| C4 | 3 | one edge | at 1440 Visits/Invoices/summary rows end at 1060, Activity ledger at 697, header dl at 728: 363px empty band beside the signature |
| C5 | 4 | — (motion unverified) | DESIGN.md:171 names the activity ledger; nothing ornamental |

**Total 19/20.**

## 4. Findings (ranked)

1. **T9/4g: Record payment's form does not take focus.** Fix: focus the form's first field (or legend, tabindex=-1) on open, as compose and Visit Change do.
2. **6b: "Last Visit: Sun Nov 1" link precedes "Add a Visit" in Visits.** Fix: Add a Visit directly under the state line, Last Visit link after it.
3. **T8/4f: Details has no heading at desk widths; Phone link does not name the Client.** Fix: `<h2>` in the aside; name "Call Camille Boyd, (585) 555-0142".
4. **4a/C4: one right edge.** Activity table 661px ends at 697 vs 1060. Fix: let the ledger's What column grow to the column. Rubric note: "≤ max-content+32" and "one right edge" clash for any table narrower than the column.

Observations (no check failed): void WarningText printed twice verbatim; Visit Cancel discards typed notes without asking (Priya act 10); Activity rows say "for the Client" where the name would do.

