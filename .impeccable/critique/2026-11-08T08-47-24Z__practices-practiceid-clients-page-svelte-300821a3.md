---
target: Staff Clients list (rubric round 7, due strip under test; craft 13/20)
total_score: 38
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 1
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:bc2f13f7f04642910f127c7e95a6f594aed069f30e888b5a63b39fe4be738230"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-08T08-47-24Z
slug: practices-practiceid-clients-page-svelte-300821a3
---
Method: rubric round 7 at a1fde0a4 (the due-strip experiment), usability + craft, three blind graders, median per item, plus a verdict on the strip.

## Usability 38/40 (graders 38, 37, 38) · Craft 13/20 (13, 13, 12)

Usability medians: H1 4, H2 4, H3 4, H4 4, H5 4, H6 4, H7 4, H8 3, H9 4, H10 4 → 38... (G2 H4 3, G3 H6 3, all H8 2–3).
Craft medians: C1 4, C2 3, C3 3, C4 3, C5 0 → 13 (C5 zeroed by the rubric rule: the strip made a usability check fail).

## Verdict on the due strip: it hurts (all three)
- It took width from Status: an overdue row wrapped to 78px at 1440, the round's only row failure at a rubric width.
- Its crowded-week amber clashed with the amber Owes badge, carried no visible words, and was computed over the loaded, filtered page, so the same Client changed color with the filter.
- Axis labels at ~12px, under the 16px floor, overlapping ("Today4 wk").
- Nothing in T1–T8 used it; the sorted date text already says what the dot says.
Reverted in 60e08e41.

## Other findings
1. Imani Brooks's overdue row also wraps to 78px at 1280 without the strip (regression from 09ca4c2d's roomier cells).
2. Clients' first column is flush with the page edge (flushStart) while Staff's is inset (C4).
3. Record card: the money line can start with an orphan "·" at 375.
4. Apply moves left when "Also show…" hides on the same row at 1920 (G1).
5. "care or a request" still waiting on Mark's words.

## Rubric change after this round
C5 now passes when DESIGN.md records that no signature candidate has earned its place (PRODUCT.md: at most one); recorded at the craft rubric's foot.
