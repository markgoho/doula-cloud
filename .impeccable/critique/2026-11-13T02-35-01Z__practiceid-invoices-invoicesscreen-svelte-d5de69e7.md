---
target: Staff Invoices list (rubric round 3; craft 18/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte"
target_fingerprint: "sha256:42e3f07fbb62e59517b2445aecc9f61f6aebc3455db7c09d6d72d572cfd5df59"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte
timestamp: 2026-11-13T02-35-01Z
slug: practiceid-invoices-invoicesscreen-svelte-d5de69e7
---
Method: rubric round 3 at the commit after build 9. Usability (`invoices-list.md`) + craft, three blind graders, median per item.

## Usability 39/40 at the median (graders 38, 38, 38) · Craft 18/20 (16, 18, 19)

Medians: H1 4, H2 4, H3 4, H4 3, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 3, C3 4, C4 3, C5 4.

Tasks: T1 to T5, T7, T8 pass 3/3. T6 fails 3/3 on one part.

## Findings (two or three graders)

- T6, H4/H7 (WCAG 1.3.1): in the table the cell under "Reference" hides the reference (`aria-hidden`), so a screen reader hears "Postpartum, billed By hand" under that header; the reference is spoken only in the row link's name (3/3). Introduced by build 5's fix for "heard twice".
- H4: the filter bar is arranged unlike the Clients list's: Clients shows a visible "Show only" label on the choices' own row, then fields and Apply; Invoices hides its legend and puts the radios inline with the search (3/3).
- C4: the Money column's x moves between views, because its state track is sized from the rows shown (2/3).
- C2: the totals' notes break mid-phrase at 375px and 320px ("$15,465.00 on 17 / Invoices") (2/3).

## Notes (one grader)

- H10: in the Unpaid or Overdue view an empty search offers only "Show every Invoice", which drops the typed text.
- C4: result line to table is 21px here and 44px on Clients; the Invoices table fills its frame, so on a very wide window the name sits far from the amount. C4: at phone width "Overdue" shares a line with the search field.
- C3: the search field's native clear control is the browser's. C1: only the data is the domain's.
- Observed: the nav hides Invoices from an employed Doula though the BFF serves her and the screen has copy for her.
