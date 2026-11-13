---
target: Staff Invoices list (rubric round 5; craft 19/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte"
target_fingerprint: "sha256:3d89c753181a3b6c7f22f2f969ebff290d604cf0991b72dfeb4b2727f940101e"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte
timestamp: 2026-11-13T03-41-04Z
slug: practiceid-invoices-invoicesscreen-svelte-d5de69e7
---
Method: rubric round 5 at the commit after build 11. Usability (`invoices-list.md`) + craft, three blind graders, median per item.

## Usability 39/40 at the median (graders 38, 39, 39) · Craft 19/20 (18, 19, 19)

Medians: H1 4, H2 4, H3 4, H4 3, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T8 pass with all three graders. Round 4's finding (result line to list gap) is closed: measured 19px on both lists.

## Findings (two or three graders)

- H4, C4: the table is built unlike the Clients list's. Invoices fills its frame (`columnWidths="fill"`), leaving 200px or more between a Client's name and the amount on a wide screen, where the Clients table hugs its content (3/3, under H4 or C4). The Money cell differs too: Clients says "$1,500.00 paid" beside the amount; Invoices shows a bare amount on a paid row with "Paid" in the Payment cell, and aligns amounts to a fixed state track (2/3).
- C4: at 375×740 the first card is cut after its amount line; "Overdue" wraps alone onto a second radio line and Apply takes its own line (2/3).

## Notes (one grader)

- H2: the headings "Money" and "Payment" both read as money.
- H8: on an empty search a screen reader hears "No Invoice matches …" twice, from the status line and the empty state.
- C3: "written off" is a red badge while the surface brief's OWN-WORLD line still calls it a plain word; DESIGN.md's Signal Red entry is the rule and the brief is stale.
- T7: with a view and a search applied, row 1 is the 8th stop.
- The "×" icon is on the overdue badge and the remove-filter tag. The seed has no Stripe-billed row and no payment on its way.
