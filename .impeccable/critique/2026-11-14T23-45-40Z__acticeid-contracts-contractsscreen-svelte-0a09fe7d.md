---
target: Staff Contracts list (rubric baseline round 0; craft 15/20)
total_score: 15
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 14
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2749-contracts-hill-climb/app/src/routes/practices/[practiceId]/contracts/ContractsScreen.svelte"
target_fingerprint: "sha256:73ad2826534b81d42dd74d8436cdb60b12abf1e26179c6b33a334524aa1ba9dc"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2749-contracts-hill-climb/app/src/routes/practices/[practiceId]/contracts/ContractsScreen.svelte
timestamp: 2026-11-14T23-45-40Z
slug: acticeid-contracts-contractsscreen-svelte-0a09fe7d
---
Method: baseline round at 0a23e0bf, before any change to the screen. One route, the rubric `contracts-list.md` plus craft; three blind graders, median per item, all in chrome-devtools as Renata at 1280×800, 640px, 375px, forced colors and both themes. No `persona-walker` walk: the screen has no control to walk, and T6 and T7 were observed by the orchestrator signed in as Priya (no Contracts link; the typed address answers "You cannot view this"). Seed: nine Contracts not signed (six sent, from today to 41 days; three not sent), three void requests nobody has decided, one declined, two that ended in a void.

## Usability 15/40 at the median (graders 14, 16, 15) · Craft 15/20 (17, 14, 15)

Medians: H1 1, H2 2, H3 1, H4 1, H5 2, H6 1, H7 0, H8 3, H9 2, H10 2. Craft: C1 2, C2 3, C3 4, C4 2, C5 4.

Jobs: T1 to T10 fail (3/3 each).

## Findings (two or three graders)

- T1, H2e, H6a, H6c: "Waiting since" reads `created_at`, the day the Draft was made, as an instant ("Oct 4, 2026, 7:30pm", "2 minutes ago"); no day sent, no days waited, no sender; Drafts stand among the sent rows in no stated order (3/3).
- T1, T2, T4, H3a to H3c, H4a, H4c, H6b: no filter bar, views, search, tags or "Clear filters"; signed and voided Contracts are on no list (3/3).
- H1c, H1d, H1f, H4a: no result line and no count, of the Contracts or of the queue (3/3).
- T2, T3: a row opens the Engagement at its top; the Contract heading is at y=1455 at 1280×800, a request at y=2134, "Decline" at 2162 and "Void this Contract" at 2333 (3/3).
- T3: the queue is under the Contracts, at y=821, below the first screen (3/3).
- T5, H6d: no Doula, due date or amount on a row; no Doula filter (3/3).
- T6, T7, H5d, H5e, H8g: Priya and Lena have no Contracts link and meet the refusal page (3/3).
- T8, T9, H4e, H4f: neither table has a caption or a name; no "Skip to results" (3/3).
- T10, H9a: `bothRead` fails the whole page when the void requests alone are refused; the failure page offers only "Go to your Practice overview"; a refused "Load more" says "Failed to load more contracts" under the table (3/3).
- H2d: this year's year and a clock time on every day (3/3).
- H4b: no badge; Draft and Sent are plain text (3/3).
- H9b, H10d: the empty state names nothing looked for and offers no next step (3/3).
- H10b: the intro offers "send, void or invoice the Contract there", which is true of no row (2/3).
- C1: with the name removed, a generic three-column admin list (3/3). C4: the Contracts table is 458px wide and the queue 1150px, so they share no right edge; a card at 375px spends about 250px on two facts (3/3).

## Notes (one grader)

- H1a: the loading frame has no intro, so the Contracts move down when it lands (A).
- H2c: a Draft under "Waiting since" and "still waiting on" reads as waiting on a Client (A).
- C4: the "Void requests" heading has 36px above and below, with no section step (A).

## Expected and missing, before the brief

Named by all three: views by status, search by Client, a count, the days waited, who sent it, a Doula filter, an order control, the signed Contracts with their day and PDF, the amount, a reminder from the row, a CSV export, a count on the queue. Named by two: sort by a column heading (Later, #2955), status pills, saved views (not in the baseline; the address is the saved view), bulk selection. Named by one: whether the Client opened it (Later, #2952), a row menu (Declined).

Declines and deferrals judged: #2951 (reminder) holds as sequencing 3/3, each adding that a reader trained by DocuSign will look for it; #2954 (export) holds 1/3, weak 1/3 against the Invoices list's "Download CSV", and one grader did not find the row; #2955 (heading sort) holds only once the Order select exists (3/3); #2952 holds (1/1). Saved views is added to the baseline as Declined before round 1.
