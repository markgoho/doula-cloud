---
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/schedule-round-6/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte"
target_fingerprint: "sha256:8ead2fedd52bff2de459caa30a3926a195a2215f7cc733fca4970ff9d8ee6338"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/schedule-round-6/app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte
timestamp: 2026-11-14T13-10-12Z
slug: practiceid-schedule-schedulescreen-svelte-65635fb0
---
---
target: Staff Schedule (rubric round 6, after #2888's builds; craft 19/20)
total_score: 38
max_score: 40
p0_count: 0
p1_count: 2
---
Method: round 6 at 48ddbeb3 (#2888), after the boxes, radios and selects that apply when changed (#2901), the Visit type filter (#2902, #2907), the whole-row target and the heard-once row names (#2905), the back link from an Engagement (#2906), the badge's words on the box and a year each way for an empty search (#2907), "This week" in Days with Earlier and Later stepping a week (#2916), and an emptied search sent as drawn (#2919). Usability (`schedule.md`, T3's budget no longer naming Apply) and craft, three blind Opus graders from the template, each writing "Expected and missing" before reading the surface brief. Walked blind first, by Sonnet walkers: Dee (1280×800, two passes: Priya's Tuesday Visit, every Visit without its Doula, Camille's Visits, last week and the week after next; the first pass's search defect was fixed in #2919 before the second) and Priya (375×740: her next Visit, her week, last week). The round was paused by a usage limit partway through; the graders resumed on the same stack and signed-in pages, and none marked a check unverified for it.

## Usability 38/40 at the median (graders 37, 37, 40) · Craft 19/20 (19, 19, 19)

Medians: H1 4, H2 4, H3 4, H4 2, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 3, C3 4, C4 4, C5 4.

Tasks: T1 to T5 and T7 to T10 pass (3/3); T6 fails (2/3). T1 1 action, 18 Visits on one page. T2 1 to narrow and 1 to the Engagement with Reassign in view; Back restores the filter and focus. T3 1 action, both Visits, the one past page 1 included, counted on the line beforehand. T4 from Priya's walk: she arrives on `?mine=true`, Imani Brooks's card whole on the first screen. T5 the first card ends at 732 of 740. T10 1 action each way; last week is "This week" then "Earlier", 2 actions (Priya), or Choose days (Dee).

Round 5 scored 37. The lost points: H4's two failed checks, and C2.

## Findings (two or three graders)

- T6 and H4, WCAG 1.3.1 (2/3, A and B): the When cell is `aria-hidden` (`namedByRowLink`, from #2905's fix for #2864 item 2), so a screen reader moving by cell hears a blank under "When", the sorted column, and the day and time only inside the Client's link. Decided: every visible cell is heard, and a row's link adds the day and time only when another loaded row has the same Client.
- C2 (2/3, B and C): the result line is one line of about 92ch at 1280, past the 65–75ch prose measure. Declined, with the reason recorded in DESIGN.md: it is a one-sentence status line, not prose, and two lines at 1280 would cost the Clients list its seventh row.

## Notes (one grader each, or unscored)

- H4 (A): color per Doula (Acuity, Jane App) is not in the baseline. (B): a Visit's end time or length, a past Visit's outcome (Dee reached for it), its location, and adding a Visit from the Schedule are not in the baseline. Each is being added with a verdict.
- H10 (B): "This week" includes days already gone, which surprised Priya on a Saturday.
- H8 (A): the empty state's icon carries no information.
- C4 (A): at 1568px an applied tag joins the bar's row and the Client field shrinks; at 1280 the bar holds still.
- Unscored (A, B): "Nothing you saved is lost" on a read-only list's load failure answers a worry nobody has there; being dropped. (B): the card reads "Doula: · Aditi Sundaram" to a screen reader; the separator is being hidden.
- Unscored (C): a filter tag looks like the Apply button where it sits beside it.

## Expected and missing

Named by two or three: a week or month grid (all three; the month decline held for all, the week deferral held); a Visit's length, location or status (A, B, C; deferred to #330, now recorded). Named by one: color per Doula (A), the Client's phone on the row (B), bulk Reassign (B, C), a print sheet (A, C, as recorded).
