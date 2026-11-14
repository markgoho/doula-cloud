---
target: Staff Schedule (rubric round 5, after the loose-end builds; craft 19/20)
total_score: 37
max_score: 40
p0_count: 0
p1_count: 4
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/schedule-round-5/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte"
target_fingerprint: "sha256:4daa8912b9702805dfc9a7fcf66bc4b5a8b399b47e1029da16815e0f809adc30"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/schedule-round-5/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte
timestamp: 2026-11-14T04-16-39Z
slug: practiceid-schedule-schedulescreen-svelte-65635fb0
---
Method: round 5 at 15427d25, after the climb's loose-end builds: the Visit's type on the row and a Doula arriving on Mine (#2870), a reader-chosen order (#2869), the Client search (#2870), the narrowing to Visits that need a new Doula with its count (#2863), Earlier and Later with the phone fold (#2870), times in the Practice's zone (#2872). Usability (`schedule.md`, with T9, T10 and the rewritten T3 and T7) + craft, three blind graders from the template, each writing "Expected and missing" before reading the surface brief. Walked blind first: Priya (375×740: her next Visit, a Client by name, last week, latest first) and Dee (1280×800: T2, T3, T9, T10, the order). The graders were stopped at about 35 minutes and told to report from the evidence in hand; checks not reached are marked unverified in their reports and are not counted as fails (forced colors, light-theme contrast and the measured sameness with the sibling lists for one grader; a full Tab sweep for another).

## Usability 37/40 at the median (graders 37, 38, 38) · Craft 19/20 (19, 19, 20)

Medians: H1 3, H2 4, H3 4, H4 3, H5 4, H6 4, H7 4, H8 3, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T10 pass (3/3). T1 2 actions, 17 Visits on one page. T2 2 actions to narrow, 1 to the Engagement with Reassign in view, Back restores the filter and focus. T3 2 actions, both Visits, whole without "Load more", counted on the line beforehand. T4 from Priya's walk: she arrives on `?mine=true`, first card's bottom 661.8 of 740. T9 2 actions, the name in no URL (a POST), Back keeps the search. T10 1 action each way, one action back.

Round 4 scored 39 with one point held by three unbuilt conventions. Those are built and every job passes. The 37 is three new single-check failures the new controls brought, none of them a job.

## Findings (two or three graders)

- H4, omitted from the baseline (3/3): a filter by the Visit's type (Acuity's appointment type, Calendly's event type); (2/3): a whole-row click target (Stripe). (2/3): the month calendar is "Later (#330)" with a deferral, not a reason.
- H1 (2/3): when one Visit shows, the result line drops the order: `Showing 1 Visit for "Camille" from Fri Nov 13 to Sat Dec 12.` (`visitSchedule.ts`, `hasMore || count > 1`).
- H8 (2/3): with "Latest first" chosen the order is said three times: the result line, the "Order:" tag and the select.
- C4 (2/3): in the table the comma after the day sits about 20px from the time ("Tue Nov 17,    11:00am"), so the When cell reads as two cells.
- Outside any check (3/3): the bar's select, boxes, radios and order do nothing until Apply; Dee waited for the list to change four times and Priya twice. The baseline's decline of instant filters holds for typed fields, not for these. (3/3): the Engagement's back link reads "Clients" when the reader came from the Schedule (#2864).

## Expected and missing, before the brief was read

Named by all three: a week or month calendar; a filter by Visit type; an export, print or calendar feed. By two: a whole-row click target; a Visit's length and place; booking a Visit from the list; a Doula picker that takes several; saved views. By one: a "Today" jump (built in effect: the Days tag's × and the "Today" heading); a mini-month date picker (built through "Choose days"); showing cancelled Visits.

Declines the graders judged: ordering by Doula or by Client holds (3/3). A Visit's length, place and status ("the model lacks them") hold (3/3). Saved views ("the address is the view") holds for two, weak for one. The week grid's deferral holds; the month view's does not (3/3). Export and print deferred to #2871: a deferral, not a reason (3/3). Instant filters: the decline holds for typed fields only (2/3).

## Notes (one grader)

- "Needing a new Doula" (the box) and "No longer on this work" (the badge) are two names for one condition; Dee read them as the same and the counts agreed.
- The empty search's longer look goes forward only; a Visit last month is missed.
- An empty search leaves about 50px of blank room under its tag.
- The baseline's row "Mine on already for a Doula" still reads "Build (#2870)"; it is built.
- Priya: "Earlier" from the opening days steps 30 days, so "last week" meant reading October; tapping "Next 30 days" in a stepped window lights the radio and leaves the list until Apply; the search carried into Earlier.
- Walk brief error (the orchestrator's): Dee was asked for Priya's "Friday" Visit; on this seed her Visits fall on Mon, Tue and Sat. The graders walked T2 for her Saturday.
