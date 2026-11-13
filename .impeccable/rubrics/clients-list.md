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
- **T7 Keyboard:** every filter and every row is reachable, with a visible focus ring. Getting past the filter bar to row 1 takes ≤ 6 Tab stops.
- **T8 Failure:** with the BFF refusing the list, the screen says what happened in Voice-rule words, nothing is lost, and "Try again" recovers.

## Heuristic checks

1. **Visibility of system status:** loading shows a skeleton within 300ms. Every filter change visibly changes the list or says that nothing matched. The count line states what is shown, including the filter and the order. Load more states progress.
2. **Match with the real world:** every label is a word a doula or owner uses (no "has work"). Due dates or postpartum windows are shown as the person says them. Money reads as owed/paid/overdue with amounts.
3. **User control and freedom:** every filter except the Name is in the address (back, reload and share keep it); the Name is kept by back and forward, never in a URL (#1829). Any filter can be cleared in one action. Leaving for a Client and coming back restores the filter and position.
4. **Consistency and standards:** title, action, filter bar, table and count use the same templates and placement as every other Staff list (measured: same title x/size, same action placement, same filter bar). The primary action looks like the primary action elsewhere. Badge colors mean one thing across the app. A convention the category has taught this screen's readers (the surface brief's Category baseline) is on the screen, or the brief records its decline with a reason that holds for a reader those products have trained; a convention a grader names that the baseline omits fails until the baseline records it.
5. **Error prevention:** filter combinations that can never match are not offered. A money filter is never offered to a reader who may not see money (BFF refuses it too).
6. **Recognition rather than recall:** the order is stated and useful (T1/T4 do not depend on knowing creation dates). Filters are visible, not hidden in menus. Each badge's meaning is clear from its words alone.
7. **Flexibility and efficiency:** T1, T2, T3, T4 and T5 each pass within their action budgets. The page size covers a 14-doula Practice's live roster, or a name filter makes paging unnecessary.
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
