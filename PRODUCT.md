# Product

<!-- impeccable:product-schema 1 -->

This file summarizes product truth for design work. Where it and a canonical record disagree, the record wins: `GLOSSARY.md` for terms, `docs/adr/` for decisions, `docs/personas/` for people, `docs/design/brief.md` (Voice section and governing principle) for copy and behavior.

## Platform

web

## Users

Practice-management software for doula practices, sized for a multi-doula agency (the pilot is a 14-doula agency) and still usable by one doula working alone.

- **Staff**, in the Staff app. Roles combine on one person:
  - **Owner** — runs the Practice, reconciles invoices, approves work (Renata Alvarez, `docs/personas/practice-owner.md`).
  - **Admin** — office manager who never works a Visit (Dee Whitlock, `non-doula-admin.md`).
  - **Doula**, employed or contracted — reads her Clients, logs Visits, messages, covers on-call (Priya Raman, Lena Vasquez).
  - **Solo doula** who is Owner, Admin and Doula at once (Maya Okonkwo).
- **Clients**, in the Client portal — sign a contract, keep a birth plan, pay invoices, message their doula (Hannah Sorensen first pregnancy, Camille Boyd postpartum only, Nadia Haddad pregnancy loss).
- **Evaluators** — a doula deciding whether to sign up (Tasha Bell).

The personas are proto-personas built from the founder's knowledge, not research; a pilot finding overrides them.

## Product Purpose

Run a doula practice end to end: Clients and their Engagements, contracts, Visits, on-call, Messages, invoices and payments (Stripe Connect), with a Client portal on the other side. Success is a Practice that runs its whole week in DoulaCloud and a Client who always knows who is on their birth, whether they have signed, and what they owe.

## Positioning

In order of weight. Design reinforces these through how the product works, not through claims painted onto it.

1. **Built for a whole agency, not a solo doula stretched.** Employed and contracted doulas, an office Admin, assignment and on-call live in one Practice, and one person can hold several roles. For design: a screen shows each person what their role needs, and dense, many-Client views (lists, schedules, invoices) are first-class, not an afterthought.
2. **It feels effortless every time.** Everything answers at once, nothing jumps after it paints, focus lands where expected, a form keeps what was typed through a failed send, a reload or a closed tab, and a long list never stutters. For design: speed and stability are part of the look; an effect that costs smoothness loses.
3. **The Client has a real place of their own.** Contract, birth plan, invoices and messages in one portal that speaks to the Client as "you". For design: the portal is its own experience for a person who is not a software user, not a cut-down Staff screen.
4. **Every change says who did it and when.** An owner can always answer "how did this come to be?". For design: the activity ledger is the product's signature component and is treated consistently wherever a record is shown.

## Operating Context

- A doula at 3am, twenty hours awake, on a phone with a weak or absent signal. The app is offline-maybe: it reads Clients and writes Visits and Messages with no signal; money stays online (ADR-0059).
- An owner at 9am at a desk, reconciling invoices across thirty-plus Clients.
- A Client on their own phone, who may be grieving a pregnancy loss (`docs/personas/loss-client.md`).
- Components may be embedded in a Practice's own website, so they must work at any width they are given.

## Capabilities and Constraints

- Stack: SvelteKit (`app/` Staff app and Client portal, `site/` marketing, `ops/` platform operator), Go BFF in `api/`.
- Design tokens live in `app/src/lib/styles/tokens.css`; components use semantic tokens only, never hard-coded values. The component tiers and Templates are in ADR-0018.
- Terminology is fixed by `GLOSSARY.md` (Practice, Client, Engagement, Visit, Staff, Request, and others).
- American English everywhere, enforced by `app/src/lib/spelling.usage.spec.ts`.

## Brand Commitments

Binding, and not open to redesign:

- **Name:** DoulaCloud, one word (ADR-0050). Nothing else about the visual identity is binding: the palette (Plum Dusk), type, tables, density, and whether a screen uses a table at all are all open.
- **Visual world (Staff app), chosen 2026-11-08:** the category standard, executed at the craft level of Linear, Stripe Dashboard and GOV.UK services. Mark took it over two hands of dealt directions for its consistency: every other world showed "glimpses of something interesting" but none held together. Distinctiveness comes from craft and at most one signature component, never from a costume world. The signature component is the activity ledger (Positioning item 4), shown on records, and the Clients list carries none by design because a list is not a record. The care timeline needed dates the data does not hold; its buildable form, a due strip in the Date column, was a list-level candidate that all three graders in rubric round 7 judged to hurt the Clients list, and it was reverted (60e08e41) (DESIGN.md).
- **Voice** (`docs/design/brief.md`, Voice): no third-person pronoun for anyone; "you" and "your" for the reader; a person's first name once known, the full name where Clients are listed side by side; a clear label with at most one short line under it; a line a person would say aloud; "sign in", never "log in"; domain nouns capitalized where they name a record. Build specs enforce the parts a machine can find (`app/src/lib/copy.*.usage.spec.ts`).
- **Service patterns:** the GOV.UK Design System is the reference for every screen that asks for something or reports a failure — the decision, never the look (ADR-0021, `docs/design/govuk-alignment.md`).
- **Smoothness is checked, not hoped for:** nothing shifts after it paints; every action acknowledges itself within 100ms; loading is skeletal, not a spinner; focus is always visible and predictable; keyboard is a first-class path; a dense list scrolls at 60fps under a real Practice's data.

## Evidence on Hand

- No customers, testimonials, case studies, press, pricing or usage numbers exist yet. Do not fabricate any.
- Research so far: `docs/research/` (pilot agency interviews, when recorded) and the persona files above.

## Product Principles

1. **Conventional in pattern and behavior, distinctive in execution.** Jakob's Law: the nav, buttons and menus work the way other software does; character comes from craft, not novel interactions.
2. **Smoothness is the character.** A speed or stability regression is a design regression.
3. **The words a doula or a Client used beat ours** (ADR-0045).
4. **Quiet enough to read at the end of a long shift**, without being anonymous.

## Accessibility & Inclusion

- Usable by everyone who has to use it; GOV.UK patterns for forms and errors.
- Text contrast at least 4.5:1, control boundaries at least 3:1 (WCAG SC 1.4.11).
- Every screen complete and usable from 320px up, layout intrinsic to its container, not the device (ADR-0024, verified per ADR-0025).
- Copy never assumes a gender (Voice rule 1).
