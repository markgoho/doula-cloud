# Literature: how a book gets from the founder's shelf into a rule

The decision is [ADR-0041](../adr/0041-a-book-enters-the-repo-as-evidence-and-becomes-a-rule-only-through-a-decision.md). This document is its living half: the book table, the read-back template, and the mechanics for the agent that writes one. It changes every time a book moves a stage. The ADR should not change.

The one-line version: **a book is evidence, and only the founder's recorded decision turns evidence into a rule.** Nothing before that decision may say what Doula Cloud should or must do.

## The books

The source files live in the founder's Google Drive, folder `eBooks`; some sit in a subfolder named for the book. Every book below has a PDF there. The PDFs are copied flat to `~/ebooks/doula-cloud/` on the machine that runs the agents, by the file name in the table, and never enter the repo. The copy needs a gcloud login with Drive scope (`gcloud auth login --enable-gdrive-access`) and then the Drive API's `alt=media` download by file id.

Stage is one of: **shelf** (no read-back yet), **read-back** (on trunk in `docs/research/books/`), **synthesized** (values track: its claims sit in at least one theme synthesis), **decided** (a value ADR or an adopt-or-not ADR cites it).

### Values track

| Book | Author, year | Drive file | Stage |
|---|---|---|---|
| The SaaS Playbook | Rob Walling, 2023 | `The-SaaS-Playbook-ebook.pdf` | read-back |
| Start Marketing the Day You Start Coding | Rob Walling, 2023 ed. | `Start_Marketing_the_Day_You_Start_Coding-eBook.pdf` | shelf |
| Make: The Bootstrapper's Handbook | Pieter Levels | `make-bootstrapper-handbook.pdf` | shelf |
| Jobs To Be Done | see the read-back for the edition | `JTBD-Book.pdf` | shelf |
| The Almanack of Naval Ravikant | Eric Jorgenson, 2020 | `Eric-Jorgenson_The-Almanack-of-Naval-Ravikant_Final.pdf` | shelf |
| The Best Interface Is No Interface | Golden Krishna, 2015 | `no-interface.pdf` | shelf |
| Design for Cognitive Bias | David Dylan Thomas, 2020 | `design-for-cognitive-bias.pdf` | shelf |
| Landing Page Hot Tips | Rob Hope | `Landing-Page-Hot-Tips-Ebook-v1-1.pdf` | shelf |
| Copyhackers: Uplift | Copyhackers | `Copyhackers_Uplift_eBook_USLetter-V03.pdf` | shelf |

### Reference track

| Book | Author, year | Drive file | Stage |
|---|---|---|---|
| Form Design Patterns | Adam Silver, 2018 | `form-design-patterns.pdf` | shelf |
| Inclusive Design Patterns | Heydon Pickering, 2016 | `inclusive-design-patterns.pdf` | shelf |
| Inclusive Components | Heydon Pickering, 2019 | `Inclusive_Components_-_Heydon_Pickering.pdf` | shelf |
| Design Systems | Alla Kholmatova, 2017 | `design-systems.pdf` | shelf |
| Expressive Design Systems | Yesenia Perez-Cruz, 2019 | `expressive-design-systems.pdf` | shelf |
| Laying the Foundations | Andrew Couldwell, 2019 | `laying-the-foundations-pdf.pdf` | shelf |
| Atomic Design | Brad Frost, 2016 | `atomic-design.pdf` | shelf |
| Refactoring UI | Adam Wathan and Steve Schoger, 2018 | `Refactoring UI v1.0.2.pdf` | shelf |
| Flexible Typesetting | Tim Brown, 2018 | `flexible-typesetting.pdf` | shelf |
| Giving a Damn About Accessibility | Sheri Byrne-Haber, 2021 | `Giving-a-damn-about-accessibility.pdf` | shelf |
| Going Offline | Jeremy Keith, 2018 | `going-offline.pdf` | shelf |
| Designing User Interfaces | Michal Malewicz and Diana Malewicz, 2020 | `DESIGNING_USER_INTERFACES_Eng_1.pdf` | shelf |

Books in the folder that are on neither track, by the founder's decision of 2026-09-10: the engineering-craft titles (Accelerate, Clean Code, 97 Things, Infrastructure as Code, the database and Angular books) and the unrelated ones. They may join later; a book joins by a row in a table above.

## The read-back template

A read-back is one file, `docs/research/books/<slug>.md`, written in American English with one unbroken line per paragraph. It has these sections, in this order, and no others.

1. **Source line.** Title, author, publisher, year, ISBN where the book has one, and the date the agent read it. Then the page rule for this file: whether page numbers are the printed page or the PDF index, and the offset between them, found once by opening a page that carries both.
2. **What this file is for.** Two sentences: this is the evidence, it records no Doula Cloud decision. Where the book touches something the repo already decided, the ADR or ticket is named and the decision belongs to that record.
3. **The claim.** The book's thesis in the author's own frame, and what the author says the claim is not.
4. **The argument, chapter by chapter.** What each chapter claims, what evidence it offers, and the examples it rests on. Every claim carries its chapter and, where the source has one, its page. Quotations are short phrases only; the book is under copyright.
5. **The book's own exceptions.** Where the author limits the claim, admits a counter-case, or says a rule does not apply. These are the parts a later reader most needs and the parts a summary most often drops.
6. **Where the book touches this repo.** Each place a claim meets a recorded decision, a ticket, or a screen that exists. The format is: the claim and its page, then the record that decided the matter, then nothing else. No ruling. No "so the product should".
7. **What has changed since publication.** For each example, tool, or figure the book leans on: is it still true, and what is the later evidence. A book applied without this check was one of the named failures of the first attempt.
8. **Where the book is weakest.** Correlation presented as cause, examples that no longer exist, a claim the author's own evidence does not reach.

The test for every sentence in sections 3 through 8 is: does it report what the book says, or what the repo decided? A sentence that does neither is cut.

## Mechanics for the read-back agent

- **One agent per book**, in its own worktree, on the subagent model already configured. The brief forbids spawning further agents; a killed parent leaves orphans.
- **Spawns are staggered** about twenty seconds apart; three worktree agents launched at once fail in the provisioning hook.
- **The agent reads the text extraction, not the PDF.** Beside each PDF in `~/ebooks/doula-cloud/` sits `<name>.txt`, made with `pdftotext -layout`, in which one form feed ends one PDF page, so PDF page N is the Nth form-feed-separated block. The agent reads it front to back. It opens the PDF itself, with the `Read` tool and its `pages` parameter, only for a figure or a table the text lost. It does not skim from the table of contents and it does not work from memory of the book. A claim it cannot find on a page is not in the read-back.
- **The extraction is made once per book**, by whoever starts its read-back, with `pdftotext -layout <name>.pdf <name>.txt` in that folder. It never enters the repo.
- **The pilot goes first.** One book's read-back lands and is read by the founder before the next is started, so the template is corrected on one file rather than twenty.
- **Order after the pilot** is by dependency: a book that touches an open ticket goes before one that does not, and the values track goes before the reference track because its syntheses gate the grilling sessions.
- **The read-back is landed by PR** through the worktree flow, one book per PR, `Closes #N` in the body where a ticket asked for it.
- **The book table above is updated in the same PR**, moving the row to **read-back**.

## The review before merge

A second agent, not the author, reviews every read-back and every theme synthesis before it merges. Two checks, both recorded as a PR comment:

1. **No ruling.** Every sentence in sections 3 through 8 is read for a statement of what Doula Cloud should, must, or ought to do. One such sentence fails the review. The fix is to name the record that decided the matter, or to cut the sentence.
2. **Citations.** Ten claims are chosen across the file, weighted toward section 6, and each is opened at the cited page in the PDF. A claim the page does not support, or a wrong page, fails the review. The reviewer applies the file's own page rule from the source line.

A synthesis has a third check: **every book on the track is present.** A theme synthesis that omits a book's position on the theme, where the book has one, is incomplete.
