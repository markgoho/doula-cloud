# Clients list: what a 4 means

A heuristic scores 4 only when **every** check under it passes, each with evidence a grader can point to: a screenshot, a measured value, the accessible text, or a persona walker's act log. A check with no evidence fails. 3 = one check fails, 2 = two, 1 = three or more, 0 = the heuristic's core job is absent.

Graders are three independent agents per round. None sees an earlier score, this branch's commits, or another grader's output. The score per heuristic is the median of the three. A task check ("T…") is walked blind by `persona-walker` against the running stack, and the grader reads its act log.

Seed: the demo Practice under `dev:full` (Rooted Birth Collective, 14 doulas, 30+ Clients). Widths: 1440px, 768px, 375px; 200% zoom at 1280px.

## The jobs (from docs/personas)

- **T1 Renata, money:** from arriving on Clients, see every Client who owes money, overdue first or marked, without paging. ≤ 2 actions, and the count is stated.
- **T2 Renata, paperwork:** see every Client whose care has begun with no Contract. ≤ 2 actions.
- **T3 Renata, coverage:** see which Doula carries each live Engagement, and narrow to one Doula's Clients. ≤ 2 actions.
- **T4 Priya, phone (375px):** see only her own Clients, the soonest due first, with her first Client on the first screen.
- **T5 Anyone:** find a named Client (e.g. "Camille") from the list. ≤ 2 actions, no lost filter state.
- **T6 Screen reader:** a row reads as name, then each line's kind, calls and facts with audible separators. The table's purpose and count are announced.
- **T7 Keyboard:** every filter and row is reachable in drawn order with a visible focus ring; "Skip to results" reaches the result line in ≤ 2 stops and row 1 in one more; tabbing the bar stops once per control (a radio group is one stop), on nothing that is not a control and on no control this reader is not offered.
- **T9 Dee, room in a month:** see which Clients are due in March, and only Priya's, by the due date and not by scanning the Date column. ≤ 2 actions at 1280px (the Doula select, then the Due select), with the month in the address, in a tag and in the result line.
- **T10 Dee, a spreadsheet of what the list shows:** one press on "Download CSV", which an Owner or Admin sees, takes every Client the applied narrowing, name and order match, not the 30 loaded, in plain-word columns with no email, phone, address or clinical field; the file keeps the server's name and the page says "Downloaded clients-2026-11-14.csv, 16 Clients." in place; a refusal or no connection is said in place in Voice-rule words with the button still there; each download is one Activity entry. (`personas/non-doula-admin.md`, #2891)
- **T8 Failure:** with the BFF refusing the list, the screen says what happened in Voice-rule words, nothing is lost, and "Try again" recovers.

## Heuristic checks

1. **Visibility of system status:** loading shows a skeleton within 300ms. Every filter change visibly changes the list or says that nothing matched. The count line states what is shown, including the filter and the order, and, while more rows exist, how many Clients there are in all. Load more states progress.
2. **Match with the real world:** every label is a word a doula or owner uses (no "has work"). Due dates or postpartum windows are shown as the person says them. Money reads as owed/paid/overdue with amounts.
3. **User control and freedom:** every filter except the Name is in the address (back, reload and share keep it); the Name is kept by back and forward, never in a URL (#1829). Any filter can be cleared in one action. Leaving for a Client and coming back restores the filter and position.
4. **Consistency and standards:** title, action, filter bar, table and count use the same templates and placement as every other Staff list (measured: same title x/size, same action placement, same filter bar). The primary action looks like the primary action elsewhere. Badge colors mean one thing across the app. A convention the category has taught this screen's readers (the surface brief's Category baseline) is on the screen, or the brief records its decline with a reason that holds for a reader those products have trained; a convention a grader names that the baseline omits fails until the baseline records it.
5. **Error prevention:** filter combinations that can never match are not offered. A money filter is never offered to a reader who may not see money (BFF refuses it too).
6. **Recognition rather than recall:** the order is stated and useful (T1/T4 do not depend on knowing creation dates). Filters are visible, not hidden in menus. Each badge's meaning is clear from its words alone.
7. **Flexibility and efficiency:** T1, T2, T3, T4, T5, T9 and T10 each pass within their action budgets. The page size covers a 14-doula Practice's live roster, or a name filter makes paging unnecessary.
8. **Aesthetic and minimalist design:** no warning color on more than a third of the rows in the seed. Table rows have equal rhythm (row heights within 4px of each other); record cards keep one field order. No element on the page carries no information. Nothing at 375px pushes row 1 below the first screen except the title, the action and the filters.
9. **Error recovery:** T8 passes. A filter with no results says so and offers the way back to the unfiltered list.
10. **Help and documentation:** every filter's effect is clear from its label. The empty state for each filter says what was looked for and what to do. No term needs a glossary.

## Cross-cutting (each failure costs a point from the heuristic named)

- 320px and 200% zoom: no horizontal page scroll, every control usable (4, 7).
- WCAG 2.2 AA: contrast, focus not obscured, target size 24px (4, 7).
- The constraints hold: ADR-0021 GOV.UK patterns, ADR-0024 intrinsic layout (no viewport queries), the Voice rules (4, 2).

## Changes to this rubric

- 2026-11-08, after round 5: H3 no longer asks that the Name filter live in the address. #1829 forbids a Client's name in any URL, so every grader either failed a screen for obeying a recorded decision or passed it by setting the check aside. The check was wrong about the job, not hard to pass.
- 2026-11-08, after round 13: H8's equal-rhythm check now applies to table rows; record cards are held to one field order instead. Applied to cards, it contradicted the check beside it: cards with different content can be equal height only by drawing empty lines, which 'no element carries no information' fails. Round 12 failed the screen for uneven cards, round 13 for the empty Money line that evened them. Equal rhythm serves scanning across rows of a table; a stack of cards is read one at a time.
- 2026-11-13, after the Schedule's climb closed: H4 gained the category baseline check. The jobs came from the persona records and no check asked what a screen of this kind has taught its readers to expect, so three rounds scored the Schedule 40/40 without anyone asking whether its table should sort (#2866). The check was missing a part of the reader's job, which is the ground for changing a rubric. The same day the check was sharpened: a recorded decline passes only when its reason holds for a reader the category has trained, since the first baseline's declines of a sort were recorded and wrong (#2869).
- 2026-11-13, #2867: H1 asks for the total. The category baseline (#2866) found that the Invoices list says "the first 30 of 35" and this list said no total; the endpoint lacking the figure was a reason to add it, not to leave it out.
- 2026-11-13, #2869/#2863: T7 no longer counts stops. The keyboard reader's job is to skip the bar, which "Skip to results" does, and to operate every control; six was the bar's size when the rubric was written, not a job, and a bar that holds what the category baseline asks for cannot keep it.
- 2026-11-14, #2871: T9 added, "room in a month". The category baseline (#2866) found that every list a roster is kept in can be narrowed by a date, and this one could only be read down its Date column; the journey that asks it ("Priya has room in March", `journeys/non-doula-admin.md:89`) had no job here. The jobs came from the persona records and none was about when, which is a part of the reader's job the rubric was missing, and the ground for adding one. H7 counts it with the other budgets.
- 2026-11-14, #2891: H8 counts a red or amber badge as a warning color; an Invoice owed and not yet due is no longer one (DESIGN.md, "Money is said once"), so the plain word "owed" does not count and a red "N days overdue" does. The check is unchanged; what the app draws as a warning changed, and a grader who counted every "owed" as amber would now over-count. T1 gained a way to meet its "overdue first" without paging: the "Most overdue first" order.
- 2026-11-14, #2891: T10 added, from the persona records and the surface brief's export row, which turned from "Later" to "Have it". Two of three graders in the #2891 round found the Clients export "Later" while the Invoices list had one, and the rubric's jobs ended at reading the list. The same job is on the Invoices rubric (#2871); a bookkeeper's and a tax year's questions leave the product the same way from either list. H5 gains nothing new: the export posts the applied narrowing, never an unapplied draft.
