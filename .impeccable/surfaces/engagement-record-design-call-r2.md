# Engagement record (#2755): design call before round 2, 2026-11-15

Round 1 scored 32/40 (8 at 1: 8a, 8c, 8h; 3e, 6d, 9c, the Voice note, C4/C5). One Opus agent, from the three grader reports, the walk logs, the source at f416fb55 and the records. Heights are **estimates** (round-1 per-section numbers, arithmetic shown); the build reports the real ones. It amends `engagement-record-design-call.md`; nothing here reopens ADR-0018's 2026-11-07 amendment (no rail, tabs or sticky).

## D1. Details leaves the sections and sits under the facts at wide widths (8c, 8g, baseline 27)

- `RecordDetail` gains a `details` snippet (an aside region with its own `h2 "Details"`, `id="details"`). **DOM order: Jump to → facts → sections → details.** Today (`RecordDetail.svelte:736`) the facts take `grid-row: 1 / span 2` beside Jump to (row 1) and the sections (row 2), so the facts top is level with Jump to (382 = 382). From the 73rem container the grid becomes `grid-template-rows: auto auto auto 1fr`: Jump to col 1 row 1; facts col 2 `1 / span 2` (unchanged); details col 2 row 3, `align-self: start`; sections col 1 `2 / -1`. The sections span the `1fr` track, so they are sized in the flexible step and do not inflate rows 1–2; row 2 is then only what the facts need past Jump to, and Details starts under the facts. Narrower, Details stacks last, after Activity. No viewport query (ADR-0024). `EngagementScreen` sets no grid (4c). **Hold-still:** at 1280 the first h2 top and the facts top do not move from round 1 (±4px). **Fallback** if they move or the aside overlaps: Details stays a section, last before Activity, with D1's rows and links; total then ≈ 5910 (D10).
- **Focus order:** facts, the sections, then Details: the aside's lower half is read last, as the narrow layout draws it. T9 is amended (R5) to say so; a CSS `order` that reorders focus is refused.
- **Rows (no header fact repeated, 8g):** Care (Change), Phone (`tel:`), Email (`mailto:`), Portal invite (Send portal invite), Created. Under them one list: "Change the due date", "Change the care phase", "Put on or take off a Doula" (each for the reader the BFF lets act, as today's `changeable` flags), then "View Client", "Add details". Due, Care phase and Doulas are no longer rows: their values are the header's, their Changes are these three links. Where the header has no due line (Imogen, a birth with no date): "Give the due date" (today's "Not given" row's Change, #2118). With no Doula (Tamsin): "Put a Doula on". After an outcome or postpartum: no due link (as today's row rules, `EngagementScreen.svelte:1252`).
- **Forms:** each opens in place under the list in the 15–17rem column, takes focus on its h3; Cancel returns focus to its link (govuk-alignment:135). The build proves each form whole at 272px (no overflow, targets ≥ 24px); a form that does not fit is a stop-and-report, not a squeeze.
- **Anchors:** `#engagement-summary` → `#details` everywhere (`focusDoulaChange`, On call's "Put another Doula on this birth", `putDoulaOnWords`, Birth outcome's `reopenHref`), focusing the matching link above.
- "Details" stays in Jump to, last (8i: it is a heading region on the page): Jump to loops over `sections` (`RecordDetail.svelte:270`), so it appends a "Details" link when `details` is given. Effect: Phone sits ~250px under the facts at 1280, near the name (grader A's #6).
- Files: `RecordDetail.svelte` (+ its spec, style-guide record page), `EngagementScreen.svelte` (`detailsSection`, `summaryRows`), `sectionOrder.ts` (drop `'Details'`).

## D2. The header holds one move; "Add a Visit" is the Visits section's own (8h, 6b, Priya act 8)

- Header: **"Message the Client"** alone, for a reader the BFF lets send. It now opens the compose in Messages with focus in the Message field (as "Add a Visit" opened its form), so T4 is 1 action to typing, the newest Message directly above the field.
- Visits: secondary Button **"Add a Visit"** on the line after the state line (same line where both fit, a `cluster-l`); empty: "No Visits yet." + "Add a Visit" (replaces "Add the first Visit"). Priya reached for exactly this (acts-priya "Reached for" 2).
- Count at 375 (PR-G10): Message the Client, Jump to (one list), the facts (one group) = **3**. T3: Jump to → Visits (1), Add a Visit (2), form h3 focused, first field next Tab = 2 ✓. T9 stops skip → Clients → Message → On call → Birth Plan = 4 ✓.
- Rejected: plain-text facts on a phone (needs width-dependent DOM, and T8 at 375 grades their link names); an overflow menu (D9); calling two outline buttons "one list" (all three graders counted two).

## D3. On call: the Doulas' days, then the gaps; the first row on screen at 1280 (8a, C4)

- Order inside the section: state line → window line → days `SummaryList` → h3 "When a Doula cannot be reached" → gap rows → "Put another Doula on this birth" + "Record a time a Doula cannot be reached" on one `cluster-l` line. `isGapCoverFirst` is removed **on the record only**: it came from the On call list's climb (#2748/#2936, "answers tonight first"); here the state line answers tonight, and gaps-first drew the days rows under the gaps' h3 (grader C: the h3 swallowed the days it qualifies).
- State line, **ink, no icon** (D7): "Nobody on call tonight: Priya Raman cannot be reached until Mon Nov 16, 10:00am." / two gaps: "Nobody on call tonight: Priya Raman and Bethany Kroll cannot be reached." / no gap (days nobody holds): "Nobody on call tonight." / "Priya Raman is on call tonight." It wraps to 2 lines at the measure (+27px, in the estimate below).
- On call's own rubric (`on-call.md` T2) asks that the Engagement opened from a needs-cover row shows the On call heading and "the control that covers a gap **or** changes a Doula's days" at 1280×800: the days rows' Change is in view (~690), and a landing on `#on-call` puts the gap row's Change in view too. No check pins gaps-first on the record.
- Gap row at a wide container: When and the "Nobody covering" Badge on one line, reason muted under it; the 66px under the h3 becomes `--space-4`.
- Target 1280×800 (Camille): h2, state line and Priya's days row bottom **≤ 780** (est. ~715: h2 485–520, state 545–599, window 617–644, row 665–715).

## D4. Order rule a grader can apply (6d)

Replace 6d's wording with: **"A section is empty when the Engagement read says it holds nothing: no Contract, no Visit, no Message, no Invoice, no Offer, no Care Plan or Birth Plan started, Birth outcome before the due date, On call before its on-call days. No empty section stands above a non-empty one, except On call during its on-call days and Birth outcome from the due date. Activity is last on every record (brief #433: built, not what people come for). The order is settled from the Engagement read alone (`sectionOrder.ts`), so nothing moves when a later read lands (1a)."** Code: `sectionOrder.ts` `isEmpty` gains On call before its window (it is `default: false` today); Details leaves the order (D1). An empty section draws its empty line (8e) and nothing else, so what a grader sees and what the flag says agree: Invoices on a signed Contract with none raised draws "No Invoices yet." + "Create Invoice" and no state line or billing sentence. Grace's "Messages above Invoices" was 8e's bug: Invoices before a signed Contract becomes one line (D8), both are empty, and the intake table orders them.

## D5. The top bar marks the record's list (3e)

`listOrigin.svelte.ts`: `ListBack` writes its origin to module-level `$state` (`recordOrigin`); `routes/practices/+layout.svelte`'s `navItems`, for a path under `/engagements/` (the `birth-plan/` child included), marks current the item for that origin (schedule → Schedule, invoices → Invoices, on-call → On call, contracts → Contracts), **Clients by default** (cold load, Overview, Clients). Same `current` the Client record gets by prefix today. A contractor's or Practice-without-Invoices bar marks Clients when the origin item is absent. Specs: layout/StaffTopBar spec, `engagement-back-link.svelte.spec.ts`.

## D6. One back link pattern: the bare name of where it goes (3e, C4)

GOV.UK's Back link with the destination's name, the majority form already (`"Clients"`, `"Staff"`, `"Settings"`) and 3e's own quotes: "Clients", "Schedule", "Invoices", "On call", "Contracts"; the Birth Plan page "Camille Boyd" (was "Back to Camille Boyd"). `listOrigin.svelte.ts` `LABEL`, `birth-plan/+page.svelte:60`, the `ListBack.label` fallback. The two other "Back to …" labels are the portal's Invoice page (`portal/…/invoices/[invoiceId]/+page.svelte:121`, a Client screen) and the style guide's document page: out of this climb; the alignment row says the Staff rule. Amend `govuk-alignment.md:158` (it quotes "Back to the Schedule"). Grep `app/e2e` for `Back to`.

## D7. "Nobody on call tonight" said once loudly (1c, 1c', 8g, C5)

| Place | Says | Tone |
|---|---|---|
| Header status line | nothing (the Badge goes) | — |
| Facts, On call tonight | "Nobody" | amber icon + link: **the loud one** |
| On call state line | "Nobody on call tonight: Priya Raman cannot be reached until …" | ink, no icon (its own section's restatement, 6c) |
| Gap row | "Nobody covering" Badge | amber (where the act is; below the fold at 1280) |

Rubric: delete 1c'; 1c reads "with the warning icon" (the facts carry an icon, not a Badge, by the first call). 1c' existed so the phone saw it; the facts' On call row is on the 375 first screen (y ≈ 545 after the Badge line goes). `EngagementStatusLine.svelte` loses `isNobodyOnCallTonight`.

## D8. Notes settled

- **Refused "Load more"** (9c): `lib/paginatedList.svelte.ts:206,225` passes `error_.message` raw. Only the network case changes: a fetch `TypeError` → ADR-0059's sentence "We could not reach the service. Check your connection and try again."; every other error keeps today's path (other lists may rely on the passed-through message).
- **The Visits "Earlier" cap and Messages' 2** (D10) open themselves when the page lands on a hidden row (`landOnVisitId`, `EngagementScreen.svelte:818`), and for any deep link to a Message. App-wide by nature; comment on #1935, which keeps the rest.
- **Offline Engagement read**: `ErrorPage` gains kind `unreachable`: title "We could not reach the service", body "Check your connection and try again.", the same way back. The route picks it when the read never reached the service. No "You are offline" (ADR-0059 rejected it). Other routes stay #1935's.
- **"No Messages yet."**: GLOSSARY's capital on Staff screens. `MessageThread` gains `emptyMessage`; the record passes "No Messages yet."; the portal keeps its own (copy.domainNoun rules). The fact reads "No Messages yet".
- **Visits When**: one `<time>` with a normal space, `tabular-nums`; the justified split in `VisitWhen` (`isTracked`) goes. If the Schedule shares it, both change (DESIGN.md:336 "read the same").
- **h1 35.6 vs h2 28.2** (C2): the One Title Rule and the `tokens.spec.ts` ramp forbid a one-screen change. **File** a ticket against DESIGN.md's ramp; not built here.
- **Activity filters**: real links `?activity=care|money|doulas` (All: no param) `#activity`, `aria-current="page"` on the current one; onClick `goto(url, { noScroll: true, keepFocus: true })`; the page reads the kind from the URL, so Back and a shared address keep it. `+page.ts`'s `load` reads `params` only, never `url`, so the query does not re-run the Engagement read. `ListBack.go` then takes its `goto` path (covered by `engagement-back-link.svelte.spec.ts`).
- **Grace's Invoices** (8e): before a signed Contract one line: "Invoices start once Grace Whitfield signs the Contract." No state line, no billing sentence, no "No Invoices yet".

## D9. Expected-and-missing (4e): baseline rows to add to the surface brief

- **28. An overflow "Actions" menu: Declined.** The header holds one move (8h's budget of 3 on a phone); every other act sits in its section beside the fact it changes (govuk-alignment:135); GOV.UK has no overflow-menu pattern, and a menu puts each act behind a disclosure (2 actions where it is 1). Destructive acts stay last in their section, asking first (5a, 8f).
- **29. Internal Staff notes the Client never sees: Have it.** GLOSSARY.md:51 "Care Plan: Internal working notes … staff-only, never shown in the Client portal", and Visit notes (GLOSSARY.md:43). Not a founder question. To be recognized, the Care Plan's state line ends "Camille Boyd never sees the Care Plan."

## D10. Height: 3600 is not reachable; proposed amendment (8c)

Round 1: 6605 at 1440×900 = sections 5686 + ~920 (top ~470, 10 gaps × ~43, tail ~60). Estimates after D1–D8:

| Section | R1 | R2 est. | How |
|---|---|---|---|
| On call | 710 | 570 | D3: h3 gap −40, gap row one line −50, two openers one line −50 |
| Birth Plan | 675 | 630 | link and Change on one line; answers uncapped at 1440 (T1 needs them) |
| Visits | 759 | 600 | Earlier shows the newest 1, "Show earlier Visits" in place (−170 +50); Add a Visit beside the state line |
| Messages | 600 | 450 | newest 2 shown; "Latest: …" line dropped (the newest bubble is under it) |
| Care Plan | 427 | 340 | cap 15rem for the Care Plan (Birth Plan keeps 32rem) |
| Contract | 562 | 520 | PDF and "Read the Contract text" on one line; Void subsection kept (8f) |
| Invoices | 469 | 460 | — |
| Offers / Birth outcome | 149 / 151 | 149 / 151 | already 8e; their line and button are the shared `EmptyState` (button on the next line); changing it for every consumer is not this climb's |
| Activity | 554 | 430 | first page 5 rows (route constant; Overview keeps 8) |
| Details | 630 | 0 | D1 (aside beside the sections, ~560, ends above Contract) |
| **Total** | **6605** | **≈ 5240** | 4300 + top 470 + 9 gaps 387 + tail 80; error band ±250 (estimates from round-1 heights, not measured); with D1's fallback ≈ 5910 |

3600 leaves ~2650 for ten sections (265 each); On call, Birth Plan and Visits alone need ~1800 to show their state line and the first rows their jobs read (the gap, T1's answer, T3's last notes). Reaching it means hiding those behind disclosures the jobs open every time, which 8c's own second clause fails. **Proposed 8c:** "Camille's page at 1440×900 is at most 5400px (6 screens) with every section on it, each section within its ceiling in the r2 design call's table plus 10%, and no section hidden behind a disclosure its job opens every time." Reason: the cap measures the arrangement's waste, not the content's floor; 5400 keeps 3d's Back to top in force. 5400 is only ~3% over the estimate: the orchestrator fixes the number **after** the build reports the measured total (the per-section ceilings carry the check either way); with the fallback it would be 6000.

## Rubric amendments (orchestrator writes them, dated)

R1 8c as D10. R2 delete 1c'; 1c "warning icon" (D7); 8g: drop the 1c' collision. R3 6b: "The header holds 'Message the Client', opening the compose; 'Add a Visit' is the Visits section's first control"; T3: "reach Add a Visit's first field in ≤ 2 (Jump to → Visits, Add a Visit)". R4 6d as D4. R5 T9: "at wide widths the order is Jump to, facts, sections, Details (the aside's lower half last)". R6 3e/C4: back links are bare names (D6).

## Measurements the build reports

| Measure | R1 | Target |
|---|---|---|
| 1440×900 page height, Camille; and each section | 6605 | ≤ 5400; each ≤ D10 + 10% |
| 1280×800: On call h2 / state line / first days row bottom | 872 (gap row) | ≤ 780 |
| 1280 and 1440: facts top = Jump to top; first h2 top; Details aside bottom < page bottom | 382 = 382; h2 ~485 | equal; h2 ±4px of round 1; aside never lengthens the page |
| 272px aside: each Details form, overflow and targets | — | 0 overflow, ≥ 24px |
| 375×740: decisions on first screen; On call tonight fact bottom | 4; 598 | 3; ≤ 740 |
| 375 after Jump to → Birth Plan: pain-relief answer bottom | 673 | ≤ 740 |
| Amber marks on the 1280 first screen (Camille) | 3 | 1 |
| Top bar `aria-current` from Clients / Schedule / Contracts / cold load | none | Clients / Schedule / Contracts / Clients |
| Refused Load more (offline) text | "Failed to fetch" | ADR-0059 sentence |
| Hold-still: 320 scroll 0; console errors 0; T1–T10 budgets; 2a find "engagement" 0 | — | unchanged |

## Specs and e2e that change

`sectionOrder.spec.ts` (no Details; Activity last), `on-call-section.svelte.spec.ts` (days before gaps; state line words; no gaps-first), `engagement-detail.svelte.spec.ts` (header move, Visits' Add a Visit, Details aside rows and three Change links, Messages 2 shown and no Latest line, Activity 5 rows and `?activity=` links, Visits Earlier cap, no header Badge, Grace's one Invoices line, "No Messages yet."), `engagement-back-link.svelte.spec.ts` (labels, `goto` path from a filter), `RecordDetail` spec (`details` snippet, grid rows), `routes/practices` layout / `StaffTopBar` spec (current on a record), `paginatedList` spec, `MessageThread` spec (`emptyMessage`), `ErrorPage` spec (`unreachable`), `VisitWhen` spec. `app/e2e`: grep "Back to", "Add the first Visit", "Nobody on call tonight", "Latest:", `#engagement-summary`. Docs: ADR-0018 dated amendment (`details` region; consumers the Engagement record, #2754 named second; the 2026-11-15 amendment's `statusLine` "and who is on call" and "Message the Client" and "Add a Visit" as the `outline` Link's consumers are corrected), DESIGN.md (the same two lines), govuk-alignment rows 135 and 158, the surface brief: baseline row 2 ("Message the Client" in the header; "Add a Visit" in Visits, with D2's reason), the differs bullet "The header's two moves", FIRST VIEWPORT, and new rows 28–29.

## Wireframes

```
1440×900                                                   facts column 17rem
← Clients
Camille Boyd   [Message the Client]
Care phase Active   Due Dec 5, 2026, 37 weeks   Doulas Priya Raman, Bethany Kroll
Jump to On call  Birth Plan  Visits  Messages  Care Plan        | On call tonight
        Contract  Invoices  Offers  Birth outcome  Details  Act.| ⚠ Nobody
On call                                                         | Next Visit
Nobody on call tonight: Priya Raman cannot be reached until     | Wed Nov 18, 11:00am
  Mon Nov 16, 10:00am.                                          | Priya Raman
On call from Sat Nov 14 to Sat Dec 19, from 37 weeks.           | Contract  Signed
Priya Raman     On call Nov 14 to Dec 5            Change       | Owed $750.00, due Nov 29
Bethany Kroll   On call Dec 6 to Dec 19            Change       | Messages ⚠ Unanswered …
When a Doula cannot be reached                                  |
Priya Raman  Sun Nov 15, 6:00pm to Mon 10:00am [Nobody covering]| Details
             At a wedding four hours away      Change  Clear    | Care      Birth  Change
[Put another Doula on this birth] [Record a time …]             | Phone     (585) …
Birth Plan  4 of 6 answered · last changed … · not read yet     | Email     camille@…
 …answers…                                                      | Portal invite Accepted
Read or print the whole Birth Plan   [Change the Birth Plan]    | Created   Sep 6, 2026
Visits  Next: Wed Nov 18, 11:00am, Priya Raman.  [Add a Visit]  | Change the due date
 Coming up / Not scheduled yet / Earlier (newest 1)             | Change the care phase
 [Show earlier Visits]                                          | Put on or take off a Doula
Messages … Contract … Invoices … Offers … Birth outcome … Activity (5) … Back to top
```

```
375×740
← Clients
Camille Boyd
[Message the Client]
Care phase Active
Due Dec 5, 2026, 37 weeks
Doulas Priya Raman, Bethany Kroll
Jump to On call  Birth Plan  Visits
        Messages  Care Plan  Contract
        Invoices  Offers  Birth outcome
        Details  Activity
On call tonight  ⚠ Nobody          ← y ≈ 545
Next Visit       Wed Nov 18, 11:00am
                 Priya Raman
Contract         Signed
Owed             $750.00, due Nov 29     ← fold 740
Messages         ⚠ Unanswered since …
On call … (sections) … Activity … Details (last)
```

## Orchestrator corrections (2026-11-15, after the advisor review)

- **D7 is reversed in part.** The header keeps the warning Badge "Nobody on call tonight" (rubric 1c', the answer to right-rail blindness). The facts row says "Nobody" in plain words with no icon; the On call state line is plain text naming the gap. 8g exempts that one alert.
- **D1 keeps drawn order.** Details must not sit after the sections in the DOM while drawn beside the facts: Tab at 1440 would run every section and jump back up (T9 "drawn order"). Draw Details twice, as ADR-0018's contents rail and strip once were and as DataTable's two trees are: an aside right after the facts in the DOM, shown where the record-body is two columns, and the last section, shown where it is one; exactly one is displayed (display:none takes the other out of the accessibility tree), ids scoped by which copy, and a Details form rendered only into the shown copy (the `rowDetail` / `shownView` pattern). Jump to links "Details" to whichever copy is shown (one id on each copy is not possible; use the section id on the narrow copy and point Jump to at it only when narrow, or give the link the shown copy's id). If this cannot be built cleanly, Details stays a section (D1's fallback) and the build reports why.
- **8c is 5400px, fixed in the rubric before the build.** Report the measured total; do not loosen anything to meet it.
