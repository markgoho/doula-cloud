---
target: Staff Schedule (rubric round 4, with the category baseline; craft 19/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2867-schedule-day-headings/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte"
target_fingerprint: "sha256:43ad04be260e3a59f5bc65fd2c7d2dba5e352b52cb66ff3bdcbeda4074dd4f9b"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2867-schedule-day-headings/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte
timestamp: 2026-11-13T22-46-29Z
slug: practiceid-schedule-schedulescreen-svelte-65635fb0
---
Method: round 4 at 18f58d9e, after #2867's three builds (day headings with Today and Tomorrow; totals; the Practice's time zone) and the category baseline (#2866, #2869). Usability (`schedule.md`, with H4's baseline check) + craft, three blind graders briefed from the new template: each wrote "Expected and missing" before reading the surface brief. T4 walked blind as Priya (375×740) and T2, T3 as Dee (1280×800); each walker was asked what the persona reached for and did not find. Seed as round 3's.

## Usability 39/40 at the median (graders 39, 37, 38) · Craft 19/20 (19, 19, 19)

Medians: H1 4, H2 4, H3 4, H4 3, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T8 pass (3/3).

Round 3 scored 40 on a rubric that did not ask what a screen of this kind has taught its readers to expect. This round's 39 is the same screen, better, held to that question: H4 stays at 3 until the baseline's builds are on the screen.

## Findings (two or three graders)

- H4, the category baseline (3/3): three conventions the baseline marks "Build" are not on the screen: a reader-chosen order (#2869), a search by Client (#2870), and stepping earlier and later (#2870).
- H4, omitted from the baseline (3/3): the Visit's type on the row (the Engagement shows it; Priya: "is it a prenatal or the birth?"); a narrowing to the Visits that need a new Doula, and their count (Dee, a fourth walk). (2/3): booking a Visit from the Schedule; "Mine" on already for a Doula.
- C4, density (3/3): at 375×740 the bar takes about 490px and one Visit is whole on the first screen.

## Expected and missing, before the brief was read

Named by all three: sortable columns (Linear, Stripe Dashboard, MOJ); a week or month calendar; previous, next and a Today jump; a search by Client; the Visit's type, length and place; a needs-attention narrowing; an export, print or calendar feed; booking a Visit from the Schedule. By two: a way to past Visits; a Doula picker that takes several. By one: "Mine" by default for a Doula; filters that apply at once; reassigning from the row; saved views; keyboard shortcuts.

Declines the graders judged: ordering by Doula or by Client ("a schedule is read in time") holds (3/3). A week grid waiting on a Visit's length holds; the month view's deferral is weak (3/3). No export or print: holds for one grader, weak for two.

## Notes (one grader)

- H4: a refused date range has no GOV.UK error summary, and no departure is recorded. H8: narrowed to one Doula, the Doula column repeats that name; the empty state's icon. H10: "No longer on this work" names no record Priya knows.
- "Clear filters" shows with no tag when the address holds a refused range. "Mine" with another Doula picked is always empty for an Owner who is a Doula. A screen reader hears the day twice per row. "Nothing you saved is lost" on a screen that saves nothing.
- Two graders read the day headings as sharing one `tbody`; the table has one `tbody` per day (11 on page one, measured), each opening with a `th scope="rowgroup"`.
