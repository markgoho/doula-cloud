---
target: Staff Clients list (rubric round 1)
total_score: 31
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 3
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:38713c3644de1931f399c8b72040e1abd05a87d61f6e26d3184197b71614410a"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-08T02-51-21Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
Method: rubric round 1 (.impeccable/rubrics/clients-list.md), three blind graders, median per heuristic, at ee8cb550. Walks before it (on earlier SHAs, fixed since): Renata desktop, Priya 375px iframe, keyboard + accessibility tree. persona-walker unavailable (chrome-devtools MCP not connected), so walkers were general agents on Claude in Chrome, blind by instruction.

## Score: 31/40 (graders 32, 30, 30)

| # | Heuristic | G1 | G2 | G3 | Median |
|---|---|---|---|---|---|
| 1 | Visibility | 4 | 4 | 4 | 4 |
| 2 | Real world | 2 | 2 | 2 | 2 |
| 3 | Control | 4 | 4 | 3 | 4 |
| 4 | Consistency | 3 | 2 | 2 | 2 |
| 5 | Error prevention | 3 | 4 | 4 | 4 |
| 6 | Recognition | 4 | 3 | 3 | 3 |
| 7 | Efficiency | 4 | 3 | 3 | 3 |
| 8 | Minimalist | 3 | 2 | 3 | 3 |
| 9 | Error recovery | 4 | 4 | 4 | 4 |
| 10 | Help | 1 | 2 | 2 | 2 |

Tasks: T1, T2, T3, T5, T8 pass (2 actions each). T4 fails (2 of 3): an active filter opens the narrow disclosure and row 1 lands at y≈903 of 740. T6 pass (2 of 3). T7 fails (3 of 3): 7–8 tab stops across the bar, budget 6.

## Findings (two or more graders)
1. "work" vocabulary: "Clients with work", "Also show Clients with no work yet", "Nothing open" (H2, H10).
2. Postpartum lines show "—" under Due; no birth date or window (H2).
3. Warning color on 19 of 30 rows (17 "No Contract yet") (H8).
4. Primary action is a text link; Staff's is a filled button (H4).
5. Narrow: the disclosure opens with a filter on, pushing row 1 off the first screen (T4).
6. 7–8 tab stops across the filter bar (T7).
7. Empty states for owes / no Contract / mine give no next step (H10).
8. Clients filter bar differs from Invoices' chip links (H4).

## Notes (one grader)
- A tick made right after navigation was lost before hydration (also seen on Priya's walk).
- `?q=` puts a Client's name in the address: history, Referer, logs.
- "Also show…" beside a narrowing is a no-op (H5).
- No one-action removal of a single filter.
- Count not tied to the table (aria-describedby).
- Narrow record cards 164–395px tall.
