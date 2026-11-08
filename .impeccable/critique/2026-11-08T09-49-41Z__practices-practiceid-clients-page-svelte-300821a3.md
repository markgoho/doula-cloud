---
target: Staff Clients list (rubric round 9; craft 17/20)
total_score: 40
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 0
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:f58f7623ccd95c1947b06c547b06f4fd02414f1b5ff88e0fdba890b22a5f7956"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-08T09-49-41Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
Method: rubric round 9 at a1e3ff2b, usability + craft, three blind graders, median per item.

## Usability 40/40 at the median (graders 39, 39, 38) · Craft 17/20 (18, 18, 17)

Usability medians: all ten heuristics 4. Each grader dropped one different point: G1 H3 (no single-filter clear), G2 H10 ("care or a request" needs decoding), G3 H2 ("care or a request") and H6 ("then most recent" names no date).
Craft medians: C1 4, C2 3, C3 4, C4 2, C5 4.

All tasks T1–T8 pass for all three.

## Findings (two or more graders)
1. Money header right edge (x≈1263) not over the amounts (≈1171) (all three).
2. Bar-to-table gap (~5–6px) tighter than the gaps inside the bar; at 768 the count line squeezes into a 4-line side column (G1, G3).
3. "care or a request" wording (G2, G3) — awaiting Mark.

## Notes (one grader)
G2: at 1280–1366 the date's nowrap grid overflows its 154px cell and runs into the Doula name on most rows (regression from dcc5a8d2) — a real defect regardless of count. G3: "then most recent" names no date; count line ~89ch. G2: columns shift between filter states (Status starts at 227/255/272). G1: Status column too wide at ≥1440.
