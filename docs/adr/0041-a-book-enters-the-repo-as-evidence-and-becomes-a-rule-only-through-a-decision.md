# A book enters the repo as evidence, and becomes a rule only through a decision

Doula Cloud's founder has a shelf of books he wants the product shaped by: two on running a bootstrapped SaaS, one on jobs to be done, one on interfaces that disappear, one on cognitive bias, and a dozen on forms, components, design systems, type, and accessibility. The first attempt to bring them in, `docs/manifesto.md` on PR [#1270](https://github.com/markgoho/doula-cloud/pull/1270), was closed unmerged on 2026-09-10 and the reset landed as [#1277](https://github.com/markgoho/doula-cloud/pull/1277). Three agents had distilled three books into 34 principles in one day. Where the books disagreed, the agents settled it by message. Where a book met a recorded decision, the agents wrote the ruling. Speed also produced about fifteen factual errors that peer review caught. The rules came before the values they should rest on, and the person who holds those values never decided them.

**A book is evidence. Only a decision of the founder turns evidence into a rule.** The path from shelf to rule has fixed stages, each stage has a form the repo already uses, and no stage before the decision may contain a ruling about Doula Cloud.

## The decision

### Two tracks, because there are two kinds of book

**The values track** is for books that argue what a product should want from a person, how to price, what to measure, how to market, and how a founder spends time. Its stages are:

1. **Read-back.** One page-cited summary per book in `docs/research/books/`, on the template in [`docs/agents/literature.md`](../agents/literature.md). It records what the book argued, including its own exceptions, what has changed since it was published, and where it is weak. It names no Doula Cloud decision as the book's: where the repo has already decided something the book touches, the read-back cites the ADR or ticket that decided it, and the decision belongs to that record.
2. **Synthesis by theme.** One document per theme, not per book and not per conflict, once every read-back on the track is on trunk. A theme is a question the product has to answer, such as what the product asks of a person, how it is priced, or what it measures. The synthesis lays out what each book claims on the theme, where the books agree, where they conflict, and what the repo already decided. It still contains no ruling.
3. **A grilling session per theme.** The founder works through one synthesis at a time with the `grilling` skill. The session ends when his position and its reasons are recorded.
4. **A value ADR per theme.** It records the value, the evidence weighed, page-cited, and what was rejected and why. `docs/manifesto.md` is the index over these ADRs, one line each, and is written only once the first value ADR exists.

**The reference track** is for books that publish patterns: how to build a form, a component, a design system, a type scale. These are not values, and the repo already has a mechanism for them. [ADR-0021](0021-govuk-is-the-reference-for-service-patterns.md) adopted GOV.UK as a reference and [`docs/design/govuk-alignment.md`](../design/govuk-alignment.md) is the table an agent checks before building a screen. Its stages are:

1. **Read-back**, on the same template and under the same rules as the values track.
2. **An adopt-or-not decision per domain**, by the founder, recorded as an ADR in the shape of ADR-0021: what is taken, what is not, and where the new reference sits in an existing order of sources.
3. **A pattern table** an agent checks before building the thing the book is about, linked from the skill or CLAUDE.md section that governs that work.

Which track a book takes is recorded in the book table in `docs/agents/literature.md`. The founder may move a book.

### The no-ruling rule

A read-back or a synthesis states what a book claims. It never states what Doula Cloud should or must do. The reviewer of each one checks for that specifically, word by word, before it merges. A sentence that reads as a ruling is a defect in the document, whatever its content, and the fix is to name the decision it belongs to or to cut it.

### The citation rule

Every claim in a read-back carries its chapter and, where the source has one, its page. A second agent samples ten citations against the source before the read-back merges, and a wrong page fails the review. The template records which page number the file cites, the PDF index or the printed page, and the offset between them once per book.

### The hold

Until a book's read-back is on trunk, nothing in the repo cites it. Until a value ADR or an adopt-or-not ADR exists, no book is the reason for anything in a ticket, a comment, a component, or a copy decision. A read-back on trunk may be cited as evidence in a ticket. A claim becomes a rule only through the ADR that records the founder's decision.

## What was considered instead

**A manifesto first, then evidence.** This is what #1270 did, and why it closed. It put the rules in an agent's hands before the values were the founder's.

**A decision ticket per conflict.** Rejected by the founder on 2026-09-10 as the wrong unit. It only surfaces disagreements, so a book with no conflicts produces a read-back nobody opens, and it makes the founder the referee of dozens of small fights instead of forming values on a theme.

**One track for every book.** Rejected because a forms book and a pricing book are not the same kind of evidence. Silver's patterns want a table an agent checks; Walling's pricing chapter wants a decision the founder holds. Forcing both through a manifesto either dilutes the manifesto or elevates a pattern into a value.

## Consequences

- `docs/research/books/` holds the read-backs. The books themselves are under copyright and never enter the repo; a read-back quotes short phrases only.
- `docs/agents/literature.md` is the living half of this decision: the template, the book table with each book's track and stage, and the mechanics for the agent that writes a read-back. It changes as books move through the stages. This ADR should not change.
- `docs/manifesto.md` does not exist until the first value ADR does, and then holds one line per value ADR and nothing else.
- A theme that no book addresses is not a theme. The synthesis stage produces themes from what the read-backs contain, and the founder may add or merge themes before the grilling.
- This ADR governs books. Standards, primary documentation, and research into a vendor or a law follow the `research` skill as before.
