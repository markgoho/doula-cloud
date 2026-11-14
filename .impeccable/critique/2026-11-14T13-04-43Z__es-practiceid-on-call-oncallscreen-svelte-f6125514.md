---
target: Staff On call, roster and rule settings (rubric round 1; craft 18/20)
total_score: 33
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 6
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/2748-on-call-hill-climb/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte"
target_fingerprint: "sha256:340f58b3e8857d57bf9c5661dcabcc8d963d16e8601ed74c290757fd53975bcd"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/2748-on-call-hill-climb/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte
timestamp: 2026-11-14T13-04-43Z
slug: es-practiceid-on-call-oncallscreen-svelte-f6125514
---
Method: round 1 at 05da9e3a, after the arrangement's build (the BFF, the shared day range, the roster, the rule's settings, the Engagement's On call section) and two fix builds made from the blind walks before any grader looked. Three blind graders, median per item. Blind walks by `persona-walker`: Renata (375×740; walked twice, the second at 7780e98d), Priya (375×740, at 81a54cc3), Lena (1280×800, at 7780e98d), Dee (1280×800, at 7780e98d; saved and undid a gap's cover and the rule). Seed: 6 births on call in the next 30 days, 3 needing cover (a gap tonight with nobody covering on a two-Doula birth, a Narrowing that leaves Nov 23 to 29 with nobody, a birth with no Doula), a covered gap, Rowan on two births on the same days, Priya on two births whose days do not meet, a birth with its own start, a birth with no due date, two recorded rule changes. Grader B was stopped by a usage limit before it had looked at a screen and was resumed on the same tree.

## Usability 33/40 at the median (graders 31, 33, 35) · Craft 18/20 (19, 18, 18)

Medians: H1 4, H2 3, H3 4, H4 3, H5 4, H6 4, H7 1, H8 2, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 2, C5 4.

Jobs: T1 fail (3/3), T2 pass (2/3), T3 pass (3/3), T4 fail (3/3), T5 pass (2/3), T6 fail (3/3), T7 pass (3/3), T8 pass (3/3), T9 pass (3/3), T10 fail (2/3).

## Findings (two or three graders)

- T1, T4, H8d, C4: at 375×740 the first birth's card runs from y=521 to 805, so "Nobody tonight" and the days sit under the fold for Renata and for Priya (3/3). At 1280×800 the first row (581 to 732) is whole.
- T6, H4f: on a card the cover is read first, under the "On call" label, with no label of its own, so the card's order is not the table's (3/3).
- H8c: a card says "Due: · Due Fri Dec 4" and "Days on call: On call until Fri Dec 18", the hidden label and the visible word (2/3; the third named it under T6).
- T10: a typed From and To takes 4 actions (Choose days, From, To, Apply or Enter) against the rubric's 3 (2/3). The budget predates the rubric's own requirement that the days be chosen with the Schedule's control, where the fields are under "Choose days"; corrected in the rubric the same day.
- H2: the reader is not "you" everywhere: "Lena Vasquez covering" on her own row while the Doulas table says "You", and Renata named in the idle line (1/3, Voice); a covered gap reads "covered" in the Doulas table under a filter and "Lena Vasquez covering" without one (1/3, H2d). Two graders, one heuristic.
- C4: "Later" sits about 80px right of the radios at 1280px beside the unseen "Earlier", and alone on an indented line at 375px (2/3).

## Notes (one grader, or outside any check)

- T2: a birth with no Doula opens an On call section with nothing to act with (1/3).
- T5: a contractor reads "On call for no birth in these days: Joss Adeyemi", a colleague's count (1/3).
- H6b: on a birth with no Doula the Cover cell is the badge alone; "No Doula yet" is in another column (1/3).
- H8b: one-line rows are 41px with a badge and 40px without (1/3; two graders passed it as rounding).
- C4: the columns shift 8px when "Needing cover" is applied (1/3).
- The first day of a two-births overlap is never said, only "until Sun Nov 22" (Renata, Dee, and all three graders as a note).
- Under "Needing cover" the Doulas table still lists births the filter has hidden (Renata; two graders as a note).
- "Nobody tonight" under a second Doula's line read to Priya as if nobody is on call.
- The rule's sentence is under the births; Dee looked for it near the top.
- The Engagement's On call section says "a doula" and writes this year's year on its days.
- A refused week's error stays after a valid week is typed, until Save (Dee).
- The save of a gap's cover confirms with the same sentence whoever now covers (Dee).

## Expected and missing

Named before reading the surface brief: a timeline or calendar grid (3), a calendar feed (3), calling or texting the Doula on call (3), cover made from the roster's row (3), search by Client (3), a preview before saving the rule (3), print or export (2), the rule's full history of changes (2), an alert when a gap opens (2), sort by a column heading (2), Earlier and back to today (2), Cancel or Back on settings (2), "on call now" first (1), a date-range picker (1), saved views (1). Not in the baseline as written: the rule's full history (the baseline builds who last changed it and when); sort by a column heading (the baseline builds a chosen order as a select); saved views (one grader, who would not insist).

Declines judged: "Cover edited from the roster itself: Declined" now holds (3/3: one action from the row with the control in view), except for a birth with no Doula (1/3). "Search by Client: Declined" holds for one bounded page (2/3); weak on a phone, where the browser's find is hard to reach and the sibling Schedule has a Client box (1/3). "A timeline: Later (#330)" holds (3/3). "Calling the Doula on call: Later, the founder's decision" holds as escalated (3/3).
