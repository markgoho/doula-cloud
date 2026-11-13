---
target: Staff Schedule (rubric round 1; craft 18/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 3
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2747-schedule-hill-climb/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte"
target_fingerprint: "sha256:bbb7e170a4288afcc36c8552e67669e0bc38d2228311023ac8c4937663c41b2e"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2747-schedule-hill-climb/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte
timestamp: 2026-11-13T17-24-02Z
slug: practiceid-schedule-schedulescreen-svelte-65635fb0
---
Method: round 1 at 7057ebeb, after two builds (weekday-led days and the `#visit-` arrival; the siblings' filter bar with Days, Mine and a Doula, tags, the result line, the compact table, the off-Engagement Badge, failure in place and return). Usability (`schedule.md`) + craft, three blind graders, median per item; T4 walked blind as Priya (375×740) and T2, T3 as Dee (1280×800). Seed as the baseline's.

## Usability 39/40 at the median (graders 38, 38, 38) · Craft 18/20 (18, 18, 17)

Medians: H1 4, H2 4, H3 4, H4 4, H5 4, H6 4, H7 4, H8 4, H9 4, H10 3. Craft: C1 3, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T8 pass (3/3). T1 2 actions; T2 2 to narrow, 1 to open, 1 Back; T3 1 action (Load more), found by the Badge; T4 Mine and Apply, her next Visit on the first 375×740 screen; T7 6 stops.

## Findings (two or three graders)

- H10: an empty Mine or Doula result on the opening days names what was looked for and offers no next step (2/3).
- C1 (3/3), also H4 and H6 (1/3 each): between the phone and the table (768px) the records are a two-column card grid, so a list in time order reads across and then down; the Clients list draws one column there.
- C4: with "Choose days" open, To's hint puts its label about 33px above From's, the date inputs are 52px against the select's 50px, and an error widens the To column (3/3).

## Notes (one grader, or scored by none)

- H2: "Showing 1 of your Visit from…" and "1 of Priya Raman's Visit" (one grader; a defect).
- H8: narrowed to one Doula, the Doula column repeats that name on every row. C4: an Owner's bar takes about 490 of 740px at 375px.
- The comma hangs in the tracked When cell ("Fri Nov 13,      12:00pm") (3/3, unscored). A one-day range reads "from Mon Nov 16 to Mon Nov 16". At 375px the result line breaks a day across lines. The Engagement's Visits head the same column "Date".
- T3: no narrowing to the off-Engagement Visits; Dee read all 43 rows "to be sure" (3/3 as friction, no check failed).
- T6: a screen reader hears the day and time twice, in the link name and in the When cell.
- After Back and after Load more, the Engagement of the row that takes focus is fetched without a click.
