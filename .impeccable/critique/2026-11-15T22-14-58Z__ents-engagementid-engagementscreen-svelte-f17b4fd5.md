---
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2755-engagement-record-hill-climb/app/src/routes/practices/[practiceId]/engagements/[engagementId]/EngagementScreen.svelte"
target_fingerprint: "sha256:748b2a9775e21057de91638352c0c181199da54663f61f26240354daadea0650"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2755-engagement-record-hill-climb/app/src/routes/practices/[practiceId]/engagements/[engagementId]/EngagementScreen.svelte
timestamp: 2026-11-15T22-14-58Z
slug: ents-engagementid-engagementscreen-svelte-f17b4fd5
---
Method: round 1 at f416fb55. Rubric `engagement-record.md` plus craft; three blind Opus graders, median per item, in chrome-devtools as Renata at 1440×900, 1280×800, 640, 375×740, 320, both themes, forced colors. Persona walks (blind persona-walker, isolated browser contexts) at 563c2f5c: Priya at 375 (T1, T3, T4), Dee at 1280 (T5, T6), Lena at 375 (T7); act logs in `round1/`. One defect the walks found (a Visit's Change form opened after the last card) was fixed before the graders (f416fb55), with the finish review's fixes.

## Usability 32/40 at the median (graders 31, 33, 33) · Craft 18/20 (17, 17, 18)

Medians: H1 3, H2 3, H3 3, H4 4, H5 4, H6 3, H7 4, H8 1, H9 3, H10 4. Craft: C1 4, C2 3, C3 4, C4 3, C5 4.

Jobs: T1, T2, T6, T8, T9 pass 3/3; T3, T4, T5 pass on budget (saves unverified by rule); T7 passes 2/3 (A fails it: a contractor sees Invoice states, ADR-0008:344); T10 passes 1/3, unverified 2/3.

## Findings (two or three graders)

- 8a: at 1280×800 On call's first row (the coverage gap) ends at 872px, past 800 (3/3).
- 8c: Camille's page is 6605px at 1440×900, 7.3 screens against 3600 (3/3).
- 8h: four decisions on the 375 first screen by PR-G10's count (the two moves, Jump to, the facts' links) against 3 (3/3).
- 3e: nothing in the top bar carries `aria-current` on the record (3/3).
- 6d: empty sections above filled ones (Camille's Offers and Birth outcome above Details; Imani's Care Plan and Messages; Grace's Messages above Invoices) (2/3; A passed it by judgment).
- C4 / notes: the back link reads three ways ("Clients", "Back to Contracts", "Back to Camille") (3/3 notes).
- Expected and missing, not in the baseline: an overflow actions menu (3/3) — the orchestrator adds it as Declined (destructive acts sit last in their own sections, row 26; two graders judged that sound); internal Staff notes the Client never sees (2/3) — added as a filed question for the founder.

## Notes (one grader)

- T7 (A): Lena sees Morgan's Invoice states (Owed, Void); A cites ADR-0008:344.
- 9c, Voice (C): a refused Activity "Load more" shows raw "Failed to fetch".
- Offline Engagement read says "Something went wrong on our end" (C; ADR-0059).
- 1a (B): no skeleton under Slow 4G on the dev server; to re-measure on a preview build.
- 2d (B): "No Messages yet" in the facts, "No messages yet." in the section.
- C2 (B): the Visits When cell splits date and time with a wide gap; (C) h1 35.6px against h2 28.2px at one weight.
- C5 (A) / 8g (C): "Nobody on call tonight" said three times on one first screen (header Badge required by 1c', the fact, the section line); the rubric does not say which wins.
- 8e (A): Grace's empty Invoices section is four lines.
- Activity filters are `<a href="#activity">` toggles with `aria-current="page"`, not in the address (A).
