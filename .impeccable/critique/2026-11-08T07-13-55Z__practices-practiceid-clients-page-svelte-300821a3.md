---
target: Staff Clients list (rubric round 5; craft 17/20)
total_score: 38
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 1
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:bd5f1bd3d2b978350677778931b90ef5fa80b8b795d2194eadaaf8ce865c6c57"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-08T07-13-55Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
Method: rubric round 5 at 06b73fc0, usability + craft, three blind graders, median per item.

## Usability 38/40 (graders 37, 37, 36) · Craft 17/20 (17, 17, 17)

| # | Usability | G1 | G2 | G3 | Med |
|---|---|---|---|---|---|
| 1 | Visibility | 4 | 4 | 4 | 4 |
| 2 | Real world | 3 | 4 | 4 | 4 |
| 3 | Control | 4 | 4 | 3 | 4 |
| 4 | Consistency | 4 | 4 | 4 | 4 |
| 5 | Error prevention | 4 | 4 | 4 | 4 |
| 6 | Recognition | 3 | 3 | 3 | 3 |
| 7 | Efficiency | 4 | 4 | 4 | 4 |
| 8 | Minimalist | 3 | 3 | 2 | 3 |
| 9 | Error recovery | 4 | 4 | 4 | 4 |
| 10 | Help | 4 | 3 | 4 | 4 |

| C | Craft | G1 | G2 | G3 | Med |
|---|---|---|---|---|---|
| C1 | Specificity | 4 | 4 | 4 | 4 |
| C2 | Typography | 3 | 4 | 4 | 4 |
| C3 | Color | 4 | 3 | 3 | 3 |
| C4 | Rhythm | 3 | 3 | 3 | 3 |
| C5 | Signature | 3 | 3 | 3 | 3 |

All tasks T1–T8 pass for all three.

## Findings (two or more graders)
1. H6: the count line says "soonest due first", false for postpartum (newest birth first), care-ended and undated rows (all three).
2. H8/C4: rows wrap at 1280–1440 (two Doulas, "Ended Nov 2, 2026", a long overdue badge) while Status holds spare width (all three).
3. The Name field disabled until hydration drops early typing (two).
4. C3: the dark theme's one blue (oklch .79 .045 250) reads gray; links blur into secondary text, the start button reads pale (two).
5. C5: no signature component (all three).

## Notes (one grader)
Empty name search ignores the default scope and says the Client does not exist (G2, real harm: a duplicate Client). "care or a request" reads as model words (G1). Title y differs by 5px between list pages with and without an action (G1). Seed: Mira Castellanos is "Intake · Contract signed" with the pregnancy ended (G2). Decorative empty-state icon (G3). Single-filter removal takes two actions (G3).

## Rubric change after this round
H3 no longer asks that the Name filter live in the address (#1829); recorded at the rubric's foot.
