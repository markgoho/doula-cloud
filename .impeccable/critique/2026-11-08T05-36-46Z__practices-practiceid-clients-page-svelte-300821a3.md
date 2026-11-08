---
target: Staff Clients list (rubric round 3; craft 16/20)
total_score: 30
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 3
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:6ce2dc1a7f7cdb1a3bfe147109972accc55174848ae884773f108f35cfc8f58f"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-08T05-36-46Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
Method: rubric round 3 at ccc0d3c6, usability + craft, three blind graders, median per item.

## Usability 30/40 (graders 32, 29, 33) · Craft 16/20 (15, 16, 17) — usability DOWN from 34

| # | Usability | G1 | G2 | G3 | Med |
|---|---|---|---|---|---|
| 1 | Visibility | 2 | 3 | 4 | 3 |
| 2 | Real world | 3 | 3 | 4 | 3 |
| 3 | Control | 4 | 3 | 2 | 3 |
| 4 | Consistency | 3 | 3 | 4 | 3 |
| 5 | Error prevention | 3 | 3 | 3 | 3 |
| 6 | Recognition | 3 | 3 | 3 | 3 |
| 7 | Efficiency | 4 | 3 | 3 | 3 |
| 8 | Minimalist | 3 | 1 | 2 | 2 |
| 9 | Error recovery | 4 | 4 | 4 | 4 |
| 10 | Help | 3 | 3 | 4 | 3 |

| C | Craft | G1 | G2 | G3 | Med |
|---|---|---|---|---|---|
| C1 | Specificity | 4 | 4 | 4 | 4 |
| C2 | Typography | 3 | 4 | 4 | 4 |
| C3 | Color | 3 | 3 | 4 | 3 |
| C4 | Rhythm | 2 | 2 | 2 | 2 |
| C5 | Signature | 3 | 3 | 3 | 3 |

## Why it fell (self-inflicted by round 2's fixes)
- Fixed column shares (a3b414d0) made Status and Doula cells wrap: rows 40/42/68/88px, ~400px empty beside the table (H8, C4).
- Unfolding the checkboxes at narrow widths (a3b414d0) plus the three-line Pilot banner put row 1 at y=637–693 of 740 at 375px and below the fold at 320 (T4 fails 2 of 3, H7, H8, C4).

## Findings (two or more graders)
1. Phone first screen: Pilot banner ~100–150px, full-width Apply, "Clear filters" on its own row.
2. Row rhythm: column shares too narrow for Status and Doula.
3. Doula options marked "(no Clients)" are still offered.
4. Back from a row past page 1 loses the loaded pages and position.
5. Count line hides the default filter (in care), and the default includes "Care ended" Clients; "Also show Clients not yet in care" is inert beside a narrowing.
6. Name and Doula fold behind "More filters" at narrow widths (Name is the T5 path).
7. Amber means "pending/waiting" on Overview, settings/delete and elsewhere.
8. No visible acknowledgement while a new filter loads (rows stay up, aria-busy only).
9. Due header over "Born"/"Ended" values; record cards show a bare date without "Due".
10. No signature component.

## Notes (one grader)
Layout shift 0.091 when the action's skeleton becomes the link; decorative empty-state icon; money not columnar; "Clear filters" baseline 5px below Apply; malformed ?doula= shows an unrecoverable load error.
