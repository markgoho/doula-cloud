---
target: Staff Clients list (rubric round 4; craft 17/20)
total_score: 36
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 1
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:6e9313611bda6c0ea692a5b69a7d4f81c7f14dbd9b95a662cd3f63a649a873f6"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-08T06-34-50Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
Method: rubric round 4 at 2a3d0368, usability + craft, three blind graders, median per item.

## Usability 36/40 (graders 37, 37, 35) · Craft 17/20 (17, 15, 17)

| # | Usability | G1 | G2 | G3 | Med |
|---|---|---|---|---|---|
| 1 | Visibility | 4 | 4 | 4 | 4 |
| 2 | Real world | 3 | 3 | 3 | 3 |
| 3 | Control | 4 | 4 | 4 | 4 |
| 4 | Consistency | 4 | 4 | 4 | 4 |
| 5 | Error prevention | 4 | 4 | 4 | 4 |
| 6 | Recognition | 4 | 3 | 3 | 3 |
| 7 | Efficiency | 4 | 4 | 4 | 4 |
| 8 | Minimalist | 2 | 3 | 2 | 2 |
| 9 | Error recovery | 4 | 4 | 4 | 4 |
| 10 | Help | 4 | 4 | 3 | 4 |

| C | Craft | G1 | G2 | G3 | Med |
|---|---|---|---|---|---|
| C1 | Specificity | 4 | 4 | 4 | 4 |
| C2 | Typography | 3 | 2 | 3 | 3 |
| C3 | Color | 4 | 4 | 4 | 4 |
| C4 | Rhythm | 3 | 2 | 3 | 3 |
| C5 | Signature | 3 | 3 | 3 | 3 |

All tasks T1–T8 pass for all three graders; T4 fits with 4px to spare at 375.

## Findings (two or more graders)
1. H2: a postpartum line shows only "Born Oct 18, 2026", not the postpartum window; header "Date".
2. H8: the two-Doula row stacks to 74px against 40.
3. H8: the empty state repeats "Clear filters" (bar link, empty-state link, sentence).
4. H6: the Doula select folds behind "More filters" below 42rem.
5. C2: near-duplicate type sizes (labels 19.18px vs body 18.99px).
6. C4: spacing does not separate groups (bar internals equal bar-to-list); title and start button 6.5px off center; horizontal shift when the scrollbar comes and goes.
7. C2 (chronic, rounds 2–4): money is not in a column.
8. C5: no signature component.

## Notes (one grader)
"with care or a request" reads as system words (G1); count line over 75ch (G3); a name typed before hydration may be dropped (G1, G3, both unconfirmed); "More filters" summary 5px off the field row (G3); "Try again." text right above a Try again button (G2).
