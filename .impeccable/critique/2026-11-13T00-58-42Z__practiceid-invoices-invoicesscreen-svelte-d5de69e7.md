---
target: Staff Invoices list (rubric round 2; craft 18/20)
total_score: 37
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte"
target_fingerprint: "sha256:ac177032af37b213c02a309f5d8c3edc1b517fdb8c1f62f587c3cd207804a234"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2746-invoices-hill-climb/app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte
timestamp: 2026-11-13T00-58-42Z
slug: practiceid-invoices-invoicesscreen-svelte-d5de69e7
---
Method: rubric round 2 at 875a9aa8, after builds 5 and 6. Usability (`invoices-list.md`) + craft, three blind graders, median per item.

## Usability 37/40 at the median (graders 35, 37, 38) · Craft 18/20 (18, 18, 18)

Medians: H1 4, H2 3, H3 4, H4 2, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 4, C3 3, C4 3, C5 4.

Tasks: T1, T2, T4, T5, T6, T8 pass 3/3. T3 pass 2/3. T7 pass 2/3.

## Findings (two or three graders)

- H2, H4: "Of that, refunded $300.00" matches no row; the refunded Invoice reads only "Paid", and the BFF sends no per-row refund (3/3 name it; 2/3 score it).
- H4, C3: "Written off" is a red badge on the Engagement's Invoices section and a plain word on the list, where red means late (2/3). DESIGN.md's Signal Red entry counts an uncollectible Invoice as failed, so the list is the one out of step.

## Notes (one grader)

- T3, H3, H7: Back after Load more gave 30 rows at the top with focus lost; the Engagement's history entry carried the list's URL and state. A second grader saw the same once and traced it to the iframe method adding history entries; clean runs restored 35 rows and the position. To be pinned by a spec either way.
- T7: with a view applied, row 1 is the 7th stop (Skip, radios, search, Apply, Clear filters, the tag, row 1); the budget is 6.
- H2: the header "Due or paid" is a bare "Due", the Clients list's word for the pregnancy's due date (ADR-0015). H2: "Sent Nov 12 · Paid Nov 11" on a seed row paid before it was sent.
- H4: the Clients filter bar shows a visible "Show only" and a second row; the Invoices legend is hidden. The Clients header band bleeds past the text and its table hugs its content; the Invoices table fills its frame.
- C4: about five rows at 1280×800 and the phone's first card ends at 820px (1/3); 55px between the result line and the first card at 375px (1/3); a "·" hangs at a line end on the card (1/3). C2: totals at 19.18px beside 18.99px.
