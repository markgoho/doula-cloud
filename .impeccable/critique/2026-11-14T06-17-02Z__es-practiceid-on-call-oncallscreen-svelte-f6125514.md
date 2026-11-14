---
target: Staff On call, roster and rule settings (rubric baseline round 0; craft 16/20)
total_score: 12
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 21
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/2748-on-call-hill-climb/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte"
target_fingerprint: "sha256:368dab3781a400450405a14a0744a48cc619529c1b798c3dec516c18e30de0df"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/2748-on-call-hill-climb/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte
timestamp: 2026-11-14T06-17-02Z
slug: es-practiceid-on-call-oncallscreen-svelte-f6125514
---
Method: baseline round at 10bc9d42, before any change to the screens. Two routes, one rubric (`on-call.md`) plus craft; three blind graders, median per item. Blind walks by `persona-walker`: Renata (375×740, T1, T3, T10), Priya (375×740, T4), Lena (1280×800, T5), Dee (1280×800, T2, T8, T9, T10). Seed: 5 births on call in the next 30 days, 2 needing cover (a Narrowing that leaves Nov 23–29 with nobody; a gap tonight with nobody covering), a primary and a backup by Narrowing with a covered gap, Rowan and Priya each on two births, a birth with no due date. Not in the seed: a birth with no Doula yet, a birth with its own start rule.

## Usability 12/40 at the median (graders 12, 11, 12) · Craft 16/20 (18, 13, 16)

Medians: H1 1, H2 1, H3 2, H4 1, H5 1, H6 1, H7 1, H8 1, H9 2, H10 1. Craft: C1 3, C2 3, C3 4, C4 2, C5 4.

Jobs: T1 fail (3/3), T2 fail (3/3), T3 fail (3/3), T4 fail (3/3), T5 pass (3/3), T6 fail (3/3), T7 pass (3/3), T8 fail (3/3), T9 fail (3/3), T10 fail (3/3).

## Findings (two or three graders)

- H1d, T1: nothing on the roster marks tonight; Renata answered the 2 a.m. question from the Overview by joining three rows (3/3).
- H1a, T8: the settings form has no skeleton, draws 37 and 14 before its read lands, and keeps them with Save live when the read fails, so a press writes the usual answers over the Practice's rule (3/3).
- H1c, H10b: the result line states no days and no filters; at 375px the days are inside the collapsed "Filters" (3/3).
- H2a, H2d: "Window", "No window yet", "the product cannot say", "away" and "Covered by 2", where the Overview and the Engagement say "Cannot be reached" (3/3).
- H2b: this year's year on every day, no weekday in the next seven days, no clock on a gap ("Nov 14, 2026 – Nov 14, 2026" for 5pm to 9am), no `<time>` (3/3).
- H3c: Back from an Engagement loses the position (700 to 79; 600 to 79) (2/3; the third did not reach it).
- H3d: the settings route has no Back or Cancel (3/3).
- H4a, H8d, C1, C4: the births are a three-column card grid at 1280px (a table only from 84.75rem of container), and the first birth is not whole on the first screen at 1280×800 or 375×740 (3/3).
- H4b, T10: typed From and To in place of the Schedule's Days control; no presets, no Earlier or Later, no tags; a week took 4 taps on the phone (3/3).
- H4f, T6, H10c: no table has a caption or an accessible name; the births have no heading (3/3).
- H5a, H9a, T8: no `min` or `max` on the dates, 92 days never stated, and a To before From or a longer range replaces the page with "Something went wrong on our end" and loses the dates; the refused roster has no "Try again" that keeps the days (3/3).
- H5b: the rule's bounds are said only in the refusal (3/3).
- H5c: the warning says "every on-call window", which is false for a birth with its own start rule, and does not say births on call now can stop being on call (3/3).
- H6a: the order is not stated and the births needing cover cannot be put first (3/3).
- H6c, T3: the two-births flag counts the whole range, sits on the Doulas table's count only, and names no births or shared days; a two-day range reads "4 births on call, all covered." while Rowan holds two (3/3).
- H6d, T9: the rule is not on the roster and no row shows a due date; Priya and Lena each gave up looking for the rule; who changed it is in Activity only, with no values (3/3).
- T2: no filter to the births needing cover; a birth with no due date is not counted; the row opens the Engagement at its top with the On call heading at y=1078 and the gap control at y=1499; an existing gap has only "Clear", and "Who is covering" offers only the Doula who cannot be reached (3/3).
- T4: no "Mine" and no Doula filter; Priya's own row shows no days (3/3).
- H8c: one fact twice in a row (the badge and the line under it; a Doula's days repeating the "Window"); the empty result said twice (3/3).
- H8e: nothing guards a one-Doula Practice (3/3).
- H10a, H10d: whether To is included is unsaid; nothing says one birth can depart from the rule (3/3).
- C2: a date breaks inside itself in the cover lines at 1280px (1/3, a note).

## From the walks, outside any check

- After the rule moved to week 38, Camille's gap on Nov 14, now before her days, still counted as "needing cover" (Dee).
- The week view listed Lena with 0 births while she covered Morgan in it, and Marisol read "Away" today for a gap three days off (Renata).
- Lena's roster response carries a colleague's reason for a gap ("Her daughter's recital in Buffalo"), and the Engagement gives her "Clear" on that colleague's gap (Lena).
- A gap is recorded on one birth at a time; Priya wanted one place for all her births.

## Expected and missing

Named before reading the surface brief, with how many graders named each: presets (3), Earlier and Later (3), on call now first (3), Mine or a person filter (3), a needs-cover filter (3), a chosen order and a table at desktop (3), a timeline or calendar (3), the rule on the roster with a link (3), a back link on settings (3), last changed by (3), a preview before saving (3), an iCal feed (3), cover from the roster row (3), tap to call the Doula on call (2), bounds on the fields (2), a date picker that prevents bad ranges (1), tags (1), Cancel or discard (1). Every one is in the baseline tables.

Declines judged: "Cover edited from the roster itself: Declined" is sound only once the row opens the Engagement with the control in view (3/3); as built it is unsound. "A timeline: Later (#330)" holds for now (3/3). "Calling the Doula on call: Later, the founder's decision" is weak for a reader a rota product has trained and is correctly escalated (2/3).

A lone note for the baseline: the free Doulas offered where a gap's cover is chosen (When I Work and Deputy offer available staff for an open shift).
