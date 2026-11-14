---
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/schedule-round-6/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:b3b704b27b28067d927ae5da969c8aee24dea4bc8072de942d8d8f72f1cca89f"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/schedule-round-6/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-14T08-31-43Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
---
target: Staff Clients list (rubric round after the Order, bar, due-month and Money-total builds; craft 20/20)
total_score: 38
max_score: 40
p0_count: 0
p1_count: 3
---
Method: a round at 80819c8a (#2891), the first since the Clients climb's last round, after #2869 (a reader-chosen order), #2901 (boxes, radios and selects apply when changed), #2899 (held tracks, the Doula fold), #2905 (the whole-row target), #2908 (the Doula select held from first paint, an empty result's own next step, the Client's total owed), #2913 (the due month). Usability (`clients-list.md`, with T9 new) and craft, three blind Opus graders from the template, each writing "Expected and missing" before reading the surface brief. Walked blind first, by Sonnet walkers: Dee (1280×800: who owes and who is furthest behind, Priya's room in March, active with no agreement, find Camille) and Priya (375×740: her Clients and who is due soonest, her room in March). Priya's walker set the Due select with chrome-devtools `fill`, which steps it like a keyboard and is held by design until Enter (#2901); a pointer pick on the same stack applied at once, and the graders were told so.

## Usability 38/40 at the median (graders 38, 38, 38) · Craft 20/20 (20, 19, 20)

Medians: H1 4, H2 4, H3 4, H4 3, H5 4, H6 4, H7 4, H8 3, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 4, C5 4.

Tasks: T1 to T9 pass (3/3). T1 1 action (16 Clients who owe, all on one page, overdue marked). T2 1 action. T3 1 action. T4 from Priya's walk: she arrives on `?mine=true`, Camille Boyd's card whole on the first 375×740 screen. T5 2 actions, the name in no URL, Back keeps it. T9 2 actions at 1280, `?doula=…&due=2027-03`, the month in the tag and the line.

## Findings (two or three graders)

- H4 (2/3, A and B): the Clients list's export is "Later" while the Invoices list one tab away has "Download CSV"; a reader the product itself has trained expects it.
- H4 (2/3, B and C, and Dee's walk): no order by how overdue, and the Money cell says "overdue" with no day count, so "who is furthest behind" sent Dee to the Invoices list (Stripe's invoice lists teach lateness).
- H8 (2/3, B and C): warning color on 13 of the first 30 rows (43%), against "no warning color on more than a third of the rows": an Invoice owed and not yet due carries the amber badge with ⚠. Grader A read the check as the amber variant alone (27%) and passed it.

## Notes (one grader)

- A: keyboard shortcuts are "Later" for an app-wide reason, and contact details on the row are not in the baseline; at 640px (200% zoom at 1280) the Doula and Due selects fold, so T9 is 3 actions there (H6, A alone).
- B: the title sits 3px lower on the Invoices list than here at 1280 (the header action's height centers it), C4.
- C: on Priya's Mine list every card repeats her own name as the Doula.
- Both walks and A: with nothing matched, the visually hidden live result line and the empty state say the same sentence, so a screen reader reading down hears it twice.

## Expected and missing

Named by two or three: an export (A, B); an order or day count by lateness (B, C); search as you type (B, C, both accepted Enter given the POST-body search). Named by one: a status filter (B), a total owed for the "owe money" set (B), click-to-sort headers in addition to the Order select (A), contact details (A), `/` to focus the search (A, B as a deferral).
