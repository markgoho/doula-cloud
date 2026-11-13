---
target: Staff Invoices list (rubric round 7; craft 19/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 1
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte"
target_fingerprint: "sha256:b6fbfd8c12153f536a4a9ce773a58fddb1b2c61b2e7aac32ff0a6e6f2fc0e7f8"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte
timestamp: 2026-11-13T04-53-58Z
slug: practiceid-invoices-invoicesscreen-svelte-d5de69e7
---
Method: rubric round 7 at the commit after build 13. Usability (`invoices-list.md`) + craft, three blind graders, median per item.

## Usability 39/40 at the median (graders 39, 39, 40) · Craft 19/20 (19, 19, 19)

Medians: H1 4, H2 3, H3 4, H4 4, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T8 pass with all three graders. Round 6's finding (a wrapped Client name at 1920px) is closed: every row is 40px.

## Finding (two graders)

- C4: columns move between views. The Money state track follows the widest state in view, so the amounts' edge and the Reference, Sent and Payment columns shift 12 to 16px between All, Overdue and a searched view; the route's own comment says they hold.

## Notes (one grader each; H2's median is 3 on two different checks)

- H2: a refunded row reads "$1,500.00 $300.00 refunded" with a bare day under Payment, so a sighted reader never sees that it was paid.
- H2: the "Reference" column holds three facts ("INV-0031 · Postpartum · By hand"); two are not references and "By hand" has no label.
- C4: about 4.5 rows at 1280×800, with the Pilot bar and the space above the title taking the top.
- The result line's focus ring spans the bar, far past the table. A 1px vertical rule shows between "Sent" and "Payment" in the stuck heading.
- A screen reader hears the reference in the row link and again in its cell; a card is heard as "Payment: Payment due …".
- The "×" on the error badge and on the remove-filter tag (#2843).
