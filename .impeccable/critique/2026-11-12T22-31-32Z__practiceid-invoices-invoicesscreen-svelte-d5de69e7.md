---
target: Staff Invoices list (rubric round 1; craft 17/20)
total_score: 36
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 4
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte"
target_fingerprint: "sha256:8c76d0cb05cb82eda13d1077548de38644cb7da27b4d5a6e65f99bdff6891a20"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte
timestamp: 2026-11-12T22-31-32Z
slug: practiceid-invoices-invoicesscreen-svelte-d5de69e7
---
Method: rubric round 1 at 5334e5ec, after builds 1 to 4. Usability (`invoices-list.md`, H8 as amended 2026-11-12) + craft, three blind graders, median per item.

## Usability 36/40 at the median (graders 34, 36, 38) · Craft 17/20 (17, 17, 17)

Medians: H1 4, H2 3, H3 4, H4 3, H5 4, H6 3, H7 4, H8 3, H9 4, H10 4. Craft: C1 3, C2 4, C3 4, C4 2, C5 4.

Tasks: T1 to T8 pass with all three graders. T4 takes 3 actions at 375px, where the search folds behind "More filters" (2/3).

## Findings (two or three graders)

- H2: "Of that, returned" is not an owner's word; she says refunded, and one reads it as bounced checks (3/3). "Raised" is not an American office manager's word (2/3).
- H4, C4, H8: the applied-tag row is reserved while no filter is applied, so the result line sits 65px under Apply (Clients: 10 to 24px), further than the sections are from each other (3/3).
- C4: one to two whole rows on the first screen at 1280×800; row 1 bottom at 732px (3/3). One grader adds the 21ch state track in Money, sized for "payment on its way".
- H6: "Reference or Client" folds behind "More filters" at 375px and 640px (2/3).
- H8: a paid row says "paid" beside the amount and "Paid Nov 10, 2026" in the date cell (2/3).
- C1: with the name removed the screen reads as any service business's invoice book (3/3); the care kind beside the Client is the suggested fix.

## Notes (one grader)

- H4: paid, written off and payment on its way are plain words here and badges on the Engagement's Invoices section; the surface brief still promises a blue and a green badge. H4: the Invoices table fills its frame and the Clients table does not; column x moves between views.
- H8: a screen reader hears the reference in the link name and again in the Reference cell. H2: the header "Money".
- The focused "Skip to results" link covers the last total. A card's date breaks mid-date at 375px.
- Seed: no Stripe-billed row and no payment on its way; the written-off and void rows are behind Load more.
