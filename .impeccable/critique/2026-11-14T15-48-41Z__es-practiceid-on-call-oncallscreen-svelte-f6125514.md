---
target: Staff On call, roster and rule settings (rubric round 2; craft 17/20)
total_score: 32
max_score: 40
p0_count: 0
p1_count: 8
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/2748-on-call-hill-climb/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte"
target_fingerprint: "sha256:bdf1db725fac7d40d7ee958f777b606ecbd7c88814e2aa6d242652582049b64d"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/2748-on-call-hill-climb/app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte
timestamp: 2026-11-14T15-48-41Z
slug: es-practiceid-on-call-oncallscreen-svelte-f6125514
---
Method: round 2 at a8b707bb, after round 1's builds (a card in the table's order, "you" for the reader, each fact said once, the rule's changes with their values) and the extraction of what the roster shares with the lists. Three blind graders, median per item. Blind walks by `persona-walker`, all four at a8b707bb: Renata (375×740), Priya (375×740), Lena (1280×800), Dee (1280×800; saved and undid a gap's cover and the rule). Seed as in round 1. The founder chose to land after this round at whatever it scored and to file what is left.

## Usability 32/40 at the median (graders 33, 31, 30) · Craft 17/20 (14, 17, 17)

Medians: H1 3, H2 4, H3 4, H4 2, H5 4, H6 3, H7 2, H8 2, H9 4, H10 4. Craft: C1 4, C2 3, C3 4, C4 2, C5 4.

Jobs: T1 fail (3/3), T2 pass (3/3), T3 pass (3/3), T4 fail (3/3), T5 pass (3/3), T6 fail (2/3), T7 pass (3/3), T8 pass (3/3), T9 pass (3/3), T10 pass (3/3).

Against round 1 (33, 18): T10 now passes and H7 rose by one; H2 rose by one ("you" for the reader); H1, H4 and H6 each fell by one on findings round 1's graders did not reach, and C2 fell on a measurement two graders did not reach.

## Findings (two or three graders)

- T1, T4, H8d, C4: at 375×740 the first birth's card runs from y=521 to 825 (853 under Tonight), cut at the "Needs cover" badge, so why the birth needs cover is under the fold for Renata and for Priya (3/3). The Days radios, the fold row and the result line take about two thirds of the first screen. At 1280×800 the first row (581 to 734) is whole.
- H6, H2a: on a card "Days on call:" and "Due:" are drawn for a screen reader only, so a sighted reader gets "Until Fri Dec 18 · Fri Dec 4" and cannot tell which is the due date (3/3; both phone walkers stopped on it). Round 1's build took the visible words out to stop a fact being said twice.
- H8c, C4: a card with no second fact ends its line on a "·" with nothing after it, and "Covered" sits alone and centered on the next line (3/3).
- H1c: with a Doula chosen and no birth matching, the result line and the empty message drop the Doula: "No births need cover on Sat Nov 14." while another Doula's birth does (2/3).
- T6, H4f: the births table's caption is "Births on call" with no count, beside "Doulas (5)" and "Births with no days on call yet (1)" (2/3).
- H4: "Later" switches the radios to "Choose days", draws From and To and moves the roster down about 220px (2/3; Renata and Dee). At 1440px three tags wrap to a row of their own and clearing them moves the roster 67px (1/3).
- H8c: one Doula on a birth is named twice in the row, "Tonight: Rowan Petrosyan" and then "Rowan Petrosyan" (2/3).
- C4: in a table row Client, Days on call and Due are middle-aligned while On call and Cover are top-aligned (2/3).

## Notes (one grader, or outside any check)

- H6a: the result line says "tonight first"; the births with nobody tonight lead that group and the rest run by due date, which is not said (1/3).
- H4b: the Days radios run "Tonight, This week, Next 7 days"; the Schedule's run "Next 7 days, This week" (1/3).
- H1e: the rule's Save showing that it is saving was not evidenced (1/3; two passed it from source). H9b not reached (1/3; two passed it on the page). H4d not reached (1/3).
- Under a refused range the result line and the table still describe the range before it (Dee, Lena; three graders as a note).
- The rule's saved confirmation is drawn under Save, at y=670 of 800 after a scroll (Dee).
- A refused week's error stays after a valid week is typed, until Save (Dee; GOV.UK's behavior).
- Back to On call lands at the top with focus on the body when the birth has left the filter (Dee).
- Once its tag is removed on a phone, "Mine" is under "More filters and order" (Priya).
- Typed keystrokes in the date fields did not hold the year in Dee's walk; the walker's own log calls the attempt muddied by a failed click. Unverified on a real keyboard.
- A contractor's roster response lists a Doula who shares none of her births on call, with every staffId (Lena; 1/3 as a fix, 2/3 as allowed).
- Outside this surface, on the Engagement: a contractor reads another Doula's reason for a gap and has "Change" on that Doula's Visits (Lena); the Invoices section draws "No Invoices yet." before its read lands (Priya, Dee).

## Expected and missing

Named before reading the surface brief: a timeline or calendar grid (3), calling or texting the Doula on call (3), cover offered to a free Doula from the gap (3), marking oneself unreachable once across every birth (3), a calendar feed (3), sort by a column heading (3), a "this month" choice (2), visible labels on a phone's cards (2), print or export (2), results ahead of filters on a phone (1), search by Client (1), who is free shown first (1), reset to default (1). Not in the baseline: visible labels on a phone's cards (also a failed check), "this month", results ahead of filters on a phone.

Declines judged: "Cover offered to a free Doula: Declined" holds as a domain rule (3/3), and each grader adds that nothing leads from "nobody covering" to putting a free Doula on the birth. "A timeline: Later (#330)" holds (3/3). "Calling the Doula on call: Later, the founder's decision" holds as escalated (3/3). "Sort by a column heading: Declined" holds (2/3; borderline 1/3). Every Build and Have it row in both baseline tables is on screen (3/3).
