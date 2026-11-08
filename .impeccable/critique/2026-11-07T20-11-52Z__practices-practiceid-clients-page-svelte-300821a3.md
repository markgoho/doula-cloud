---
target: Staff Clients list
total_score: 23
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:f987772f73da1e58a98c0720573a37035c8173e30f3b70eca1521611cf56091f"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-07T20-11-52Z
slug: practices-practiceid-clients-page-svelte-300821a3
closed: true
---
Method: dual-agent (A: design review · B: detector/overlay)

## Design Health Score: 23/40 (Acceptable)

| # | Heuristic | Score | Key issue |
|---|---|---|---|
| 1 | System status | 3 | Count line honest, skeleton loading; no total |
| 2 | Real world match | 3 | "Has work / No work yet" is system-speak |
| 3 | Control/freedom | 3 | Filters in URL |
| 4 | Consistency | 3 | Overdue and Owes share amber pill; two look-alike links per row |
| 5 | Error prevention | 3 | Double link misclick risk |
| 6 | Recognition | 2 | Pills decoded by color+icon+text; no money column; order unexplained |
| 7 | Efficiency | 1 | No sort, no state filter, ~60 tab stops before Load more |
| 8 | Minimalist | 1 | Routine facts as loud as exceptions |
| 9 | Error recovery | 2 | Load failure has no retry |
| 10 | Help | 2 | Pill vocabulary unexplained |

## Design Specificity Verdict
Generic admin CRUD. Detector: 0 findings in source (page + DataTable, Badge, Link, FilterBar, ListPage) and in the authenticated overlay (30 rows, 40-94px). Busyness is structural: ~11 marks per row; color inverted against attention (Contract signed/Paid saturated green, No Contract yet dimmest grey with minus icon).

## Priority Issues
1. [P1] Routine and exceptional signals carry equal weight. Fix: routine states as plain muted text; pills only for exceptions (Overdue stronger than Owes, No Contract yet, No Doula yet, Request pending). distill / quieter.
2. [P1] No sort by attention, no state filters, no money column (server-side sort is BFF work). shape + layout.
3. [P2] Two identical links per row (name -> hub, kind -> Engagement); hub already links Engagements. distill.
4. [P2] ~470px chrome above row 1; Overdue pill wraps under 65ch cell cap. layout.
5. [P2] Blank Status/Doula cells in the "with work" view; Work column redundant. clarify.

## Persona Red Flags
- Renata: must read every cell to find who owes; Overdue == Owes visually.
- Priya: no due date / on-call on the list.
- Sam: exception announced last; ~2 tab stops per row.
- Alex: no sort, filter, in-list find.
- Casey: 3-4 cards per phone screen; two adjacent small links.

## Minor Observations
- "so far. More remain." double hedge; mixed-polarity toggles under one legend; minus-circle icon on Active reads as removed; Pilot pill adds to pill fatigue; no retry on load error.

## Questions to Consider
- Would anyone miss "Contract signed"/"Paid" on a calm row?
- Is this two screens: an owner's work queue and a doula's roster?
- Should "No Contract yet" on 20 of 30 rows be a count at the top instead?
