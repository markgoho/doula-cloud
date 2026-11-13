---
target: Staff Schedule (rubric baseline round 0; craft 17/20)
total_score: 25
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 10
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2747-schedule-hill-climb/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte"
target_fingerprint: "sha256:88bf39e46ed9b7be3cb9b76aed65268402c5aa23dc6e3481a2346870f39079ca"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2747-schedule-hill-climb/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte
timestamp: 2026-11-13T16-07-32Z
slug: practiceid-schedule-schedulescreen-svelte-65635fb0
---
Method: baseline round at fbff9c84, before any change to the screen. Usability (`schedule.md`) + craft, three blind graders, median per item; T4 walked blind as Priya (375×740) and T2, T3 as Dee (1280×800) by `persona-walker`. Seed: 43 scheduled Visits in the default 30 days across 11 Doulas, 5 in the last week, two off-Engagement (one behind Load more), page size 30.

## Usability 25/40 at the median (graders 24, 22, 25) · Craft 17/20 (16, 17, 17)

Medians: H1 3, H2 2, H3 2, H4 2, H5 3, H6 2, H7 3, H8 4, H9 2, H10 2. Craft: C1 3, C2 3, C3 4, C4 3, C5 4.

Tasks: T1 pass (3/3, 2 actions), T2 fail (3/3), T3 fail (3/3), T4 fail (3/3), T5 fail (3/3), T6 fail (3/3), T7 fail (3/3), T8 fail (3/3).

## Findings (two or three graders)

- H2, H6, T2, T5: no weekday on any day and this year's year on every one ("Nov 13, 2026, 12:00pm"), so "her Thursday" is counted by hand (3/3).
- T4: a Doula with no Owner or Admin role has no "Mine" and no Doula control; Priya's next Visit is the 7th card and one card fits 375×740 (3/3).
- H6, T3: an off-Engagement row is words only, "(No longer on this work)", with no warning Badge and no narrowing, so every row's Doula is read, after Load more (3/3).
- T2: a row opens its Engagement at the top; Reassign is about 2478px down, a third action ("Jump to", "Visits") reaches it and the way back is two Backs (3/3).
- H3: no filter is cleared in one action and nothing returns to the opening days ("Clear filters" is the inert reserve) (3/3); Back loses the loaded pages and the position (1153 to 81; 1518 to 0; 1574 to 54) (3/3).
- H4: no applied tags (3/3); no caption and a result line unlike the siblings' (2/3).
- H5, H2, H4: To before From is sent and answered by the full "Sorry, there is a problem" page, which blames "our end" (3/3).
- H1: the result line states no days, no Doula and no order (3/3).
- H9, T8: a refused read is the full error page with no "Try again" and the narrowing gone; a refused Load more is not in Voice-rule words (3/3). The empty state offers no way back to the opening days (2/3).
- H10: nothing says the To day is included (3/3); the empty state names neither the days nor the Doula (3/3).
- T6: the table has no caption; two Visits of one Client have the same link name (3/3).
- T7: 10 to 12 Tab stops from the title to row 1, three per date field (3/3).
- C1: no element a generic list would lack (3/3). C2: the clock does not line up in When (3/3). C4: one 240px card per 375×740 screen and 3 to 4 rows at 1280×800 (3/3).

## Notes (one grader)

- H8, C4: the empty clear reserve leaves about 70px between the fields and the result line. H6: the folded "Filters" at 375px does not say what is applied. T7: after Apply, focus drops to body. T8: `bothRead` fails the schedule when only the roster read fails. H10: the default days are hidden in the folded bar at 375px.
- Seed: Priya has no Thursday Visit in the window.
