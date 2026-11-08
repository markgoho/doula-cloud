---
target: Staff Clients list (rubric round 8; craft 18/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 0
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:5bc06758b170d8357ee96d9a0ba88f0d30bb5b01c6e017c24e368bd0f0f1de50"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-08T09-17-55Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
Method: rubric round 8 at 00d5e8c0, usability + craft (both rubrics as amended after rounds 5 and 7), three blind graders, median per item.

## Usability 39/40 (graders 39, 37, 39) · Craft 18/20 (16, 17, 17)

Usability medians: H1 4, H2 4, H3 4, H4 3, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4.
Craft medians: C1 4, C2 3, C3 4, C4 2, C5 4.

All tasks T1–T8 pass for all three.

## Findings (two or more graders)
1. "overdue" overflows its 5.5ch money-state track and runs into the Date cell: "$950.00 overdueBorn Oct 18" at 1280–1920 (all three).
2. The filter bar reflows between filter states at one width (Apply jumps from the right of the switch row to a second row under a Doula filter; the count line moves between inline and its own row), and the Clients bar's arrangement differs from Invoices' (all three).
3. Dates start at different x by lead word: Due / Born / Ended (all three).
4. Amber and red each read as two meanings app-wide (owes vs other warnings; overdue vs error) (two).

## Notes (one grader)
PRODUCT.md positioning 4 already names the activity ledger as the product's signature component, contradicting DESIGN.md's "no signature" (G1). Single-filter removal takes two actions (G2). Record cards uneven in height at 768/640 (G2). Overdue not sorted first under the owes filter (G2).
