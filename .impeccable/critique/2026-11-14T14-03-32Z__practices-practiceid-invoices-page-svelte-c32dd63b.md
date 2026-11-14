---
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/schedule-round-6/app/src/routes/practices/[practiceId]/invoices/+page.svelte"
target_fingerprint: "sha256:8cf63f6ce8a449dc45ed3fa06edd631d6c97175c718bc62c941d7df403afc33e"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/schedule-round-6/app/src/routes/practices/[practiceId]/invoices/+page.svelte
timestamp: 2026-11-14T14-03-32Z
slug: practices-practiceid-invoices-page-svelte-c32dd63b
---
---
target: Staff Invoices list (rubric round after the Order, bar, period, export and billed builds; craft 19/20)
total_score: 36
max_score: 40
p0_count: 0
p1_count: 5
---
Method: a round at 4f33dd12 (#2891, #2871), the first since the Invoices climb's last round. It follows #2869 (a reader-chosen order), #2901 (boxes, radios and selects apply when changed), #2905 (the whole-row target), #2909 (the Sent period and Download CSV), #2914 and #2915 (Billed with a period, a download that says it happened, the phone's first card), #2919 (an emptied search sent as drawn), #2920 (owed no longer a warning) and #2922 (every cell heard). Usability (`invoices-list.md`, with T9 and T10) and craft were scored by three blind Opus graders from the template, each writing "Expected and missing" before reading the surface brief. A Sonnet walker walked first, blind, as Dee (1280×800): who to chase, a check for INV-0012, what was billed in October, and a spreadsheet of unpaid Invoices this year.

## Usability 36/40 at the median (graders 38, 36, 37) · Craft 19/20 (19, 19, 19)

Medians: H1 4, H2 3, H3 4, H4 3, H5 4, H6 3, H7 4, H8 4, H9 4, H10 3. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Tasks: T1 to T10 pass (3/3; T7 unverified by one grader). T2 1 action, six rows longest first. T3 1 action to the row with Record payment; Back restores the filter, scroll and focus. T4 2 actions, no name in the URL. T5 the first card on the first 375×740 screen. T9 1 to 2 actions, "Billed $12,790.00 on 14 Invoices" for October. T10 one press, "Downloaded invoices-2026-11-14.csv, 14 Invoices.", 12 plain columns, one Activity entry.

## Findings (two or three graders), each filed with acceptance criteria

- H2 (2/3, A and B): "Billed" means two things. It is the period total's amount, and also a method: each row says ", billed By hand" to a screen reader, and the CSV has a "Billed" column of By hand or Stripe.
- H10 (2/3, B and C): the totals say whose they are only with a period. Under a view or a search nothing says they are the whole book's. Dee could not tell whether "Clients have paid" is before or after refunds.
- H6 (2/3, B and C): at 640px (200% zoom at 1280) and at 375px, Sent and Order sit behind "More filters".
- H4 (3/3, different items): conventions neither on the screen nor in the baseline. Named by two or more: a Paid (and void or written-off) state filter, a filter by Doula, New Invoice from the list, a reminder from the row, A/R aging buckets, and the Client's contact details from an overdue row.
- C4 (3/3): in the record cards the "Sent" and "Payment due" lines start about 4px right of the card's edge, pushed by a leading "·" separator. At 640px a line can end on a dangling "·".

## Notes (one grader, or unscored)

- The Order select reads "Payment due, earliest first" while the result line says "longest overdue first" (A, B).
- Download CSV stays enabled with 0 rows (B). The download notice pushes the bar down when it appears (C). The CSV's file name does not carry the period (C).
- Stale docs: the surface brief and one DESIGN.md line still say "amber owed" (B).
- An Enter pressed inside a radio's 250ms settle may drop a typed search (A, low confidence).
