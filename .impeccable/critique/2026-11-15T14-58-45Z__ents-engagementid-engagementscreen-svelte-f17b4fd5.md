---
target: Staff Engagement record (rubric baseline round 0; craft 18/20)
total_score: 20
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 13
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2755-engagement-record-hill-climb/app/src/routes/practices/[practiceId]/engagements/[engagementId]/EngagementScreen.svelte"
target_fingerprint: "sha256:6f139ab2d3a56a7b140cc0a0f5b508e76ac1a7161ef9e7f112175e78b8de5610"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2755-engagement-record-hill-climb/app/src/routes/practices/[practiceId]/engagements/[engagementId]/EngagementScreen.svelte
timestamp: 2026-11-15T14-58-45Z
slug: ents-engagementid-engagementscreen-svelte-f17b4fd5
---
Method: baseline round at c4d06d22, before any change to the screen. Rubric `engagement-record.md` plus craft; three blind Opus graders, median per item. The stack was reseeded about a minute after the graders opened Camille Boyd's record (the seed build's own restart landed after they spawned), so every id changed and the shared browser went to /login. Each grader had one live capture at 1280×800 and graded the rest from source; checks only a live page could prove are marked unverified and not counted as fails. No persona walks (T1, T3, T4, T5, T7 unverified from a walk). Seed: Camille Boyd with ten weeks of history (started 70 days ago, Contract signed 56 days ago, a deposit paid and a balance due in 14 days, three past Visits with notes, eight Messages over three weeks, 33 Activity entries).

Measured by the orchestrator at 1280×800: first section (On call) at 1078px; page 7546px. At 1440×900: On call at 1100px, page 6846px; Birth Plan 1141px, Care Plan 644px inline; widths: summary 660, Visits table 1265, Invoices table 1215, Notice 1359, Messages 1359, Activity 680. At 375×740: page 10280px; first screen is back link, name, one link, ten chips.

## Usability 20/40 at the median (graders 20, 19, 22) · Craft 18/20 (15, 18, 18)

Medians: H1 1, H2 3, H3 2, H4 1, H5 4, H6 1, H7 1, H8 1, H9 3, H10 3. Craft: C1 4, C2 4, C3 3, C4 3, C5 4.

Jobs: T2 fails 3/3; T6 fails 3/3 (no Activity narrowing by kind); T1 fails 2/3 (Birth Plan uncapped, pain relief last of the fourth subsection), T5 fails 2/3, T8 fails 2/3, T10 fails 2/3 (no "Try again" on a section's failed read). T3, T4, T7, T9 unverified (no walks).

## Findings (two or three graders)

- 1b, 1c, 1c', 1d, 6a, T2: nothing at a glance; no status line under the name (care phase is the summary's third row, under a 10-chip "Jump to"); no on call tonight, next Visit, Contract state, owed or unanswered on the first screen (3/3).
- 4b, 4c: no facts column; the right half of the 1280 first screen is empty; `RecordDetail` has no facts region (3/3).
- 6b: the header holds "Message the Client" alone; "Add a Visit" is only in Visits (3/3).
- 2b: no weeks beside the due date (3/3).
- 8a, 8c, 8d, T1: first section below the first screen; page 7.6 screens at 1440; Birth Plan and Care Plan inline at full length (3/3).
- 6d: one section order for every phase; an empty Birth outcome sits second, above a signed Contract (3/3).
- 8f, C3: "Void this Contract" is a red fill on the signed Contract (`ContractStatus.svelte:448`) (3/3).
- 3c: the Birth Plan page's back link goes to the record's top, not `#birth-plan` (3/3).
- 3d: no "Back to top" on a 7546px page (3/3).
- T6, 10a: Activity cannot be narrowed by kind; the BFF serves no kind parameter (3/3).
- 6c: Invoices' first line is "Contract price", not what is owed (2/3).
- T10, 9a: a section's failed read shows a Notice with no "Try again" (`EngagementScreen.svelte:2879`, `:3237`) (2/3).
- 4a, C2: no width rule; tables, Notice and thread run 1215–1359px (2/3, the third unverified).
- 3e: no `aria-current` on the top bar (1/3 measured, 2 unverified) — note.
- Voice: "Failed to …" messages (`EngagementScreen.svelte:306`, `:588`, birth-plan) — note (1/3).

## Expected and missing

- Client's phone and email on the record, tap to call (HubSpot, Jane): 2/3 (A, C), not in the baseline. Finding under H4; the orchestrator adds it to the baseline as Build.
- Destructive acts in an overflow menu (Stripe, Linear): 2/3, both judged row 26 (void last in its section, secondary) a sound answer.
- Every Build row (status, facts column, header moves, next Visit, owed, unanswered, weeks, kind filter, Back to top): named 3/3, not built yet.
- Declines judged: sticky nav holds once Back to top is built (3/3); breadcrumb does not hold until 3e is built (1/3); shortcuts Later (#2956) holds; next/previous and copy link hold.
