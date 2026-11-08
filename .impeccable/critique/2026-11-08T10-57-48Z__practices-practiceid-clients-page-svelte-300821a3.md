---
target: Staff Clients list (rubric round 11; craft 19/20)
total_score: 40
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 0
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:8b621b54a4a4b6ba8bcab324989cff2be8fd5dd304563eaae07c2963b376a93e"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-08T10-57-48Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
Method: rubric round 11 at 5d60ef85, usability + craft, three blind graders, median per item. Widths now include 1024, 1100, 1180.

## Usability 40/40 at the median (graders 40, 39, 38) · Craft 19/20 (18, 18, 19)

Usability: every heuristic median 4. Dropped points: G2 H2 ("care or a request"), G3 H3 (single-filter clear takes two actions) and H10 ("care or a request").
Craft medians: C1 4, C2 4, C3 4, C4 3, C5 4.

All tasks T1–T8 pass for all three.

## Findings (two or more graders)
1. 1024–1199px gets single-column phone cards ~640px wide in a ~1000px container; 1–3 Clients per screen (all three).
2. Cell gutter ~9px (inset 0.75×space-1): the longest name nearly touches Status (G2, G3).
3. Columns shift between filter states because Name sizes to the page's longest name (all three).
4. "care or a request" wording (G2, G3) — awaiting Mark.

## One grader
Table stretches to 1814px at 1920 with ~600px dead Status space before Money (G1). Count line off the left edge (G1). DESIGN.md says records below 55rem; code says 75rem (G1). Single-filter clear takes two actions (G3). Overdue not sorted first under owes (G3). 320 default view: row 1 starts at 681 (G3).
