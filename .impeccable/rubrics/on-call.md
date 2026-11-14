# On call: what a 4 means

Two routes, one rubric: the roster (`on-call/`) and the rule that makes it (`settings/on-call/`). They are one task for the person using them (#2748), so a heuristic's checks read both and a failure on either costs the point.

Each heuristic's checks are lettered, one claim to a letter. A heuristic scores 4 only when **every** lettered check passes, each with evidence a grader can point to: a screenshot, a measured value, the accessible text, or a walk's act log. A check with no evidence fails. 3 = one lettered check fails, 2 = two, 1 = three or more, 0 = the heuristic's core job is absent. A job (T1–T10) costs its point under the heuristic that names it: T1–T4, T9 and T10 under 7, T5 under 5, T6 and T7 under 4, T8 under 9.

Graders are three independent agents per round. None sees an earlier score, this branch's commits, or another grader's output. The score per heuristic is the median of the three.

In this rubric a **filter** is a control on the screen. A **Narrowing** is the domain fact (`GLOSSARY.md:49`): the part of a birth's on-call days one Doula holds.

Seed: the demo Practice under `dev:full` (Rooted Birth Collective, 14 Doulas), every time of day relative to the stack's current day in the Practice's zone. In the next 30 days: at least five births on call, three or more of them on call tonight; one covered by one Doula; one covered by two by Narrowing, each with her own days (Morgan Grier: Marisol, then Lena, a contractor); one whose Doula's days leave days with nobody (Casey Estrada); one with a Coverage gap tonight and nobody covering (Camille Boyd, Priya); one gap with a covering Doula; Rowan and Priya each on two births whose days overlap; two Clients of one family name (Casey and Quinn Estrada); a birth with a Doula and no due date (Imogen Okafor); a birth with a due date and no Doula yet; a birth with its own start rule. Priya is on two births, one on call tonight. A recorded change to the Practice's rule exists. Widths: 1440px, 1280px, 768px, 375px; 200% zoom at 1280px.

## The jobs

Each is a hypothesis from the persona records (`docs/personas`, `docs/journeys`, `docs/test-plans`), not research (ADR-0045). The records name the jobs; the action budgets are ours. A press of Apply is an action.

- **T1 Renata, phone (375px), tonight:** "If two Clients go into labor the same night, she needs to know within a minute who is free and who is already at a birth." On arriving on On call with nothing chosen (the screen keeps its 30 days, #2026, RA-G17), the births on call tonight come first under their own heading and are told from later births without reading a date range; the first of them is whole on the first screen (375×740), and the result line says how many births tonight have nobody. Each birth on call tonight says who holds tonight: the Doula whose days include today, the covering Doula where a gap is in force, or that nobody does. "Tonight" alone is ≤ 2 actions. The births and Doulas the tonight view names are the ones the Overview's "Who is free tonight" names, in the same words. (`personas/practice-owner.md:15`, `journeys/practice-owner.md:158`, `journeys/practice-owner.md:173`, `journeys/practice-owner.md:209`)
- **T2 Renata or Dee, where cover is needed:** in the days shown, find every birth that needs cover: a birth on call with days nobody holds, a gap nobody covers, or no Doula on it at all. The result line says how many, and ≤ 2 actions show only those. Each such row says everything that is missing (the days with nobody, and each Doula who cannot be reached with from when to when and that nobody covers), not the first thing only. In 1 action from the row, the Engagement opens with its On call section's heading and the control that covers a gap or changes a Doula's days both in view at 1280×800. After covering the gap there, Back returns to the roster with the filter and position kept, and the birth no longer counts as needing cover. (`personas/practice-owner.md:11`, `personas/non-doula-admin.md:11`, `journeys/practice-owner.md:51`, `GLOSSARY.md:47`)
- **T3 Renata, who holds two at once:** see which Doulas are on call for two or more births on the same day in the days shown, with the births and the shared days named, without counting rows: the flag is on her row of the Doulas table and beside her name on each of those births, and when any Doula holds two at once the result line says so and never says "all covered" alone. One Doula's births alone are shown in ≤ 2 actions. A Doula on two births whose days do not meet is not flagged. (`test-plans/practice-owner.md:132`, `journeys/practice-owner.md:210`, `personas/employed-doula.md:11`)
- **T4 Priya, phone (375px), when she is on call:** signed in as Priya, an employed Doula with no Owner or Admin role (walked as her by `persona-walker`, never judged from Renata's session), arriving by the nav, the roster opens on her own births ("Mine", which the nav's link turns on for a Doula, as #2870 decided for the Schedule; a tag she can clear to read the whole Practice, which ADR-0008 gives her), with the days she holds for each and anyone who shares the birth; the first is whole on the first screen (375×740). 0 actions to her births, ≤ 2 to everyone's. From her birth's row she reaches, in 1 action, the place she records a time she cannot be reached. (`personas/employed-doula.md:13`, `personas/employed-doula.md:17`, `GLOSSARY.md:47`)
- **T5 Lena, a contractor:** signed in as Lena (walked as her), the roster holds only the births she is on, with her own days; no other Client's name or due date and no colleague's birth count is on the screen or in the roster's response; a colleague's name and whether she can be reached still are; a Doula filter, if drawn for her, lists only what she may read; and no control or link offers her what the BFF refuses her (the rule's settings). (ADR-0008, `api/internal/oncall/roster.go`, `test-plans/contractor-doula.md:41`)
- **T6 Screen reader:** at 1280px each table's purpose and row count are announced, and a birth's row reads as the Client, who is on call with each Doula's own days where they differ, the days on call, then the cover, each under its column's name once, with audible separators. At 375px a birth's card reads the same facts in the same order, each with its label. "Needs cover" and "cannot be reached" are said in words, not by color or icon alone. Casey Estrada's and Quinn Estrada's links are told apart in a list of links.
- **T7 Keyboard:** every control and row link on both routes is reachable in drawn order with a visible focus ring; "Skip to results" reaches the result line in ≤ 2 stops and the first birth in one more; a radio group is one stop; nothing that is not a control takes a stop, and no control this reader is not offered does.
- **T8 Failure:** with the BFF refusing the roster, the screen says what happened in Voice-rule words, the days chosen are kept, and "Try again" recovers. A To day before the From day, and a range longer than the BFF serves (92 days), are each answered in place beside the fields in words that say what to do, never by the full error page. On the settings route a refused save (a week of 50) is said on its field and in the summary and the typed values are kept; and when the saved rule cannot be read, the form says so and offers no Save that would write the usual answers over the Practice's own.
- **T9 Dee or Renata, the rule:** on On call, read the rule in force (when on call starts and how long it runs past the due date) without leaving the screen, and reach its settings in 1 action. There, change the week from 37 to 39; before saving, read what the change does to the births on call now (how many stop being on call, and that a birth with its own start rule keeps it); save; read a confirmation that states the saved rule. Back on On call, the days have moved. Who last changed the rule, and when, is read on the settings route. (`personas/non-doula-admin.md:11`, `GLOSSARY.md:45`, CLAUDE.md "Audit trail", `api/internal/oncall/settings.go`)
- **T10 Renata or Dee, another stretch of days:** "Tonight", the next 7 days or the next 30 days in ≤ 2 actions; a typed From and To in ≤ 3; "Later" steps the days by their own length in 1 action and "Earlier" steps back as far as today, keeping every other filter; the result line and the address state the days; one action returns to the days the screen opens on. The roster shows births still under way and gaps still in force, so it offers no day before today and says why where a reader types one. (`test-plans/practice-owner.md:131`, `api/internal/oncall/roster.go`, the surface brief's baseline rows for the presets and the steps)

## Heuristic checks

1. **Visibility of system status**
   - a. Loading shows a skeleton within 300ms, on both routes, and nothing moves when the content lands.
   - b. Every filter change visibly changes the roster or says that nothing matched.
   - c. The result line states how many births are on call, how many need cover, the days, and every applied filter.
   - d. A birth on call tonight says so.
   - e. Saving the rule shows that it is saving, then that it saved and what was saved.
2. **Match with the real world**
   - a. Every label is a word a doula or an office manager uses. "Window", "narrowing" and "gap" do not appear unexplained; a row says "on call from … to …" or its like.
   - b. Every day reads as ADR-0022 writes one, the year only when it is not this one, with its weekday for any day in the next seven; a time a Doula cannot be reached reads with its clock as ADR-0022's 2026-10-24 amendment writes a Visit's ("Sat Nov 14, 6:00pm").
   - c. Days and times are the Practice's own (ADR-0036), and where the reader's zone is another the screen says which zone they are in.
   - d. A state the Engagement's On call section and the Overview's "Who is free tonight" also say (covered, cannot be reached, nobody on call, no Doula yet) is said in the same words here.
3. **User control and freedom**
   - a. Every filter is in the address (back, reload and share keep it).
   - b. Any filter is cleared in one action, and one action returns to the days the screen opens on.
   - c. Leaving for an Engagement and coming back restores the filter and the position.
   - d. The settings route has a way back to where the reader came from that changes nothing, and the roster is one action from it.
4. **Consistency and standards**
   - a. Title, intro, filter bar, result line and table are the components the Clients list, the Invoices list and the Schedule use, in the same place (measured: same title x and size, same filter bar arrangement, same result line, same table header and row style), and at 1280px the births are a table as on those lists.
   - b. The days are chosen with the Schedule's own control and words, and applied filters are tags as on those lists.
   - c. Badge colors and words mean what they mean on the other lists.
   - d. The settings route is arranged as its sibling Practice settings are (the Timezone screen): same title, field, hint, Save and saved message.
   - e. Every "Build" and "Have it" of the surface brief's two Category baseline tables is on the screen, and each "Declined" names a reason that holds for a reader those products have trained. An expected thing two of three graders name that the baseline omits, or a decline two of three find unsound, fails this check (`grading-round.md`).
   - f. T6 passes.
   - g. T7 passes.
5. **Error prevention**
   - a. A range that cannot be served (To before From, more than 92 days, a day before today) cannot be picked where the control can prevent it, and the longest range is stated before it is hit.
   - b. The rule's bounds (weeks 20 to 42, 0 to 42 days) are stated on the fields before a save is refused.
   - c. What saving the rule does is stated before the press, and it is true: births already under way move, a birth with its own start rule keeps it, and a birth on call today can stop being on call.
   - d. T5 passes: a reader is offered only what the BFF will serve them.
6. **Recognition rather than recall**
   - a. The roster's order is stated and is the BFF's: tonight's births first, then by due date, soonest first; and the reader can put the births that need cover first.
   - b. A birth that needs cover carries the warning Badge beside its words and says what is missing, so it is found by scanning one column; a covered birth is plain words.
   - c. A Doula on call for two births on one day is flagged on her Doulas-table row, with the births and shared days, and beside her name on those births.
   - d. The rule in force is stated on the roster, a birth with its own rule says so, and the row shows the due date the days come from.
   - e. Each row says, without opening it, the Client, who is on call and for which days, the days on call, and the cover.
7. **Flexibility and efficiency:** T1, T2, T3, T4, T9 and T10 each pass within their budgets (one lettered check each, a–f).
8. **Aesthetic and minimalist design**
   - a. Amber only on the warning Badge of a birth that needs cover and of a Doula holding two births on one day, never a row or cell tint (DESIGN.md, One Meaning Per Hue); no red outside an error.
   - b. Table rows differ in height only by whole lines, where a row holds more Doulas or more that is missing; record cards keep one field order.
   - c. No element carries no information; no fact is said twice in one row; an empty result is said once.
   - d. At 1280×800 and at 375×740 the first birth (Client, who is on call, days, cover) is wholly on the first screen, under the title and the filters.
   - e. On a Practice with one Doula the screen draws no table of one and no flag that cannot fire (judged from a spec).
9. **Error recovery**
   - a. T8 passes.
   - b. A filter with no births says so once and offers the way back to the days the screen opens on.
10. **Help and documentation**
    - a. Every control's effect is clear from its label, including whether the To day is included.
    - b. The screen says which days it shows when no one has chosen any.
    - c. Each of the roster's tables says what it lists; a birth with no days yet says what is missing and where to fix it.
    - d. Each rule field says what it means with a usual answer, and the settings route says a single birth can depart from the rule and where.

## Cross-cutting (each failure costs a point from the heuristic named)

- 320px and 200% zoom: no horizontal page scroll, every control usable, on both routes (4, 7).
- WCAG 2.2 AA: contrast, focus not obscured, target size 24px (4, 7).
- The constraints hold: ADR-0021 GOV.UK patterns, ADR-0022 dates carry the exact day or instant in a `<time>`, ADR-0024 intrinsic layout (no viewport queries), the Voice rules (4, 2).
- Applying or clearing a filter moves neither Apply nor the roster, at 320px and 1440px, in light and dark, beyond the tags' own row at 320px as DESIGN.md records for the Clients list (#2734) (4).

## Changes to this rubric

- 2026-11-14, before the baseline: the draft was read by an Opus agent and by Gemini (which timed out part way). Their findings rewrote it: lettered checks; T5, T6 and T7 tied to a heuristic; "filter" for the screen's controls; T1 on the screen's own 30 days with tonight first; T2 counting a birth with no Doula; T3 as two births on one day; T9 asking for the effect on births on call now; "cannot be reached", the Engagement's and the Overview's words, in place of "away".
- 2026-11-14, after the baseline, before any build: T4 says "arriving by the nav". The draft had "Mine" on for a Doula at the bare address; #2870 decided, for the Schedule and the Clients list together, that the nav's link carries it and the bare address stays the whole Practice. The check contradicted a recorded decision.
