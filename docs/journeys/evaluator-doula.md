# Tasha Bell — evaluate and decide

- **Persona**: [evaluator-doula.md](../personas/evaluator-doula.md)
- **Goal**: answer one question fast — is this built for what I actually do, and
  can I get out again if it is not?
- **Entry point**: a search result or a doula Facebook group, landing on a
  marketing site that does not exist yet
- **Done looks like**: a Practice with one test Client in it and an intention to
  come back — **or** a clear reason she left. Both close the journey.

Tasha is the only Persona who may legitimately abandon. Every stage therefore
names its **abandon point**: what makes her close the tab there.

## Moment of truth

**Stage 4 — the first screen after she creates a Practice.** She has spent her fifteen minutes to get here. This screen is the product's one chance to say "doula" and "birth plan" back to her. It now does (TB-G4, closed): the seven links moved to the shell's top bar, and this screen names the birth plan, the visits and the contract a first Client will bring.

## Words

Tasha is not yet a domain expert. She is the Persona furthest from `CONTEXT.md`.

| Domain term | What Tasha says | Note |
| --- | --- | --- |
| Practice | "my business", "us two" | Signup asks for a "Practice name" in her first 15 seconds |
| Engagement | "a client" | She has never heard the word and never will unless we teach it |
| Client | "my clients", "the mom" | `CONTEXT.md` avoids "mom" deliberately |
| Plan Template | "the form" | `CONTEXT.md` avoids "form" deliberately — the two agree on nothing |
| Birth Plan | "birth plan" | The one term that matches. It is also the term she is shopping for |

The divergence is the finding: the terms she arrives with are the terms
`CONTEXT.md` explicitly rejects. This is not a naming quibble at the top of the
funnel — it is the vocabulary of the page that has to sell her.

## Stages

### Stage 1 — Find it and judge it in thirty seconds

**Thinking**: "Is this another clinic tool wearing a doula costume?"
**Pain points**: she has been burned by medical software before. Three other tabs
are open. She wants to see "doula" and "birth plan" before she gives anyone an
email address.

- **1.1** — Follow a link from a search result or a Facebook group.
- **1.2** — Read what the product is for.

**Abandon point**: she never arrives at all. There is no marketing site (TB-G1).

### Stage 2 — Find the price

**Thinking**: "What does this cost, per month, for two doulas?"
**Pain points**: pricing is the second question she asks, and an unanswered one
reads as expensive.

- **2.1** — Look for a pricing page.

**Abandon point**: no price is published on the marketing site. That half of TB-G2 moved to [#868](https://github.com/markgoho/doula-cloud/issues/868) (what the site says) and [#284](https://github.com/markgoho/doula-cloud/issues/284) (building it). The in-product half is closed: once signed up, **Billing** states `One Credit covers one Engagement`, `One Credit costs $20.00`, and runs every ledger row through a label instead of printing the raw enum (TB-G2's in-product half and TB-G3, both closed).

### Stage 3 — Sign up

**Thinking**: "Fine, I will spend one minute."
**Pain points**: she has none of Maya's motivation. Every field costs her.

- **3.1** — Open `/signup`.
- **3.2** — Fill four fields: Practice name, Your name, Email, Password.
- **3.3** — Press **Create Practice** (`POST /api/staff/signup`). The Practice, the
  Staff record, and a membership holding Owner + Admin + Doula are created in one
  statement (`signup.go:152`).

This stage is genuinely cheap and is the strongest leg of her journey. One screen,
four fields, no email confirmation step.

- **3.3-a** — Look for anything telling her roles exist. The signup screen now states it, before she commits: the account holds Owner, Admin and Doula, why, and where roles are read and changed afterward. The role-scoped **Staff** nav item repeats it (TB-G7, closed).

**Abandon point**: "Practice name" asks her to name a business she may not think
of as one. Low risk, worth watching.

### Stage 4 — The first screen — moment of truth

**Thinking**: "Show me the thing I came for."
**Pain points**: none remaining. The seven links moved into the shell's persistent top bar ([#452](https://github.com/markgoho/doula-cloud/issues/452)), and this screen is now `OverviewHub`'s zero-Client state, which names the work rather than the filing cabinet and offers one action.

- **4.1** — Land on `/practices/[practiceId]`.
- **4.2** — Read "Nothing is here yet, because no Client is. Add one and this becomes the Client's birth plan, your visits to the Client, and the contract and invoices between you." and follow **Add your first Client**.

**Abandon point**: none remaining (TB-G4, closed by [#287](https://github.com/markgoho/doula-cloud/issues/287)).

### Stage 5 — Kick the tires

**Thinking**: "Let me put a fake client in and see what happens."
**Pain points**: she must invent a client to see any real screen.

- **5.1** — **Find or add a Client** on the Clients list, search, find nobody, follow **Add a new Client**. Intake is one question per page from there (ADR-0017): the name, then date of birth, email, phone, address, then whatever the Practice put on its own Client Field Template.
- **5.2** — Answer the name question and press **Save and come back later**. The save is **free** and creates a Client and nothing else — no Engagement, no credit spent — and lands her on that Client's own detail hub.
- **5.2-b** — **Start new work with {name}** from the hub. This is the act that costs, and it says so before she commits: `Credit cost 1 credit` and `Balance after 2`. She holds Owner, so asking and approving collapse into one act, and the Engagement is created and the Credit locked there. A trial still has a size — three Engagements, not three Clients — but it is now named on screen rather than discovered at a paywall.
- **5.3** — Open the Engagement from the Clients list.
- **5.4** — See Visits, Care Plan, Birth Plan, Contract, Invoices, and Messages on
  one page. This is the first moment the product looks like doula work — and it is
  four clicks past the point where she was deciding whether to leave.

### Stage 6 — Judge the exit

**Thinking**: "If I hate this in six months, do I get my clients back?"
**Pain points**: this is half of her stated question and the product does not
answer it anywhere.

- **6.1** — Look for an export.

**Abandon point**: none remaining. **Settings** offers a whole-Practice data export and **Delete this Practice**, a 30-day countdown before erasure (TB-G5, closed, split across [#288](https://github.com/markgoho/doula-cloud/issues/288) (export) and [#871](https://github.com/markgoho/doula-cloud/issues/871) (deletion)).

### Stage 7 — Judge the way in (migrating owner)

**Thinking**: "I have two years of clients in a spreadsheet and a Drive folder."
**Pain points**: Tasha *is* the migrating owner. Her existing data is the reason
switching is expensive.

- **7.1** — Look for an import.

**Abandon point**: there is no import (TB-G6). Every Client must be typed by hand,
and the only fields that exist to type are name and email — so the spreadsheet
cannot be reproduced even manually.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| TB-G1 | 1 | Both | No marketing site exists. Her journey starts on a page that has not been built. | [#284](https://github.com/markgoho/doula-cloud/issues/284) |
| TB-G2 | 2 | Experience | **Narrowed; the in-product half is closed** ([#285](https://github.com/markgoho/doula-cloud/issues/285)): **Billing** now states `One Credit costs $20.00` before purchase. No price is published on the marketing site — that remainder moved to [#868](https://github.com/markgoho/doula-cloud/issues/868) (what the site says) and [#284](https://github.com/markgoho/doula-cloud/issues/284) (building it). | [#868](https://github.com/markgoho/doula-cloud/issues/868), [#284](https://github.com/markgoho/doula-cloud/issues/284) |
| TB-G3 | 2 | Interaction | **Closed.** The Billing screen now states what a Credit buys ("One Credit covers one Engagement") and its origin column runs every value through a label instead of printing the raw enum. | [#286](https://github.com/markgoho/doula-cloud/issues/286) |
| TB-G4 | 4 | Experience | **Closed.** The first screen is now `OverviewHub`'s zero-Client state: it names the birth plan, the visits, the contract and the invoices a first Client will bring, and offers one action, not a menu. | [#287](https://github.com/markgoho/doula-cloud/issues/287) |
| TB-G5 | 6 | Interaction | **Closed, split in two.** **Settings** offers a whole-Practice data export and **Delete this Practice**, a 30-day countdown before erasure. | [#288](https://github.com/markgoho/doula-cloud/issues/288) (export), [#871](https://github.com/markgoho/doula-cloud/issues/871) (deletion) |
| TB-G6 | 7 | Interaction | No data import for a migrating owner, and Client creation accepts only name and email. | [#289](https://github.com/markgoho/doula-cloud/issues/289) |
| TB-G7 | 3 | Experience | **Closed.** Signup grants Owner + Admin + Doula and now says so, before she commits ([#878](https://github.com/markgoho/doula-cloud/issues/878)). The Staff screen speaks the team's words, `Owner, Admin, Doula`, not the schema's ([#262](https://github.com/markgoho/doula-cloud/issues/262), closed), and a role-scoped **Staff** nav item signposts the roster from every screen. | [#290](https://github.com/markgoho/doula-cloud/issues/290) |

Also hit here, filed on their owning maps: **MO-G3** (Client takes name and email
only — which is also why TB-G6 cannot be worked around by hand).
