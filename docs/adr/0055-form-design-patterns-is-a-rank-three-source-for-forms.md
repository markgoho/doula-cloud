# Form Design Patterns is a rank-three source for forms, and the other form books stay evidence

DoulaCloud asks people a lot of questions. [ADR-0021](0021-govuk-is-the-reference-for-service-patterns.md) made GOV.UK the default reference for every screen that asks for something or reports a failure, and its 2026-09-01 amendment set the order of sources to walk where GOV.UK has no pattern. [`docs/design/govuk-alignment.md`](../design/govuk-alignment.md) holds the pattern-by-pattern decisions. Five books on the reference track also have a position on forms, and under [ADR-0041](0041-a-book-enters-the-repo-as-evidence-and-becomes-a-rule-only-through-a-decision.md) none of them could be the reason for anything until the founder decided which, if any, to adopt.

On 2026-10-03 the founder worked through the theme synthesis at [`docs/research/books/themes/forms.md`](../research/books/themes/forms.md) in the forms grilling session, [#1719](https://github.com/markgoho/doula-cloud/issues/1719): one round of six questions, each with a recommended answer, every one taken as written ("agree"), and a positions record confirmed the same day ("confirmed"). This ADR records those decisions. Each reason below is the founder's, as the recommendation the founder accepted on #1719; a book appears only as evidence weighed.

**One book enters the order of sources, at rank 3 and for forms only. Two are declined as sources and stay as evidence. GOV.UK's rule that a question opens with nothing chosen becomes the general rule, and three standing decisions hold.**

## The decision

### *Form Design Patterns* is adopted at rank 3, for forms only

*Form Design Patterns* (Adam Silver, 2018; [read-back](../research/books/form-design-patterns.md)) enters ADR-0021's order of sources at rank 3, tied with *Inclusive Components* and Adrian Roselli's writing, for a screen that asks a person a question, checks the answer, or reports a refusal. As with GOV.UK, the decision is taken and never the markup. Where it and NHS.UK or ONS disagree, rank 2 wins.

The founder's reason (#1719, P1): GOV.UK's gaps on forms are real, the question protocol and repeated items among them, and this source fills them in GOV.UK's own direction, so a rank-3 answer rarely fights a rank-1 one. It is one author's book on evidence without a control, which keeps it below the design systems that are maintained and tested with users.

Weighed for: the error summary that takes focus (Ch. 1, pp. 51–58); one thing per page (Ch. 2, pp. 70–73); "(optional)" and no asterisk (Ch. 2, pp. 75–79); three boxes for a memorable date (Ch. 3, pp. 154–160); check your answers (Ch. 2, pp. 107–109); no disabled submit button (Ch. 1, pp. 62–63); the words an error avoids (Ch. 1, pp. 63–66); and, where GOV.UK is silent, the question protocol (Ch. 1, pp. 27–28), a field whose width tells the length of the answer (Ch. 2, pp. 79–82), and focus to the heading after a repeated item is removed (Ch. 9, pp. 367–369). The author's own limit, rules broken only "occasionally" (Introduction, p. x) and a departure from convention only on "thorough and diverse" research (Ch. 1, pp. 29–30), matches ADR-0021's burden on departing.

Weighed against: the headline figure for one thing per page, Just Eat's 5%, comes from a whole-checkout redesign with no control (Ch. 2, p. 69); the one study named on live checking is argued against on reasoning (Ch. 1, p. 61; read-back section 7); and GOV.UK has since moved the hint and the message out of the `<label>`, so parts of the markup are out of date (read-back section 7). The last is why only the decision is taken.

Rejected: rank 2, tied with NHS.UK and ONS, because that rank is for maintained, user-tested systems; and evidence only, because GOV.UK's gaps on forms would then fall to original work while a source in GOV.UK's direction sat unused.

### *Inclusive Design Patterns* is declined as a source and stays evidence

*Inclusive Design Patterns* (Heydon Pickering, 2016; [read-back](../research/books/inclusive-design-patterns.md)) is not a source. The founder's reason (#1719, P2): its form pattern loses to GOV.UK on every point it disputes, and its own read-back records that it was never tested past three fields, which is not the intake form; what remains agrees with GOV.UK and the brief's Voice section, so a rank would add either a second voice saying the same thing or one saying the opposite.

Weighed for, and already carried by GOV.UK: a `<label for>` on every control (A Registration Form, pp. 267–268); a fieldset only where it helps (pp. 272–273); radios over `<select>` (A Blog Post, p. 86); HTML before ARIA (A Filter Widget, pp. 227–232); a label that describes purpose over brand voice (pp. 287–289).

Weighed against and rejected: one general message in a live region above the submit button, no list of errors, and focus left on the button (pp. 278–282); an asterisk with `aria-required` for a required field (pp. 274–276); login and registration as a two-button toolbar on one page (pp. 262–266); and the three-field limit of the pattern (read-back section 8; pp. 274, 280).

Also rejected: rank 3 beside *Inclusive Components*, because the same author's later book already holds that rank; and the filter-widget chapter alone, because it is a component question for the inclusive components theme ([#1720](https://github.com/markgoho/doula-cloud/issues/1720)).

### *Designing User Interfaces* is declined as a source and stays evidence

*Designing User Interfaces* (Michal and Diana Malewicz, 2020; [read-back](../research/books/designing-user-interfaces.md)) is not a source. The founder's reason (#1719, P3): every point on which it agrees, GOV.UK already gives, and every point on which it disagrees, a standing record has already refused. The look of a form belongs to the visual craft and type theme ([#1722](https://github.com/markgoho/doula-cloud/issues/1722)).

Weighed for, and already carried by GOV.UK: the label above the field (Forms, p. 241); radios for five options or fewer (pp. 245–247); the optional field marked (p. 256); a reveal in place of a confirm field (p. 234); every form tested with real users (p. 230).

Weighed against and rejected: every message under its field and no summary (p. 233); a check mark on a field before submit (p. 233); a placeholder reading "Please enter your e-mail…" (p. 256); progress always shown as a percentage, a count or a slider (p. 259); animation and an animated character to make a form memorable (p. 259); the cursor moved past a hyphen for the person (p. 244); and "many studies" named for the inner shadow with none cited (p. 238; read-back section 8).

Also rejected: rank 4 or 5, for the look of a form only, because the visual craft theme weighs the look with *Refactoring UI* beside it.

### The other books on the track

*Inclusive Components* already holds rank 3 by ADR-0021's amendment, and its standing under ADR-0041 is the inclusive components theme's decision (#1720). For forms, every claim its to-do list chapter makes, a `<form>` so Enter submits, a visible label and no placeholder in its place, and no disabled submit button (A Todo List, pp. 44–49), agrees with GOV.UK, so this ADR needs nothing from it. *Refactoring UI*'s form position, spacing and the drawn well of an input (Layout and Spacing, pp. 83–86; Creating Depth, pp. 155–156), is the visual craft theme's (#1722). The other seven books on the track have no position on this theme, as the synthesis records.

### A question opens with nothing chosen

GOV.UK's rule, adopted by ADR-0021, is the general one: a question opens with no answer chosen, and a chosen answer is a departure that records its reason, as *Who is this Visit for?* ([#909](https://github.com/markgoho/doula-cloud/issues/909)), the Practice timezone ([#1166](https://github.com/markgoho/doula-cloud/issues/1166)) and *Who is the Doula?* ([#1515](https://github.com/markgoho/doula-cloud/issues/1515)) already do. The brief's Hick's Law row, which asks for "a default chosen for the person wherever a sensible default exists", is reworded to match.

The founder's reason (#1719, P4): a doula answering a question about a Client is not choosing a personal setting, so a chosen answer is a fact about someone else that may go through unread, which is the harm GOV.UK names and the brief's own Cognitive Bias row restates ("a default is never neutral"). The three departures show the exception works. Two opposite rules make the next builder choose between documents, the fault ADR-0021 exists to stop.

Weighed against and rejected: the most common option chosen so the person never meets an error (*Form Design Patterns*, Ch. 2, p. 86; Ch. 3, p. 192); the most popular radio option chosen (*Designing User Interfaces*, Forms, p. 252). Also rejected: leaving both rules standing.

### A field's message stays until the next submit

After a refused submit, the error message on a field stays until the person submits again, as GOV.UK and the app already do. The founder's reason (#1719, P5): a message that clears during typing while the summary still lists it is two parts of one page saying different things, and a recheck on each keystroke needs a live region or it is silent to a screen reader; *Inclusive Design Patterns* itself debounces the recheck so a screen reader is not put into "80's remix mode" (A Registration Form, p. 286) and leaves a live region per field "up to you" (p. 287). The usability sessions on [#1685](https://github.com/markgoho/doula-cloud/issues/1685) may reopen this if a person is stuck on a message already fixed.

Weighed against and rejected: a recheck of the refused field on every keystroke once a submit has failed (*Inclusive Design Patterns*, pp. 285–287); a check mark before submit (*Designing User Interfaces*, Forms, p. 233); and Wroblewski's finding of forms 42% faster with live checking (*Form Design Patterns* read-back, section 7), which is about checking before a submit and not after one. Either of the first two would be a departure from GOV.UK.

### `StepRail` and the task list hold

`StepRail` stays in `QuestionPage` and `CheckAnswers`, and the task list stays not adopted. The founder's reasons (#1719, P6): the brief chose a visible step count on purpose (Goal-Gradient, Parkinson's Law), shown and never animated; the Carer's Allowance team's 12-step bar removed with no effect (*Form Design Patterns*, Ch. 2, pp. 113–117) is one service's no-effect result, not harm, and the sessions on #1685 are the research it asks for, so the rail moves if they show it does nothing or gets in the way. No form in the app takes days, since intake saves at once and continues ([ADR-0017](0017-twelve-columns-a-practice-defined-layer-and-an-engagement-that-is-asked-for.md)), so the reason for a task list (Ch. 10, pp. 373–380) does not arise. Rejected: removing the rail before that research, and adopting the task list.

### Where the other conflicts rest

The synthesis names fourteen conflicts. Four are decided above (when the check runs, choosing an answer, steps and progress, and the evidence each book rests on). The other ten hold on records that predate this ADR, as the positions record on #1719 sets out: where a field's message goes, what a refused submit says, and where focus goes (`govuk-alignment.md`'s *Error message*, *Recover from validation errors* and *Error summary* rows; ADR-0018); how a required field is marked (*Question pages*); what a placeholder may carry (GOV.UK's Text input guidance); the words of an error (ADR-0021's content rules and `formErrors.usage.spec.ts`); login and registration (separate doors, [#1645](https://github.com/markgoho/doula-cloud/issues/1645)); revealing the password (GOV.UK's Password input component); motion (the brief's Motion rule); and moving the cursor for the person (`DateFields` and GOV.UK's Date input). Undo in place of a confirmation holds on [#473](https://github.com/markgoho/doula-cloud/issues/473)'s block-over-warn, which already weighed the argument (*Form Design Patterns*, Ch. 5, pp. 255–259).

## What was considered instead

**Adopt nothing.** A legitimate outcome under #1684, and the third option on each of #1719's book questions. Rejected for *Form Design Patterns* only, for the reason under P1; taken for the other two.

**One decision for all five books.** Rejected because the five do not stand in the same place: one agrees with GOV.UK and fills its gaps, two dispute it, and two are other themes' to decide.

## Consequences

- **The pattern table is `govuk-alignment.md`.** ADR-0041's third reference-track stage asks for a table an agent checks before building; for forms that table already exists. A forms row that GOV.UK has no pattern for, and that *Form Design Patterns* answers, is recorded there as **Answered -- Form Design Patterns**, the outcome ADR-0021's amendment already defines. No separate table is made.
- **Changes, each carried by its own ticket:**
  - ADR-0021's order of sources and the alignment table name *Form Design Patterns* at rank 3 for forms: [#1752](https://github.com/markgoho/doula-cloud/issues/1752).
  - The brief's Hick's Law row and its Hick and Tesler conflict entry are reworded to GOV.UK's rule: [#1753](https://github.com/markgoho/doula-cloud/issues/1753).
  - The `TextInput` and `Textarea` atoms stop offering a placeholder, which the style guide shows as a hint against GOV.UK's Text input guidance: [#1751](https://github.com/markgoho/doula-cloud/issues/1751), filed during #1719 since it follows from ADR-0021 already.
- Nothing else in the product changes. Every other position holds a record that was already in force.
- `docs/manifesto.md` is unchanged, since it indexes value ADRs only, and the book table in [`docs/agents/literature.md`](../agents/literature.md) moves to **decided** in #1684's sweep after all five reference-track ADRs land.
