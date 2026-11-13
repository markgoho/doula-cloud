---
target: Staff Invoices list (rubric round 6; craft 19/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 1
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte"
target_fingerprint: "sha256:409bf3715344e9c9ecf03d8dbe7d1d7c9bee2f27bc191b67deb4a8306062754b"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte
timestamp: 2026-11-13T04-27-45Z
slug: practiceid-invoices-invoicesscreen-svelte-d5de69e7
---
Method: rubric round 6 at the commit after build 12. Usability (`invoices-list.md`) + craft, three blind graders, median per item.

## Usability 39/40 at the median (graders 38, 39, 39) · Craft 19/20 (19, 19, 19)

Medians: H1 4, H2 4, H3 4, H4 4, H5 4, H6 4, H7 4, H8 3, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T8 pass with all three graders. H4 is 4 with all three: the table width rule, the Money cell and the filter controls now match the Clients list.

## Finding (three graders)

- H8, C4: at a 1920px window the Client column is held at its 15ch track while the type grows, so "Morgan Calloway" wraps to two lines and its row is 58px against 40px for the rest, with 600px or more of the frame empty. Every row is 40px at 1280, 1366, 1440 and 1600. Introduced by build 12's `--data-table-grows-track: 15ch`.

## Notes (one grader)

- H2: on a phone card a paid Invoice reads "Sent Oct 12, 2026 · Nov 11, 2026"; the paid day has no visible word (the table has the "Payment" header; the card has none).
- T6: in the table the Sent day and a paid row's day carry their kind only from the column header.
- At 375px Apply sits at the far edge with about 35px between it and the field. At 768px the cards are 500px wide with a third of the frame empty. The Engagement sits at its top for about a second before scrolling to the Invoice.
