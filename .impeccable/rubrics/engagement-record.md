# The Engagement record: what a 4 means

One route, `engagements/[engagementId]/`, and the Birth Plan's own page under it (`birth-plan/`), which T1 reaches from it. A heuristic's checks read both, and a failure on either costs the point.

Each heuristic's checks are lettered, one claim to a letter. A heuristic scores 4 only when **every** lettered check passes, each with evidence a grader can point to: a screenshot, a measured value, the accessible text, or a walk's act log. A check with no evidence fails. 3 = one lettered check fails, 2 = two, 1 = three or more, 0 = the heuristic's core job is absent. A job (T1–T10) costs its point under the heuristic that names it: T1–T5 under 7, T6 under 10, T7 under 5, T8 and T9 under 4, T10 under 9.

Graders are three independent agents per round. None sees an earlier score, this branch's commits, or another grader's output. The score per heuristic is the median of the three.

In this rubric the **header** is the back link, the title and what sits beside and under it, above "Jump to"; **at a glance** is T2's eight facts; a **screen** is the viewport's height at the stated size.

Seed: the demo Practice under `dev:full` (Rooted Birth Collective, 14 Doulas; `app/scripts/demo-practice.ts`). Camille Boyd: birth, due in 20 days ("37 weeks"), signed Contract, an open Invoice, Birth Plan (pain relief: "No epidural unless I ask twice.") and Care Plan saved, a logged and a scheduled Visit, a thread whose newest Message is hers (unanswered), Priya then Bethany by Narrowing, and a Coverage gap tonight with nobody covering. Grace Whitfield: Contract sent, not signed, in intake. Tessa Moreau: paid. Imani Brooks: postpartum, her Invoice overdue. Mira Castellanos: a loss recorded six days ago. Imogen Okafor: a Doula and no due date. Tamsin Reyes: due in 25 days, no Doula. Talia Brandt and June Okonkwo: a Draft not sent. Willa Sandoval: voided. Odalys Reyes: voided and replaced by a Draft. Morgan Grier: Lena by Narrowing, covering Marisol's gap. Morgan Grier is also Lena's Engagement with a $450 Doula fee (T7). Ingrid Larsen: postpartum, intake, no Contract yet, with Joss. Completed with a birth outcome: Jamie Bishop and Reese Grier. Camille's history runs ten weeks: started 70 days ago, Contract sent 63 and signed 56 days ago, a deposit paid 55 days ago and a balance Invoice due in 14 days, three past Visits with notes, eight Messages over three weeks, 33 Activity entries. Widths: 1440×900, 1280×800, 768px, 375×740, 320×640; 200% zoom at 1280px.

## The jobs

Each is a hypothesis from the persona records (`docs/personas`, `docs/journeys`, `docs/test-plans`), not research (ADR-0045). The records name the jobs; the budgets are ours. Priya and Lena are always walked signed in as themselves by `persona-walker`, never judged from Renata's session.

- **T1 Priya, phone (375×740), the corridor:** "She did not want an epidural unless she asks twice." From arriving on Camille's record by her Clients list, read her pain-relief answer ("No epidural unless I ask twice.") in ≤ 1 action ("Birth Plan" in "Jump to"), the answer wholly on the screen after that action. The Birth Plan's answers stay on the record at a capped height (8d), the full plan with Print and the PDF 1 action from there; the "Birth Plan" link is on the first screen at 375×740 and at 320×640. (`journeys/employed-doula.md:12`, `:94`, `:169`, `personas/employed-doula.md:13`, `:35`)
- **T2 Renata, desk (1280×800), at a glance:** on arriving on Camille's record, with no action and no scroll, read: care phase; due date with how far along ("37 weeks"); the Doula(s); who is on call tonight, which here says nobody is; the next Visit's day and time; the Contract's state; what is owed and whether any is overdue; whether the thread is unanswered. These pass as 6a grades them, with no action and no scroll (graded once, there). On Imani's: the overdue amount. On Grace's: "Sent", and nothing owed. Renata's: the Doula, who is on call, the Contract and Invoice state (`personas/practice-owner.md:15`, `:19`); Priya's "where she has to be, what this Client wants, and what she was told last time" gives the next Visit and the thread (`personas/employed-doula.md:17`); the weeks are Maya's (`journeys/solo-birth-doula.md:121`).
- **T3 Priya (375×740), the Visit:** "Record what we covered so it is there next time." Read the last logged Visit's notes in ≤ 2 actions and start writing on it in ≤ 3; reach Add a Visit's first field in ≤ 2 (Jump to → Visits, then "Add a Visit", the Visits section's first control, 6b). Saving updates the Visit's notes, or adds the Visit, in place, and the next-Visit fact at a glance follows it. (`journeys/employed-doula.md:111`, `personas/employed-doula.md:21`)
- **T4 Priya (375×740), the Client wrote:** "Confirm Thursday." Read Camille's newest Message and start typing a reply in ≤ 2 actions from arriving; that the thread is unanswered, and since when, is read before the thread is opened. Sending adds the reply to the thread in place, and the unanswered fact at a glance clears without a reload. (`journeys/employed-doula.md:126`, `GLOSSARY.md:75`)
- **T5 Dee (1280×800), the money:** walked signed in as Dee, an Admin. On Camille's record, read what is owed and when it is due in 0 actions (T2), and reach "Record a payment" on the open Invoice in ≤ 2. Saving updates the Invoice's row and the owed fact at a glance in place. On a signed Contract, "Void this Contract" is present as 8f places it, and asks first. (`personas/non-doula-admin.md:21`, `personas/practice-owner.md:11`)
- **T6 Renata (1280×800), how it came to be:** on Camille's record, answer in ≤ 2 actions each who sent the Contract and when, who put Bethany on and when, and who last changed the gap and when. The Activity reads newest first with When, What and Who on every entry; it narrows by kind (All, Care and Visits, Contract and money, Doulas and on call) as the BFF serves it, never by filtering the rows loaded; older entries load in place. (CLAUDE.md, Audit trail; `GLOSSARY.md:81`, ADR-0022)
- **T7 Lena (375×740), a contractor:** on the Engagement she took by Offer, she reads her own fee and terms; on any Engagement she holds no granted attachment on, the address tells her nothing of it; no money the BFF refuses her is on the screen or in its responses; no control is drawn whose act the BFF refuses her. Add a Visit, Request void and Start the Care Plan are hers and do not fail this (ADR-0008's write table). (`personas/contractor-doula.md:22`, `:26`, `:30`, ADR-0008)
- **T8 Screen reader (accessibility snapshot only, 1280px and 375px):** the page's `<h1>` is the Client's name and the header's facts follow it in reading order with their labels; each section is an `<h2>` reached by heading navigation in the drawn order; each at-a-glance link names its fact and section (the Phone fact's `tel:` link names the number and that it calls the Client); each Change names its row; states are words, not color alone.
- **T9 Keyboard only:** every control is reachable in drawn order with a visible focus ring never hidden under anything; on Camille's, the stops from "Skip to main content" to "Birth Plan" in "Jump to" are at most the back link, the header's moves, 1, and Birth Plan's position in "Jump to"; a section's form, opened, takes focus and its Cancel returns it.
- **T10 Failure (the BFF refusing):** with one section's read refused (Invoices, then Visits), that section says so in place in Voice-rule words with "Try again", the others stand, and its at-a-glance fact says it could not be read, never "Nothing owed" or a zero. With the Engagement's own read refused, the page says what happened and offers the way back.

## Heuristic checks

1. **Visibility of system status**
   - a. Under DevTools' "Slow 4G" (ADR-0059's slow connection), loading shows the header, facts and section bars as skeletons within 300ms, and nothing moves more than 4px when each read lands (measured: the first section heading's top and the facts column's top, before and after).
   - b. Under the name, before "Jump to": care phase, due date with weeks, and the Doula(s), or "No Doula yet" (Tamsin) and no due line (Imogen). Today care phase is the summary's third row, below a 10-chip "Jump to".
   - c. On call tonight is stated at a glance as a person's name or that nobody is (Camille), in words.
   - c'. Where nobody is on call tonight, the header's status line says so with the warning Badge: the one loud place, so a reader who never looks at the right-hand column still meets it (ADR-0018:293's right-rail blindness). The at-a-glance fact and the On call section's line say it in plain words.
   - d. Each at-a-glance number is the BFF's (the next Visit from a read that serves the next one, not the first 30 rows loaded; owed from the Invoices read), verified by a grader against the network response.
   - e. No uncaught error in the console on load (today a `TypeError` from `listOrigin.svelte.ts:77`).
2. **Match with the real world**
   - a. The page's text holds no "Engagement" (a case-insensitive find, visible copy and accessible names), and every label is a `GLOSSARY.md` term or its "Client says" word (`journeys/solo-birth-doula.md:20`).
   - b. Weeks read "37 weeks" for Camille, "36 weeks" for Tamsin, "28 weeks" for Grace, and nothing for Imani, Mira and Imogen.
   - c. Activity entries under 7 days read relative, older ones "Aug 14, 2026, 8:12pm", in the Practice's zone (ADR-0022:38, :42, ADR-0036); every entry is a `<time datetime>` carrying the instant.
   - d. The capital on Contract, Client, Doula, Practice, Owner and Admin follows the domain-noun rule.
   - e. A Contract's state uses its own words ("Draft, not sent yet", "Sent, waiting for the Client to sign", "Signed", "Voided, no longer active"), as on the Contracts list.
3. **User control and freedom**
   - a. Back returns to the list the reader came from, with its filter kept, from the Clients list, On call and Contracts.
   - b. Every in-place form has a Cancel that changes nothing (where focus goes is T9's).
   - c. The Birth Plan's page has a way back to the record's Birth Plan section.
   - d. Where the page is taller than 2700px at 1440×900, a "Back to top" link follows each section taller than 900px and the page's end, and moves focus to "Jump to".
   - e. The top bar marks the list the record belongs to as current (`aria-current`), and the back link names that list ("Clients", "On call", "Contracts").
4. **Consistency and standards** (T8 and T9 cost their points here)
   - a. One width rule, measured at 1440×900 on Camille's record: each `DataTable` is at most its own width set to `max-content` plus 32px, measured in DevTools (ADR-0018's 2026-11-13 amendment: tables shrink to their columns); prose (a state line, a paragraph, the thread, a Notice, a form) at most `--measure`; key/value rows (`SummaryList`) span the main column with each value's text at most `--measure`; no block wider than the main column; the main column's rows and tables share one right edge. Today: Visits 1265, Invoices 1215, Notice 1359, Messages 1359.
   - b. At 1440×900 and at 1280×800 the facts column sits beside the main column, its top level with the main column's first content, in the room that is empty today; it stacks only where its container is narrower than at 1280px, by a container query, never a viewport query (ADR-0024); the title stays at the gutter (ADR-0018:293).
   - c. `RecordDetail` takes the facts as a named snippet, and `EngagementScreen`'s `<style>` sets no grid for it, so the Client record (#2754) can take it.
   - d. Summary lists, tables, buttons, Badges and their hues are the climbed lists' components, in `DESIGN.md`'s One Meaning Per Hue Rule (`DESIGN.md:213`): amber where someone must act, red for overdue, neutral for owed and not yet due.
   - e. A convention the surface brief's Category baseline lists is on the screen, or its decline holds for a reader those products have trained; two of three graders naming a missing one, or finding a decline unsound, fails this.
   - f. T8 passes.
   - g. T9 passes.
5. **Error prevention** (T7 costs its point here)
   - a. No act that is not undone (void, end care, take a Doula off) is in the header, and each asks first, with what it does in words.
   - b. Priya is shown no control, link, section or fact whose read or act the BFF refuses her (no "Nothing owed" where she may not read money); Lena's is T7.
   - c. T7 passes.
6. **Recognition rather than recall**
   - a. At a glance, at 1280×800 on Camille's record, every fact T2 names is on the first screen with its label, untruncated. Links: on call tonight to On call, next Visit to Visits, Contract to Contract, owed to Invoices, unanswered to Messages; the header's facts (phase, due, Doula) carry none. A fact pressed onto one line with another to fit does not pass (`grading-round.md`, Contracts' void requests).
   - b. The header holds "Message the Client", 1 action from arriving and opening the compose, and no destructive act; "Add a Visit" is the Visits section's first control, for a reader the BFF lets add one.
   - c. Each section that is not empty says its state in its first line (Birth Plan: how many answered, the day it last changed, and whether the Client has read it, from `clientAcknowledgedAt`; Contract: its state; Invoices: owed).
   - d. A section is empty when the Engagement read says it holds nothing (no Contract, no Visit, no Message, no Invoice, no Offer, no Care Plan or Birth Plan started, Birth outcome before the due date, On call before its on-call days). No empty section stands above a filled one, except On call during its on-call days and Birth outcome from the due date; Activity is the last section (in one column the record's Details close the page after it, as the facts column's lower half). Checked on Camille (on call), Grace (intake), Imani (postpartum) and Jamie Bishop (completed).
7. **Flexibility and efficiency:** T1, T2, T3, T4 and T5 each pass within their budgets (one lettered check each, a–e).
8. **Aesthetic and minimalist design**
   - a. At 1280×800 on Camille's record the first section's heading, its state line (6c) and its first row or summary item are wholly on the first screen (bottom ≤ 800px). Today: measured at the baseline (at 1440×900, On call starts at 1100px).
   - b. At 375×740 the first screen holds the header's facts (1b), the on-call-tonight fact and the "Birth Plan" link. Today it holds the back link, the name, one link, ten chips and one summary label.
   - c. Camille's page at 1440×900 is at most 5400px (6 screens; baseline 6846px, round 1 6605px), with every section still on it; no section is hidden behind a disclosure that its job opens every time.
   - d. The Birth Plan's answers are on the record at a capped height (≤ 1 screen at 375×740; today 1141px at 1440), the full plan 1 action away; the Care Plan is behind its own page or a capped view (today 644px).
   - e. An empty section is its heading and one line, its add control on that line or the next (measured: at most heading plus 2 line boxes).
   - f. "Void this Contract" is a secondary Button in the Contract section's last subsection, after its WarningText; red fill is on the confirm step only. Today it is a red fill on the signed Contract (`ContractStatus.svelte:448`).
   - g. No fact is repeated between the header (1b) and the at-a-glance facts (6a), except that nobody on call tonight is in both, loud only in the header (1c'); a section's state line (6c) may restate its own section's fact. "Jump to" is not wider than its links need, and no box carries no information.
   - h. At 375×740 the first screen holds ≤ 3 decisions, counted as PR-G10 counts them (a list of links is one, a group of summary rows is one; `journeys/employed-doula.md:169`).
   - i. "Jump to" wraps to at most 2 lines at 1280px and 4 at 375px, and every link in it is a section on the page.
9. **Error recovery**
   - a. T10 passes.
   - b. A refused save in a section keeps what was typed and says on its field what to do.
   - c. A refused "Load more" keeps the rows shown and says so beside the button.
10. **Help and documentation** (T6 costs its point here)
    - a. T6 passes.
    - b. Each empty section says what it waits for and where to start it (Invoices before a Contract, Birth Plan before one is started); a postpartum or after-outcome record says why there is no Birth Plan.
    - c. The void warning says in words what voiding does, before the confirm.
    - d. Any term on the page a walker asks the meaning of fails this check.

## Cross-cutting (each failure costs a point from the heuristic named)

- 320px and 200% zoom: no horizontal page scroll, every control usable (4, 7).
- WCAG 2.2 AA: contrast, focus not obscured, target size 24px (4, 7).
- The constraints hold: ADR-0021 GOV.UK patterns, ADR-0022 dates in a `<time>`, ADR-0024 intrinsic layout (no viewport queries), ADR-0036 the Practice's zone, the Voice rules (4, 2).
- Every state renders whole: Camille, Grace, Imani, Mira (after a loss: no Birth Plan nag, no on-call line), Imogen, Tamsin, Talia (Draft), Willa and Odalys (voided, and replaced), Morgan Grier, the completed Engagement; a grader opens each at 1280×800 and 375×740 (4).

## Changes to this rubric

- 2026-11-15, before the baseline: drafted from the orchestrator's jobs and measurements and the category baseline. Weeks settled before drafting (the surface brief's differs list).
- 2026-11-15, after the two readings: T1 targets Camille's pain-relief answer in ≤ 1 action, and the Birth Plan stays on the record at a capped height (8d): Priya's job is one answer in a corridor (`journeys/employed-doula.md:94`).
- 2026-11-15, after the two readings: 8h counts the phone's first-screen decisions as PR-G10 does, so the header and the facts cannot add moves past its budget of 3.
- 2026-11-15, after the two readings: 8f says where "Void this Contract" goes and how it looks, in place of "never the loudest"; T5 points at it.
- 2026-11-15, after the two readings: 1c' says nobody on call tonight in the header's status line as well.
- 2026-11-15, after the two readings: 4b holds the facts column beside the main column at 1280×800 as well as 1440×900; letting it stack at the desk's width left the horizontal-space complaint unfixed.
- 2026-11-15, after the two readings: 3d sets one threshold (2700px at 1440×900, after each section taller than 900px); 8c stays at 4 screens.
- 2026-11-15, after the two readings: T2's facts cite the personas' own lines.
- 2026-11-15, after the two readings: 3e and 8i check wayfinding by the back link and the top bar; a breadcrumb is declined in the brief.
- 2026-11-15, after the two readings: 6d is a rule a grader can apply before the design call writes the per-phase orders, which then replace it.
- 2026-11-15, after the two readings: T6 asks for an Activity narrowed by kind and served by the BFF, which the brief now builds; "no job on record" was contradicted by T6.
- 2026-11-15, after the two readings: #2958, #2945 and #2949 are filed and out of this climb, so T5, T6, T7, T9, 3a, 5b and 10c no longer depend on them.
- 2026-11-15, after the two readings: 8a needs the first section's state line and first row, not its heading alone; T3, T4 and T5 end at the save updating the section and the fact at a glance in place; 8g scopes "said twice" to the header and the facts; 6b names the header's two moves.
- 2026-11-15, after the two readings (Opus O5–O9, O14, O18, O21): 2c follows ADR-0022's relative times under seven days; 6c drops "whether the Client has read it", which no BFF field records; "at a glance" is T2's eight facts and 6a names each link's section; T9's stops are counted from the skip link; 4a measures a table against its own `max-content`; 4c names the snippet; 5b is Priya's only and T2 is graded once under 6a; 3b leaves focus to T9; T3 and T4 cite the Thinking lines; T5 is Dee's walk; 1a runs on Slow 4G and measures the facts column; 2a and 10d are a find for "Engagement", GLOSSARY words, and any term a walker asks about.
- 2026-11-15, after the two readings (Opus O4, O15): 4d holds the hues to `DESIGN.md:213` (overdue is red, owed and not yet due neutral); 3e reads `aria-current`; 8i caps "Jump to" at 2 lines at 1280px and 4 at 375px.
- 2026-11-15, orchestrator: 6c keeps whether the Client has read the Birth Plan; the BFF serves it as `clientAcknowledgedAt` (`EngagementScreen.svelte:1311`), so O6 was wrong on that point.
- 2026-11-15, after round 1 (before its build): 8c's cap is 5400px, not 3600. The 3600 was the orchestrator's guess ("about 4 screens") with no job record behind it; the round-2 design call showed On call, Birth Plan and Visits alone need about 1800px to show their state lines and the first rows their jobs read (the gap, T1's answer, T3's last notes), so 3600 could pass only by hiding what a job opens every time, which 8c's own second clause fails. The number is fixed before the build and may fail.
- 2026-11-15, after round 1: 1c' keeps the header's warning as the one loud place (it answers ADR-0018:293's right-rail blindness); 1c and the On call line say it in plain words, and 8g exempts that one alert (graders A and C found the fact said three times, and 8g and 1c' in conflict).
- 2026-11-15, after round 1: 6b holds one header move, "Message the Client"; "Add a Visit" moves into Visits, where Priya's walk reached for it, so the phone's first screen holds three decisions (8h; all three graders counted four). T3's budget is unchanged (2).
- 2026-11-15, after round 1: 6d defines "empty" from the Engagement read and keeps Activity last; the per-phase table it waited for was never written into the brief (2/3 graders failed 6d on Details and Activity under empty sections).
- 2026-11-15, after round 2: 4a lets key/value rows span the main column, values still at the measure, and asks for one right edge across rows and tables. Capping every SummaryList at the measure left a 300–400px empty band between the content and the facts column at 1280 and 1440 (3/3 graders, C4), which is the founder's "not making efficient use of the horizontal space"; the reading measure the cap protected is the value's text, which stays capped.
- 2026-11-15, after round 2: T8 names how the Phone fact's link reads, since the round-3 design call adds the Client's phone as a sixth fact (baseline row 27; 3/3 graders found it only at the page's end on a phone).
