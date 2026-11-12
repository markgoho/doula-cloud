---
target: Staff Invoices list (rubric baseline round 0; craft 16/20)
total_score: 25
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 8
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte"
target_fingerprint: "sha256:72676988ec8bcc998a96a5e5c619ec3137502ba020210704bffa2e57b144ee1d"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte
timestamp: 2026-11-12T04-26-15Z
slug: practiceid-invoices-invoicesscreen-svelte-d5de69e7
---
Method: baseline round at 2fcfd641, before any change to the screen. Usability (`invoices-list.md`) + craft, three blind graders, median per item. Seed: 35 Invoices (16 paid, 10 owed, 7 overdue 3 to 62 days, 1 void, 1 uncollectible, 1 refunded), page size 30.

## Usability 25/40 at the median (graders 21, 26, 26) · Craft 16/20 (16, 16, 16)

Medians: H1 2, H2 2, H3 4, H4 2, H5 4, H6 3, H7 2, H8 1, H9 3, H10 2. Craft: C1 4, C2 3, C3 4, C4 2, C5 3.

Tasks: T1 fail (3/3), T2 fail (3/3), T3 pass (2/3; one grader measured position lost on back, 900 to 471), T4 fail (3/3), T5 fail (3/3), T6 fail (3/3), T7 pass (3/3), T8 fail (3/3).

## Findings (two or three graders)

- H1: the result line "Showing every Invoice." states no count and no order and is false at 30 of 35; Load more states no progress (3/3).
- H2: "Billed" beside "Billed via"; "Paid" the total and "Paid" the day (3/3).
- H4: the result line is unlike the Clients list's (3/3); no unpaid narrowing, so "Clients owe / 17" cannot be shown as rows (3/3).
- H6, T2: the overdue list is ordered newest raised first, so 3 days late leads and 61 days late is last (3/3).
- H7, T1: no unpaid view; the 17th unpaid Invoice is on page 2 (3/3). T4: no reference or Client search (3/3).
- H8: amber and red on 16 to 18 of the rows (3/3); the Overdue badge beside "N days overdue", and "—" in Paid on every unpaid row (3/3); row 1 at 1017px at 1280×800 and 991 to 1008px at 375×740, each record card about 640px (3/3).
- H9, T8: a refused read offers no "Try again" (3/3); the load-more failure is not in Voice-rule words (3/3).
- H10: nothing says Overdue is part of Clients owe (3/3).
- T6: the table has no caption; two Invoices of one Client have the same link name (3/3).
- C4: title to intro gap larger than the section gaps (2/3); the job does not fill the first screen (3/3). C5: the totals block or the badge field competes with the list (2/3). C2: totals' amounts and the card's Amount do not share an edge (1/3, note).

## Notes (one grader)

- H3: position and loaded pages lost on back. H2, H10: "Uncollectible" beside the Engagement's "Write off". H4: "Owed"/"Overdue" here, "owed"/"61 days overdue" on Clients; filter stacked above Apply. H10: the overdue empty state names no next step. C1: no element a generic invoice list would lack.
- Seed: Tessa's INV-0002 reads paid the day before it was billed; every row is By hand.
