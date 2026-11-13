---
target: Staff Schedule (rubric round 2; craft 19/20)
total_score: 38
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2747-schedule-hill-climb/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte"
target_fingerprint: "sha256:0ea5bfdec1c2a0047c116d640dfc5cdffb18ccba05b767b3be10d3e087b1ac9f"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2747-schedule-hill-climb/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte
timestamp: 2026-11-13T18-36-11Z
slug: practiceid-schedule-schedulescreen-svelte-65635fb0
---
Method: round 2 at e97faadc, after three more builds (empty states with a next step, one column of records, the date fields on one baseline, the comma hidden in the tracked When cell, "When" on the Engagement; the seed's Visits back with each Engagement's own Doula; the empty state's opening days first and "Showing Priya Raman's 3 Visits"). Usability (`schedule.md`) + craft, three blind graders, median per item; T4 walked blind as Priya (375×740) and T2, T3 as Dee (1280×800). Seed: 42 Visits in the default 30 days, no Engagement with more than two Doulas.

## Usability 38/40 at the median (graders 38, 38, 40) · Craft 19/20 (19, 19, 19)

Medians: H1 4, H2 3, H3 4, H4 3, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T8 pass (3/3).

## Findings (two or three graders)

- H2, H4: the table's When cell draws no comma and splits the day and the clock ("Fri Nov 13      2:00pm"), where ADR-0022 writes "Thu Sep 4, 2:00pm" and the Engagement and the cards show "Fri Nov 13, 2:00pm" (2/3). The comma was hidden after round 1's unscored note that it hung before the aligned clock; the fix for a note cost two checks.
- C4: the table sizes to its content (753px on the whole list, about 470px narrowed to one Doula), so When and Doula move about 5px between narrowings (3/3); the first column's trailing gutter is 17.6px against the Clients list's 11.7px (1/3); a range error wraps to three lines in the To field and lifts "To" about 90px above "From" (1/3); the table stops at 753px where the siblings' fill about 1205px (1/3).

## Notes (one grader, or scored by none)

- T3: nothing narrows to the off-Engagement Visits; Dee read all 42 rows, "tedious with more" (3/3 as friction, no check failed).
- T2: the Engagement page has no link back to the Schedule; browser Back restores it.
- T6: the day and time are heard twice per row, in the link name and in the When cell.
- T8: a refused Load more says "Try again" and the control still reads "Load more" (the Clients and Invoices lists too).
- H1: the skeleton's 300ms was judged from source and specs; no grader got a live reading.
- "Next 7 days" alone shows its tag and no "Clear filters" (the siblings' rule: beside one tag its × is the clear).
