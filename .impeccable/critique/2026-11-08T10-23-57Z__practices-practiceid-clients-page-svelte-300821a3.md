---
target: Staff Clients list (rubric round 10; craft 19/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 1
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:38e71720e567235bf218ee54b3bb0412c90536ba2f104ca9f0b9b194eaf4e077"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-08T10-23-57Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
Method: rubric round 10 at 80ab68eb, usability + craft, three blind graders, median per item.

## Usability 39/40 (graders 39, 38, 40) · Craft 19/20 (19, 19, 19)

Usability medians: all 4 except H8 3 (G1 3, G2 3, G3 4). G2 also failed H4 (ADR-0024: the table breaks 1024–1180px).
Craft medians: C1 4, C2 4, C3 4, C4 3, C5 4.

All tasks T1–T8 pass for all three.

## Root cause of what's left
The five Clients columns use fixed shares tuned to the pixel at 1280:
- 1280: two-word names wrap (Name share 2.76) → 54px rows while Status has ~300px spare (G1, G2).
- 1024–1180: the date prints over the Doula name, "Money" breaks to "Mone/y", most names wrap (G2) — ADR-0024.
- Under a Doula filter Money and Date jump right by the removed column's width (G1).
- Badge rows at 1920 are 41.6px (G3, within the 4px check).

## Smaller (one grader each)
Record card: date margin doubles the gap before "·" (G1, G2). "Ended" nearly touches its date at 5.2ch (G1). Doula text flush to the table's end edge (G1). Result line floats on no column edge (G3).
