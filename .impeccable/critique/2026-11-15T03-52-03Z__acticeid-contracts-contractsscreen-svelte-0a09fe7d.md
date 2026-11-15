---
target: Staff Contracts list (rubric round 2; craft 19/20)
total_score: 39
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 1
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2749-contracts-hill-climb/app/src/routes/practices/[practiceId]/contracts/ContractsScreen.svelte"
target_fingerprint: "sha256:78e6481db00cd387557a8b15e28a24cb82c55dfe38e33968e8bf79dcc7bbb322"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2749-contracts-hill-climb/app/src/routes/practices/[practiceId]/contracts/ContractsScreen.svelte
timestamp: 2026-11-15T03-52-03Z
slug: acticeid-contracts-contractsscreen-svelte-0a09fe7d
---
Method: round 2 at 032873ff, after the build from round 1's failed checks (the default view's order, a column only where the view can fill it, the void requests on three tracks). Four blind walks by `persona-walker` at 032873ff: Dee (1280×800, pointer then keyboard only), Renata (1280×800), Priya (375×740, touch), Lena (1280×800). No walk showed a defect, so no fix build ran; the stack was started again before the graders, because Dee's walk sent a Draft. Three blind graders, median per item, all in chrome-devtools as Renata.

## Usability 39/40 at the median (graders 40, 39, 38) · Craft 19/20 (18, 19, 18)

Medians: H1 4, H2 3, H3 4, H4 4, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Jobs: T1 to T10 pass (3/3 each).

Round 1's three findings hold on the page: what is not signed lists the Contracts sent, 41 days down to today, then the three not sent (H6a, 3/3); no view draws a column it cannot fill (H8e, 3/3); each void request's reason sits in a track of about 72ch (C2, 2/3; the third measured 85 to 90ch on the longest).

## What holds each lost point

- H2, two graders, a different check each, so no finding by the two-of-three rule and still a 3 at the median. B, H2d: June Okonkwo's Draft reads "made Nov 13" on the list and "Created Nov 14, 2026" on her Engagement (Dee's walk). The second is the day the Engagement was made, and the seed backdates a Contract to before its Engagement existed: a seed fault, not two readings of one instant. C, Voice: at 375px a card with no Doula reads "Doula No Doula yet".
- C4 (3/3): the title sits 40px from its intro and the intro 36px from the next section; one grader adds that one row of nine is on the first screen at 1280×800. `ListPage`'s gap, every list's: #2957.

## Notes (one grader, or the walks)

- H4d: no reminder or send-again (C; Dee's walk reached for it again). #2951, the founder's.
- "Void asked" on the row, "Void requested" on the Engagement (A, C). #2958.
- For about 300ms after a Show change the old rows stay, marked busy, under the new tag (A; Dee, Lena).
- The download's confirmation is at the end of the result line and easy to miss (B; Renata).
- On a card "Sent by …" and "Doula …" are divided only by a dot a screen reader does not hear (B).
- The Voided view names who sent a Contract, not who voided it (C; Renata). #2958.
- The table ends at 1079px in the default view and the queue at 1238px (C).
- "Joss Adeyemi +1" hides Lena's own name (A; Lena). #2958.
- "Client, A to Z" orders by the name as written, given name first (Lena).

## Expected and missing, before the brief

Named by all three: a reminder from the row, sort by a column heading, a count beside each view, a date range, an export, expiry, whether the Client opened it. Named by two: bulk selection, a row menu, saved views, search as it is typed. Named by one: who voided a Contract and why, on its row (not in the baseline as a row fact; #2958 builds it), starting a Contract from the list. Nothing two graders named is missing from the baseline. Declines judged: every decline holds (3/3), the count beside each view "weakly" (3/3); #2951 holds as sequencing for two and does not hold for one; #2954 holds, weakly against the Invoices list's own "Download CSV" (2/3).
