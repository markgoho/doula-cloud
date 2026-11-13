---
target: Staff Schedule (rubric round 3; craft 19/20)
total_score: 40
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 0
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2747-schedule-hill-climb/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte"
target_fingerprint: "sha256:a88fdb8dc2e69a8901ec0275c4f98f63e2cf6cbb336f01546f360c2b26151937"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2747-schedule-hill-climb/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte
timestamp: 2026-11-13T19-23-23Z
slug: practiceid-schedule-schedulescreen-svelte-65635fb0
---
Method: round 3 at 26ecd813, after one more build (one `VisitWhen` for the Schedule and the Engagement with its comma drawn, held Client, When and Doula tracks, a range error drawn once over the date pair) and the records (DESIGN.md, ADR-0018, govuk-alignment). Usability (`schedule.md`) + craft, three blind graders, median per item; T4 walked blind as Priya (375×740) and T2, T3 as Dee (1280×800). Seed as round 2's.

## Usability 40/40 at the median (graders 40, 39, 40) · Craft 19/20 (19, 19, 19)

Medians: H1 4, H2 4, H3 4, H4 4, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T8 pass (3/3). T1 2 actions; T2 2 to narrow, 1 to open with Reassign in view, 1 Back to the list as left; T3 1 or 2 actions, both rows found by the badge; T4 Mine and Apply, her next Visit whole on the first 375×740 screen; T5 0; T7 6 stops, 2 by "Skip to results".

## Findings (two or three graders)

- C4, density (2/3): at 375×740 an Owner's title and bar fill about 475 of 740px, leaving one whole Visit; at 640 and 768px the list is 126px cards, because the table's floor is a 55rem frame and the held 42ch Doula track makes the table about 822px.
- C4, the When cell (1/3 as a failed check, 3/3 as a note): the comma is drawn and a gap of 3 to 5ch follows it before the right-aligned clock, so one phrase reads as two columns. The gap is the price of weekdays at one x and clocks on one edge in constant tracks (DESIGN.md, "A Visit's When").

## Notes (one grader, or scored by none)

- H1 (one grader; a defect, fixed after the round): a cold load of `?doula=` draws the tag "Doula: that Doula" and the select "Any Doula" until the roster lands, then both redraw.
- T3: nothing narrows to the off-Engagement Visits (3/3 as friction; every round). "Mine" and a different Doula picked is a dead end for an Owner who is a Doula (1/3).
- T6: the day and time are heard twice per row (3/3).
- H2: "No longer on this work" does not say what work when read alone on a list row (1/3; the Engagement page's words, #2648).
- H1: the skeleton appears at exactly the 300ms limit (1/3).
