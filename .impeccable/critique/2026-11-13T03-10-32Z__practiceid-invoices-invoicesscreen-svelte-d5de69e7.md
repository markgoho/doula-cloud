---
target: Staff Invoices list (rubric round 4; craft 19/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 1
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte"
target_fingerprint: "sha256:929b7daad26d8d5beacea378a74105ee7e4f21a3c1ef3a2acc675df233e7744c"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte
timestamp: 2026-11-13T03-10-32Z
slug: practiceid-invoices-invoicesscreen-svelte-d5de69e7
---
Method: rubric round 4 at the commit after build 10. Usability (`invoices-list.md`) + craft, three blind graders, median per item.

## Usability 39/40 at the median (graders 38, 39, 39) · Craft 19/20 (19, 19, 19)

Medians: H1 4, H2 4, H3 4, H4 3, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T8 pass with all three graders.

## Finding (three graders)

- H4, C4: the gap from the result line to the list is 18 to 20px here and 40 to 45px on the Clients list (1280 and 1920). The Clients bar's ruler holds a second line for its longest result wording, empty in the default view.

## Notes (one grader)

- H8: a void or written-off record card ends "Sent Nov 12, 2026 · —"; the dash says nothing.
- T5: at 375×740 the first card's money line ends at 723px, 17px from the edge.
- The "×" icon is on the red overdue badge and on the remove-filter tag, side by side when Overdue is applied.
- The headings "Money" and "Payment" both read as money. The Invoices table fills its frame on a very wide window (#2739 owns `fill`).
- Between about 1000px and 1210px the list shows record cards capped near 638px wide, from the 71rem floor.
- A search inside a narrowed view searches that view; a paid Invoice's reference takes a third press through the empty state's button.
- The nav hides Invoices from an employed Doula though the BFF serves her.
