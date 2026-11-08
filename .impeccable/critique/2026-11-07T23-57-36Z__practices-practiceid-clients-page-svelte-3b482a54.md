---
target: Staff Clients list
total_score: 25
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte"
target_fingerprint: "sha256:0c4e9ce2702863b178672160c14616f73e3980f489fe9345cc501c8067c9ddf6"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/practices/[practiceId]/clients/+page.svelte
timestamp: 2026-11-07T23-57-36Z
slug: practices-practiceid-clients-page-svelte-3b482a54
---
Method: dual-agent (A: design review · B: detector/overlay)

## Design Health Score: 25/40 (Acceptable), up from 23

| # | Heuristic | Score | Key issue |
|---|---|---|---|
| 1 | System status | 3 | "Also show Clients with no work yet" changes no visible row on page one (created_at DESC order) |
| 2 | Real world match | 3 | No due date or birth window, the doula's own axis |
| 3 | Control/freedom | 3 | Filters in URL |
| 4 | Consistency | 3 | Shared templates; primary action is a text link |
| 5 | Error prevention | 3 | Read-only; stale Load more abandoned |
| 6 | Recognition | 2 | Order unstated (newest record first) |
| 7 | Efficiency | 1 | No sort, no attention filter, no Doula filter, 30 per page |
| 8 | Minimalist | 3 | Calm; "No Contract yet" amber on ~20 of 30 rows becomes wallpaper |
| 9 | Error recovery | 2 | Raw error text; Try again present |
| 10 | Help | 2 | "with work" unexplained |

## Design Specificity Verdict
Frame generic; Status cell (calls as badges, facts muted, Intake-without-Contract a fact) is product-specific. Detector: 0 findings in page, ListPage, DataTable; overlay clean on default and ?all=true.

## Priority Issues
1. [P1] No usable order or attention view: sort by attention (Overdue > Owes > Request > Active no Contract), say the order, add "Needs action". BFF work, #2724.
2. [P1] No time axis: Due / postpartum window column, sortable. BFF field.
3. [P2] 30 per page for an agency roster: larger page or inline name filter.
4. [P2] Facts run together for screen readers ("IntakeContract signed"). FIXED in 12bd21ed.
5. [P2] "No Contract yet" warning fatigue: count once in the filter bar as a link, or quieter style.

## Persona Red Flags
- Renata: no "who owes" view, no Doula filter.
- Priya: lands on the whole agency, not her own; no due dates.
- Screen reader: run-together facts (fixed); no table caption.
- Keyboard: Load more every 30; search is a separate page.
- Phone: long card scroll; toggles push first card down.

## Minor Observations
Primary action reads secondary; mixed-polarity toggles under one legend; "Nothing open" rows among current ones; badge rows 47.5px vs 40px; system-voiced error defaults.
