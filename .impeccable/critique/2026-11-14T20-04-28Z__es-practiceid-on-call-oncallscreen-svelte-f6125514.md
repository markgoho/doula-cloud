---
target: Staff On call, roster and rule settings (rubric round 3; craft 19/20)
total_score: 37
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 3
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/agent-a0b0ecd923ac0ea80/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte"
target_fingerprint: "sha256:47a46239a91b90f2d625e3139ac6eb5a42ceacde3ee508f25c0a1ef5892aa7a8"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/agent-a0b0ecd923ac0ea80/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte
timestamp: 2026-11-14T20-04-28Z
slug: es-practiceid-on-call-oncallscreen-svelte-f6125514
---
Method: round 3 at 40b4b883, after #2931's build (PR #2941: below a 42rem bar the Days row folds with the other filters so the first birth fits 375×740, and a card's dates say what they are). Three blind graders, median per item. Blind walks by `persona-walker`, all four at 40b4b883: Renata (375×740), Priya (375×740), Lena (1280×800), Dee (1280×800; covered a gap and undid it, saved the rule at week 39 and undid it, refused days, a refused week of 50, Offline and "Try again"). No walk showed a defect, so no fix build ran before the graders. Seed as in round 2. Another climb's stack ran on another localhost port, whose cookies the default chrome-devtools context and Claude in Chrome share, so every grader walked in chrome-devtools only, in one named context the orchestrator signed in.

## Usability 37/40 at the median (graders 38, 37, 34) · Craft 19/20 (19, 19, 18)

Medians: H1 4, H2 4, H3 4, H4 3, H5 4, H6 4, H7 4, H8 2, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Per grader: H2 4/3/4, H4 4/3/1, H5 4/4/3, H8 2/3/2, the rest 4/4/4. Craft C2 4/4/3, C3 3/4/4, C4 4/3/3.

Jobs: T1 pass (3/3), T2 pass (3/3), T3 pass (3/3), T4 pass (3/3), T5 pass (3/3), T6 pass (2/3; one fail on the births caption's count), T7 pass (2/3; one unverified), T8 pass (3/3), T9 pass (3/3), T10 pass (3/3).

Against round 2 (32, 17): T1 and T4 now pass at 375×740 (Renata's and Priya's first cards whole on the first screen; H7 2→4, H8d passes); the card's visible labels answer round 2's H6 and H2a finding (H6 3→4); H4 rose from 2 to 3; H1 rose from 3 to 4; C4 rose from 2 to 3 and C2 from 3 to 4.

## Findings (two or three graders)

- H8c: a birth with one Doula names her twice, "Tonight: Rowan Petrosyan" and then "Rowan Petrosyan" (Casey and Quinn Estrada; 3/3). One grader adds Tamsin Reyes's "Nobody" and "No Doula yet". #2932 carries the one-Doula half.
- H4b: the Days radios run "Tonight, This week, Next 7 days, Next 30 days"; the Schedule's run "Next 7 days, This week, Next 30 days" (2/3). #2932 carries it.
- C4: on Tamsin Reyes's card at 375px, "Due Wed Dec 9 ·" ends on a separator with nothing after it and "No Doula yet" sits centered, off the card's left edge (2/3, and Renata's walk act 5). No open ticket.

## Notes (one grader, or outside any check)

- H2b: the two-births flag reads "Also Quinn Estrada, today to Sun Nov 22", with no exact day, and keeps "today" in a range that starts later (Nov 21–27) (1/3 fail; one more grader and Renata's walk as a note).
- H8b: at 1280px Morgan Grier's row is 151px against 153px rows, 2px off a whole line (1/3).
- H4 cross-cutting: two tags and "Clear filters" wrap to a row of their own at 1440 and 1280px and move the result line and the roster 67px (1/3). #2734.
- T6, H4f: the births table's caption "Births on call" carries no count, beside "Doulas (5)" (1/3). #2932.
- H5a: To's `min` stays at today after From is set, so To can be picked before From where the control could prevent it (1/3).
- H8e: the one-Doula spec checks only that the Doulas table is absent, not that no flag that cannot fire is drawn (1/3).
- C3: no `::selection` or `caret-color` (1/3; another grader found them styled). C2: the result line runs about 140 characters at 1280px (1/3).
- Walks, and every grader as a note: "Later" switches Order from "Tonight first" to "Soonest due first" by itself and drops "Tonight first" from the select (Renata, Lena); after "Later" the previous range's roster is fetched again with no click (Lena acts 3 and 12; two graders); "Later" still opens From and To (#2932).
- At 375px the filter panel stays open after a change applies, so the births sit below the fold until it is closed (Renata acts 7–11; one grader).
- Offline, the roster says "Who is on call did not load." and never that the connection is down; a reload offline shows the browser's own page, no app shell (Dee acts 25–26; two graders; may be the dev server, which runs no service worker).
- On the Engagement: a gap's saved confirmation, "The time Priya Raman cannot be reached is changed.", does not say who now covers, and the Activity's "Gap in on-call cover changed" says neither the cover before nor after (Dee acts 6, 10, 21; one grader). The gap form starts on "Choose a Doula" when the reader is its only option (Priya act 5).
- The rule's refused-week error stays after a valid week is typed, until Save (Dee act 17; GOV.UK's behavior). Under refused days the old result line and rows stay (Dee acts 13–14; #2932).
- Engagement Invoices, checked once on the rendered page with the Invoices read held 8 seconds: the section showed its heading and the loading skeleton until the read landed, and never "No Invoices yet." or "Create Invoice" (Camille Boyd's and Morgan Grier's Engagements, sampled every 50ms). #2935's build holds on the page.

## Expected and missing

Named before reading the surface brief: a timeline or calendar grid (3), calling or texting the Doula on call (3), covering a gap from the roster row (3), a calendar feed (3), search by Client (3), sort by a column heading (3), print (2), a notice or reminder (2: one Have it, one a reminder before a Doula's own days), "Earlier" drawn disabled with its reason at today (1), an "as of" time (1). Not in the baseline: a reminder before a Doula's own on-call days, "Earlier" disabled with its reason, an "as of" time (each 1/3, notes).

Declines judged: "A timeline: Later (#330)" holds (3/3, one as a deferral only). "Cover edited from the roster itself: Declined" holds (3/3). "Search by Client: Declined" holds (3/3, each adding it is weaker on a phone). "Sort by a column heading: Declined" holds (3/3). "Calling the Doula on call: Decided, not yet built (#2937)" holds (3/3). Every Build and Have it row is on the screen (3/3).
