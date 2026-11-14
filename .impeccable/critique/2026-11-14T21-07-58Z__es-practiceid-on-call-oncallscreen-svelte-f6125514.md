---
target: Staff On call, roster and rule settings (rubric round 4; craft 18/20)
total_score: 37
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 4
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/agent-a5873f8939b686a5d/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte"
target_fingerprint: "sha256:f82c52d0179d7eb2548f46d6f1d2228195571f54fed1c2756a8b950132c7ac69"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/agent-a5873f8939b686a5d/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte
timestamp: 2026-11-14T21-07-58Z
slug: es-practiceid-on-call-oncallscreen-svelte-f6125514
---
Method: round 4 at 4431fe32 (the app as #2942 left it at 4ec4d12f; 4431fe32 adds only round 3's snapshot), owed by #2932. Three blind Opus graders, median per item. Blind walks by `persona-walker` at this SHA: Renata (375×740, read only), Priya (375×740, signed in as Priya), Lena (1280×800, signed in as Lena), then Dee (1280×800, keyboard only after sign-in; covered Camille Boyd's gap with Bethany Kroll and undid it, saved the rule at week 39 and undid it, a refused week of 50, To before From, a range over 92 days, Offline and "Try again"). The three read-only walks ran at once, each in its own named chrome-devtools context; Dee's, which writes, ran after them. No walk showed a defect needing a fix build before the graders. Every grader worked in chrome-devtools only, in one named context ("oncall-r4") the orchestrator signed in as Renata. A stale round-3 grader file in the shared scratchpad was moved aside about three minutes into grading, before grader B wrote its own; B's report shows no sign of it.

## Usability 37/40 at the median (graders 37, 38, 38) · Craft 18/20 (18, 19, 18)

Medians: H1 4, H2 4, H3 3, H4 4, H5 3, H6 4, H7 4, H8 3, H9 4, H10 4. Craft: C1 4, C2 3, C3 4, C4 3, C5 4.

Per grader (A/B/C): H3 4/3/3, H4 3/4/4, H5 3/3/4, H8 3/4/3, the rest 4/4/4. Craft C2 3/3/3, C4 3/4/3, the rest 4/4/4.

Jobs: T1–T10 pass 3/3 (T4, T5 from Priya's and Lena's walks; T7 from Dee's keyboard walk and the graders' own; T9's save and audit line from Dee's walk).

Against round 3 (37, 19): the same usability total on different items. #2942's fixes hold on the page: one Doula named once (H8c), the Days radios in the Schedule's order (H4b), the births caption's count, a stepped "Later" that leaves From and To shut, refused days that draw no result line. New two-grader fails: H3c, H5a, H8b, and C2 now 3/3. Plateau rule (`grading-round.md` step 7): two rounds at 37 on different failed items, so the founder chooses to accept or continue.

## Findings (two or three graders)

- H3c: the Engagement's "Back to On call" link keeps the filter but opens the roster at its top, not at the row (Dee act 7: scrollY 118, focus on MAIN; B, C). The browser's Back keeps both (A, B, C measured). No open ticket.
- H5a: To's `min` stays at today after From is set, so a To before From can be picked; the error comes only after Apply (A, B; round 3 had it 1/3). #2944.
- H8b: births rows differ by about 2px, not whole lines (Casey 122.8–123px against Quinn 124.7–125px; Morgan 151.3 against Camille 153.1) (A, C; C counts it under C4 too). No open ticket.
- C2: the result line has no measure and runs about 100–140ch at 1280px (3/3). No open ticket.
- C4: one grader on the 2px rows (C), one on Apply sitting apart from its controls at 375px and moving from (953,400) to (250,618) when "Choose days" opens at 1280px (A). Different checks, so no single finding; the 2px half is H8b's.

## Notes (one grader, or outside any check)

- H4 cross-cutting: two tags plus "Clear filters" wrap to their own row at 1440px and move the result line and roster 67px (A). #2734.
- Apply, every page load and the Engagement's Back link drop focus to the body (Dee acts 2–4, 7, 10, 12–13, 16, 21, 25; three graders as a note). No open ticket.
- Refused days hide the whole roster and leave a stale "Next 7 days" tag beside the checked "Choose days" (Dee act 23; three graders as a note); the refused range has no "There is a problem" summary as the rule page does (Dee). Hiding the roster is #2942's recorded choice; the stale tag and the summary are not ticketed.
- The rule page keeps "On-call rule saved … week 37" beside a typed 50 until Save (Dee act 19; B, C as a note), and checks nothing before the server. No open ticket.
- "Later" relabels Order "Tonight first" to "Soonest due first" and no Days radio is checked after it (Renata acts 8, 10, 12; Lena act 3; Dee act 25; three graders as a note). The relabel is DESIGN.md's recorded rule ("'Soonest due first' once the days shown do not include today"); the unchecked radio is not ticketed. After "Later" the previous range's roster is still fetched again with no click (Lena act 3; the orchestrator's network log: the new range, then `from=2026-11-14&to=2026-12-13` again). No open ticket.
- Filtered to one Doula and Tonight, Camille Boyd's card drops Bethany Kroll's line, so the birth reads as Priya's alone (Renata act 10; A). No open ticket.
- At 375px the panel stays open after a change applies (Priya act 6, Renata acts 6, 13; B). #2944.
- "3 need cover, 1 tonight" reads cryptic (B, C). The empty result is said twice to a screen reader, by the hidden status and the EmptyState (A). No open ticket.
- Offline says "did not load", never that the connection is down (Dee act 25). #2944.
- Engagement, outside this rubric: the gap's Activity line says only "changed", twice alike (Dee act 11). #2945. The gap form starts on "Choose a Doula" with one option, focus stays on the body when it opens, and tonight's recorded gap is not mentioned in it (Priya act 8); "Who is covering" sits 17 Tab stops past two date-times and ArrowDown does not change it (Dee act 6). Partly #2934; the rest no open ticket. A contractor's Engagement shows Marisol's private reason and offers Change on Marisol's Visits, "Add a Visit", "Request void" and "Start the Care Plan" (Lena act 6; three graders as a note). No open ticket.
- Settled by hand in chrome-devtools (#2932's AC): Choose days, From typed keystroke by keystroke "12202026" reads `2026-12-20`; To "01052027" reads `2027-01-05`; after Apply the address is `?from=2026-12-20&to=2027-01-05`, both fields hold, and the tag reads "Days: Sun Dec 20 to Tue Jan 5, 2027". Lena (act 5) and Renata (act 12) typed ranges that held too.

## Expected and missing

Named before reading the surface brief: a timeline or calendar grid (3), calling or texting the Doula (3), covering a gap from the row (3), a calendar feed (3), search by Client (3), sort by a column heading (2), a notice when a gap opens (2, both Have it), an "on call now" summary (1, built as the Tonight group).

Declines judged: the timeline and the feed, "Later (#330)", hold (3/3). "Cover from the roster: Declined" holds (3/3, measured one action away with the control in view). "Search by Client: Declined" holds (3/3). Sort by heading, answered by the Order select, holds (2/2). "Calling the Doula: Decided, not yet built (#2937)" holds (3/3); Lena reached for it (act 2). Every Build and Have it row is on the screen (3/3). Nothing two graders named is missing from the baseline.
