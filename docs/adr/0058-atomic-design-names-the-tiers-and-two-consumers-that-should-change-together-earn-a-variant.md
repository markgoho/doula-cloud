# Atomic Design names the tiers, the other design-system books stay evidence, and two consumers that should change together earn a variant

DoulaCloud already has a design system. [ADR-0018](0018-templates-are-a-design-system-layer-with-two-named-exits.md) put Templates beside atoms, molecules and organisms in `app/src/lib/components/`, and kept the Atomic Design names because "Atomic Design is one of two things the map declared it would not change". [ADR-0019](0019-pen-dev-is-the-working-surface-and-code-is-the-truth.md) makes the code the truth, `tokens.css` holds every value, and the gates hold both. ADR-0018 took a book's names as a fixed point before [ADR-0041](0041-a-book-enters-the-repo-as-evidence-and-becomes-a-rule-only-through-a-decision.md), under which a book is evidence and only the founder's recorded decision turns it into a rule. So the tier names stood on a record that no adopt-or-not decision had confirmed, and six more books on the reference track had a position on how a system is organized and kept, with no decision at all. ADR-0018 also says its bar between its two exits "is expected to be revised".

On 2026-10-24 the founder worked through the theme synthesis at [`docs/research/books/themes/the-design-system.md`](../research/books/themes/the-design-system.md) in the design-system grilling session, [#1721](https://github.com/markgoho/doula-cloud/issues/1721): one round of nine questions, each with a recommended answer. The founder took every recommendation ("Honestly I'm Fine to just go with your recommendation"), and confirmed the positions record of nine positions the same day ("Sure, confirmed"). This ADR records those decisions. Each reason below is the recommendation the founder accepted as his own on #1721; a book appears only as evidence weighed, and where a position takes a single claim, the claim and its page are named, not the book.

**One book is adopted for the tier names and the template–page split, outside the order of sources. Five are kept as evidence, with no rank. Four tiers and ADR-0018's definition of a Template hold. The bar between ADR-0018's two exits is restated, a naming rule is written as the code already follows it, and one interface inventory is made before launch. The style guide stays local-only, and no Practice brand enters the system before launch. Nothing in the product changes. ADR-0018 and `docs/design/workflow.md` change, and the inventory is filed, each by its own ticket.**

## The decision

### *Atomic Design* is adopted for the tier names and the template–page split only, outside the order of sources

*Atomic Design* (Brad Frost, 2016; [read-back](../research/books/atomic-design.md)) is adopted for two things: the names of the tiers, atoms, molecules, organisms and Templates, and the split between a Template and a page, a page being a Template with real content in it. Nothing else in the book is adopted. It takes no place in [ADR-0021](0021-govuk-is-the-reference-for-service-patterns.md)'s order of sources (GOV.UK, then NHS.UK and ONS, then *Inclusive Components* and Adrian Roselli's writing, then Every Layout, then original work with a recorded reason). ADR-0041 asks an adopt-or-not decision where a new reference sits in an existing order of sources; for this book the answer is nowhere, on purpose. This decision gives ADR-0018's fixed point the decision under ADR-0041 that it did not have.

The founder's reason (#1721, Q1 and position 1): ADR-0018 already depends on those names and needs a decision under ADR-0041 beneath them. The order of sources ranks what a screen does, and this book answers what the library's tiers are called, which no rank answers. The book gives no test for which tier a component belongs to; the repo's test is #424's rule, as ADR-0018 states it: a molecule is a part of a section, an organism a whole one.

Weighed for: an interface is a system of components (ch. 1, p. 10); atoms, molecules, organisms, templates and pages held at once (ch. 2, p. 42); a page as a template with real content that tests the system (ch. 2, pp. 52–55); and the names "not rigid dogma" (ch. 2, pp. 59–60).

Weighed against: no test for which tier a component belongs to, Pattern Lab archived in May 2026, one dashboard as the only stateful screen, and no tokens (read-back sections 7 and 8).

Rejected: adopting it whole, because its process is for a team and its tool is gone; and declining it, because ADR-0018 would then keep a book's names with no decision under them.

### *Design Systems*, *Expressive Design Systems*, *Laying the Foundations*, *Designing User Interfaces* and *Refactoring UI* stay evidence, with no rank

*Design Systems* (Alla Kholmatova, 2017; [read-back](../research/books/design-systems.md)), *Expressive Design Systems* (Yesenia Perez-Cruz, 2019; [read-back](../research/books/expressive-design-systems.md)), *Laying the Foundations* (Andrew Couldwell, 2019; [read-back](../research/books/laying-the-foundations.md)), *Designing User Interfaces* (Michal and Diana Malewicz, 2020; [read-back](../research/books/designing-user-interfaces.md)) and *Refactoring UI* (Adam Wathan and Steve Schoger, 2018; [read-back](../research/books/refactoring-ui.md)) are not sources. Each stays evidence. Where a position below takes a single claim from one of them, this ADR names the claim and its page as the reason weighed, not the book as a source; position 6 takes one, Kholmatova's variant test.

The founder's reason (#1721, Q2 and position 2): none answers what a screen does when GOV.UK has no pattern, which is what a rank is for; and each writes for a team.

Weighed for: a system is more than its library (Kholmatova ch. 1, pp. 35–36; Couldwell Chapter 9, p. 181; Malewiczes Design Systems, p. 361; Perez-Cruz Ch. 5, p. 114); and "Define systems in advance" (*Refactoring UI*, Starting from Scratch, pp. 25–28).

Weighed against: each rests on its author's own employer or one invented product, and no book measures a team with a system against one without (each read-back's section 8); and the team practices assume a team (Kholmatova read-back section 8; Malewiczes p. 372; Couldwell Chapter 11, pp. 254–257).

Rejected: a rank for how the system grows, because the repo's own ADRs already hold that. This position covers only what *Refactoring UI* and *Designing User Interfaces* say about a system. Their values, the type scale, the spacing steps and the palette, are the visual craft and type theme's, on [#1722](https://github.com/markgoho/doula-cloud/issues/1722). [ADR-0055](0055-form-design-patterns-is-a-rank-three-source-for-forms.md) already declined *Designing User Interfaces* as a source for forms.

### The other books on the track

*Form Design Patterns* is a rank-3 source for forms by ADR-0055; *Inclusive Components* is at rank 3 by [ADR-0057](0057-inclusive-components-stays-a-rank-three-source-and-a-control-that-acts-at-once-is-a-switch.md); *Inclusive Design Patterns* is declined as a source by ADR-0055 and ADR-0057, and *Giving a Damn About Accessibility* by ADR-0057; and *Flexible Typesetting* is the visual craft and type theme's (#1722). Their narrow positions on this theme, that a fix made in the system reaches every instance and that a component works wherever it is placed, the repo already holds through [ADR-0024](0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md) and the component-first rule in [`docs/design/workflow.md`](../design/workflow.md). Silver's checkout page (*Form Design Patterns*, Ch. 2, p. 113) is evidence under position 5 below. *Going Offline* has no position on this theme, as the synthesis records. This ADR adds no rank for any of them.

### Four tiers hold

The library keeps four tiers, atoms, molecules, organisms and Templates, as ADR-0018 has them and as `style-guide.spec.ts` reads them off disk. On 2026-10-24 they hold 17 atoms, 28 molecules, 22 organisms and 10 Templates.

The founder's reason (#1721, Q3 and position 3): FutureLearn's cost was the argument over each component, and #424's test (a molecule is a part of a section, an organism a whole one) ends it. The split also tells a reader of `/style-guide` whether a thing fills a section or sits in one.

Weighed for: Frost's five stages (ch. 2, p. 42); and Perez-Cruz's four layers (Ch. 3, pp. 51–59). Weighed against: Kholmatova's FutureLearn merged molecules and organisms, since more than two levels "can get confusing" (ch. 10, p. 252); and Couldwell's three plus two (Chapter 4, pp. 84–94).

Rejected: merging the two middle tiers, because #424's test already ends the argument that made FutureLearn merge them.

### A component is named by structure or by its one domain thing, never by its look

The naming rule is written as the code already follows it:

- Atoms and Templates are named by structure: `Button`, `Notice`, `ListPage`, `RecordDetail`.
- A molecule or organism that exists for one domain thing is named for that thing, in [`GLOSSARY.md`](../../GLOSSARY.md)'s words: `ContractForm`, `BirthPlanView`, `PracticeSwitcher`, `InvoiceSection`, `OfferInbox`, `ClientFieldTemplateEditor`.
- No name describes the look or is a metaphor.
- No name carries a tier suffix, because ADR-0018 already says "the directory carries the tier".

The founder's reason (#1721, Q4 and position 4): a `ContractForm` holds the contract's fields and nothing else does, so its name is its purpose, and a context-free name would hide it. Position 6's bar is what splits a reusable part out of such a component.

Weighed for: Kholmatova's name by purpose (ch. 5, pp. 111–117); and every book's name that is not the look (Kholmatova ch. 5, p. 109; Couldwell Chapter 8, pp. 152–156; Perez-Cruz Ch. 2, pp. 43–44; *Inclusive Components*, Menus & Menu Buttons, p. 64). Weighed against: Frost's context-agnostic "card" and not "product card" (ch. 5, pp. 176–177).

Rejected: Frost's rule for every tier, because it protects reuse and these components are not meant to be reused outside their domain; and Kholmatova's names with personality, because Perez-Cruz shows that a metaphor costs an explanation to every new reader (Ch. 2, pp. 43–44). No component is renamed. The rule is written into ADR-0018: [#1907](https://github.com/markgoho/doula-cloud/issues/1907).

### ADR-0018's definition of a Template holds, with no count aim

A Template owns page-level arrangement, its regions, its empty, loading and load-error states, and the frame, as ADR-0018 and its amendments of 2026-08-30 and 2026-09-29 set out. No aim for how few Templates there are is added.

The founder's reason (#1721, Q5 and position 5): ten routes would otherwise each draw their own empty and error state, which is the drift that ADR-0018 put the Templates under the style-guide gate to stop. The count is held down by position 6's bar, so a second aim adds nothing a reviewer can check.

Weighed for: Perez-Cruz's layouts "also known as templates" (Ch. 3, p. 58); Silver's checkout given its own layout (*Form Design Patterns*, Ch. 2, p. 113); and Kholmatova's strict system that documents templates (ch. 10, p. 253). Weighed against: Frost's template, which shows only content structure while the page carries the variations (ch. 2, pp. 49–55); and Couldwell's "as few as possible", with "features" and "screens" for a product (Chapter 4, pp. 91–95).

Rejected: moving the states back to the route; and a count aim.

### Two consumers that should change together earn a variant or a Template

ADR-0018's bar between its two exits, "One consumer stays a raw exception; two identical consumers earn a variant or a Template", is restated:

- **Two consumers that should change together earn a variant or a Template.** Whether two things are one pattern is whether a change to one should change the other the same way.
- **A thing that only a Template or an atom can hold enters on its first consumer.** The 2026-10-02 `LandingPage` amendment, where "One consumer clears the bar here, and the reason is not a new exit", is this case: the frame and the brief's `display` step can only live in a Template.
- The bar covers a new molecule, organism, Template or variant. An atom keeps growing on its first need, as `docs/design/workflow.md` already says ("grow the atom").

The two named exits themselves hold, and so does ADR-0018's rule that deviation is never anonymous. The 2026-09-20 amendment, that reuse of a Template whose region already fits is outside the bar, also holds.

The founder's reason (#1721, Q6 and position 6): "identical" was a proxy for "should change together", and the proxy fails both ways. Two byte-identical blocks can drift apart on purpose, and two slightly different ones can be one pattern with a variant. With the restated bar, the `LandingPage` case becomes the rule's own exception, in place of an amendment that says it is not one.

Adopted claim: Kholmatova's variant test, whether a change to one should change the others the same way (ch. 8, pp. 197–201). Weighed for: Frost's one-off until a second application wants it (ch. 5, p. 158); and Kholmatova's choice by how specific the purpose is (ch. 10, pp. 274–275). Weighed against: Perez-Cruz's three of eight brands (Ch. 2, pp. 35–36); the Malewiczes' new UI added at once (Design Systems, p. 372); and *Refactoring UI*'s new system for each new decision (Starting from Scratch, p. 28).

Rejected: the bar word for word, because it already needed an amendment that denies being an exit; and three consumers, because Perez-Cruz's number counts brands, not screens. The restated bar is written into ADR-0018: [#1906](https://github.com/markgoho/doula-cloud/issues/1906).

### One interface inventory of the library is made before launch

No gate catches two components that do one job under two names. Before launch, the whole library is looked at once, with position 6's bar as the test for a merge: [#1908](https://github.com/markgoho/doula-cloud/issues/1908), under the holistic application design map, [#405](https://github.com/markgoho/doula-cloud/issues/405).

The founder's reason (#1721, Q7 and position 7): the gates already catch a raw value (`tokens.usage.spec.ts`), a component with no style-guide page (`style-guide.spec.ts`) and a component with no fixture ([ADR-0025](0025-layout-is-verified-across-the-continuum.md)), but none sees a duplicate. The library grew across many sessions since 2026-08, and one look at the whole is cheap while every change is still free. A schedule after launch is a decision for then.

Weighed for: Kholmatova's inventory "every few months" (ch. 3, pp. 71–72); Couldwell's "26 different styles of buttons" and the "Frankenstein monster" (Chapter 6, pp. 121–126); and the Malewiczes' UI audit (UI Audit, pp. 375–386). Weighed against: the gates already named, which catch raw values, missing style-guide pages and missing fixtures.

Rejected: gates only, because none sees a duplicate; and a recurring schedule now, because it is a decision for after launch.

### The style guide stays local-only

`/style-guide` stays a route of the app that is never deployed, as the comment in `app/src/routes/style-guide/+layout.ts` says ("Never deployed -- this route only exists for local `vite dev`").

The founder's reason (#1721, Q8 and position 8): its readers are the founder and his agents, who run it locally. Frost's and Kholmatova's gain comes from other teams, and there are none. Deploying it would put parts of unlaunched screens in public before January.

Weighed for publishing: patterns are "not nuclear codes" (Frost ch. 5, pp. 173–175); and teams who publish "tend to progress faster" (Kholmatova ch. 7, p. 176). Weighed for keeping it private: publishing is optional, since outside attention is not the reason (Couldwell Chapter 3, pp. 77–78).

Rejected: deploying it now or at launch.

### No Practice brand enters the system before launch

Every Practice's Clients see DoulaCloud's one system, in light and dark, with the Practice's name as its identity on screen. No Practice color and no Practice logo appears.

The founder's reason (#1721, Q9 and position 9): a Practice color would be a second accent beside the brief's one. That is a change to [the brief](../design/brief.md) under its own "This is not a license to add a fifth", and any color a Practice picks would have to pass the contrast floors that `tokens.spec.ts` proves, which is a product feature of its own.

Weighed for: Perez-Cruz's token, role and value per brand (Ch. 4, pp. 87–89), and color themed through a few named groups (Ch. 4, pp. 96–100). Weighed against: the brief's Restraint and Rhythm sources (`docs/design/brief.md`); and Perez-Cruz's own Vox Media first attempt at "almost three hundred" variables (Ch. 4, pp. 96–100).

Rejected: a logo only, and a Practice color through a themed role. Each is a feature that needs its own ticket if it is wanted; neither is part of this ADR.

### Where the other conflicts rest

The synthesis names eleven conflicts. Six are decided above: what a template is (position 5, with position 1's template–page split), how many tiers (position 3), when a new thing enters (position 6), how a name is chosen (position 4), public or private (position 8), and the evidence each book rests on (the ground of positions 1 and 2). Positions 7 and 9 decide two questions that are not among the eleven. The other five conflicts hold on records that predate this ADR, as the first round on #1721 sets out:

- **Whether a system holds templates at all.** ADR-0018, on [#410](https://github.com/markgoho/doula-cloud/issues/410), reversed [#93](https://github.com/markgoho/doula-cloud/issues/93)'s "atoms, molecules, organisms only" and put Templates in the system. Kholmatova ties templates to a strict system (ch. 10, pp. 252–253), and this one is strict (the next item).
- **How strict, and what deviation is.** ADR-0018: deviation is never anonymous, and the `tokens:ignore` and `motion:ignore` markers carry a reason. Couldwell's "guidelines" (Chapter 4, pp. 101–103) are for a team whose designers go rogue, and there is no such team.
- **Who may change it, and where its truth lives.** ADR-0019: the code wins, and `/style-guide` renders the real components from the same source, which is Frost's "holy grail" (ch. 5, pp. 159–160). Couldwell's designers who edit tokens (Chapter 8, pp. 175–176) and Kholmatova's library that is "always slightly off-sync" (ch. 10, pp. 279–280) assume a second person.
- **Who looks after it, and how many people it takes.** [ADR-0049](0049-doula-cloud-is-one-person-by-design-and-no-night-needs-him.md): one person by design, with the gates and the Experience axis of `/code-review` as the mechanisms. The Malewiczes' three-person minimum (p. 372) and Couldwell's ambassadors (Chapter 11, pp. 254–257) have no team to apply to.
- **Whether to ask permission.** ADR-0049: there is no stakeholder to ask (Frost ch. 4, p. 107; Kholmatova ch. 7, pp. 166–167).

The standing decisions that the synthesis's last section names, and that no position changes, also hold: web components in the light DOM ([ADR-0003](0003-css-layout-primitives-as-native-custom-elements.md)); the brief's Rhythm source and "This is not a license to add a fifth" ([#409](https://github.com/markgoho/doula-cloud/issues/409)); the tokens and their API ([#417](https://github.com/markgoho/doula-cloud/issues/417), `tokens.usage.spec.ts`), whose values are #1722's; one design system by import for `site/` ([ADR-0051](0051-the-marketing-site-is-sveltekit-not-hugo.md)) and ADR-0021's rejection of `govuk-frontend`; ADR-0024; and the gates (`tokens.spec.ts`, `motion.spec.ts`, `style-guide.spec.ts`, the continuum fixtures).

### What the books add to ADR-0018, and what they change

They add a decision under its Atomic Design names (position 1), a written naming rule (position 4) and a pre-launch inventory (position 7). They change one thing: the bar between its two exits (position 6). Its definition of a Template, its two exits, and "never anonymous" hold (positions 3 and 5).

## What was considered instead

**Adopt nothing.** A legitimate outcome under [#1684](https://github.com/markgoho/doula-cloud/issues/1684), and the decline option on #1721's Q1. Rejected for *Atomic Design* only, for the reason under position 1: ADR-0018 would keep a book's names with no decision under them. Taken for the other five books.

**Adopt *Atomic Design* at a rank in ADR-0021's order.** Rejected, because the order ranks what a screen does, and this book answers what the library's tiers are called (position 1).

**One decision for the six books.** Rejected, because the six do not stand in the same place: one supplies the names the library already uses, and five write about how a team organizes and keeps a system.

**Leave ADR-0018's bar as it stands.** Rejected under position 6, because the bar already needed an amendment that says it is not an exit.

## Consequences

- **The pattern table is ADR-0018, and no table of its own is made.** ADR-0041's third reference-track stage asks for a table an agent checks before building the thing the book is about. For this theme the thing is a component, and what is adopted is a set of names, a naming rule and a bar, not patterns a screen is checked against. ADR-0018 already holds the tiers, the Templates and the two exits, so the naming rule and the restated bar go into it, and `docs/design/workflow.md`'s "Carrying a design into code" step 2, which an agent follows when it adds a component, links to them (#1907). A second table would make the next builder check two places for one rule.
- **The product does not change.** No component is renamed, no route moves, and no Template changes. Each position holds a record already in force, changes a record, or files work that has not started (position 7).
- **Changes to the records, and one piece of work, each carried by its own ticket:**
  - ADR-0018 gets a dated amendment that this ADR is the decision under its Atomic Design names (position 1), and that restates the bar between its two exits (position 6): [#1906](https://github.com/markgoho/doula-cloud/issues/1906).
  - ADR-0018 gets the naming rule (position 4), and `docs/design/workflow.md` links to it: [#1907](https://github.com/markgoho/doula-cloud/issues/1907). It and #1906 may land in one PR.
  - One interface inventory of the library before launch, with position 6's bar as the test for a merge (position 7): [#1908](https://github.com/markgoho/doula-cloud/issues/1908), under #405.
- The two records that read differently from the decisions they rest on, the brief's `--radius-sm` and the `hugo/` paths that three read-backs still name, are a conformance finding, not a decision, and stay with [#1709](https://github.com/markgoho/doula-cloud/issues/1709).
- `docs/manifesto.md` is unchanged, since it indexes value ADRs only, and the book table in [`docs/agents/literature.md`](../agents/literature.md) moves to **decided** in #1684's sweep after all five reference-track ADRs land.
