# Tasha Bell — evaluate and decide

- **Persona**: [evaluator-doula.md](../personas/evaluator-doula.md)
- **Goal**: answer one question fast — is this built for what I actually do, and can I get out again if it is not?
- **Entry point**: a search result or a doula Facebook group, landing on a marketing site that does not exist yet
- **Done looks like**: a Practice with one test Client in it and an intention to come back — **or** a clear reason she left. Both close the journey.

Tasha is the only Persona who may legitimately abandon. Every stage therefore names its **abandon point**: what makes her close the tab there.

## Moment of truth

**Stage 4 — the first screen after she creates a Practice.** She has spent her fifteen minutes to get here. This screen is the product's one chance to say "doula" and "birth plan" back to her. It now does (TB-G4, closed): the old seven-link menu is gone, replaced by a role-scoped top bar and a **Settings** hub, and this screen names the birth plan, the visits and the contract a first Client will bring.

## Words

Tasha is not yet a domain expert. She is the Persona furthest from `GLOSSARY.md`.

| Domain term | What Tasha says | Note |
| --- | --- | --- |
| Practice | "my business", "us two" | Signup asks for a "Practice name" in her first 15 seconds |
| Engagement | "a client" | She has never heard the word and never will unless we teach it |
| Client | "my clients", "the mom" | `GLOSSARY.md` avoids "mom" deliberately |
| Plan Template | "the form" | `GLOSSARY.md` avoids "form" deliberately — the two agree on nothing |
| Birth Plan | "birth plan" | The one term that matches. It is also the term she is shopping for |

The divergence is the finding: the terms she arrives with are the terms `GLOSSARY.md` explicitly rejects. This is not a naming quibble at the top of the funnel — it is the vocabulary of the page that has to sell her.

## Stages

### Stage 1 — Find it and judge it in thirty seconds

**Thinking**: "Is this another clinic tool wearing a doula costume?" **Pain points**: she has been burned by medical software before. Three other tabs are open. She wants to see "doula" and "birth plan" before she gives anyone an email address.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **1.1** — Follow a link from a search result or a Facebook group.
- **1.2** — Read what the product is for.

**Abandon point**: she never arrives at all. There is no marketing site (TB-G1).

### Stage 2 — Find the price

**Thinking**: "What does this cost, per month, for two doulas?" **Pain points**: pricing is the second question she asks, and an unanswered one reads as expensive.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **2.1** — Look for a pricing page.

**Abandon point**: no price is published on the marketing site. That half of TB-G2 moved to [#868](https://github.com/markgoho/doula-cloud/issues/868) (what the site says) and [#284](https://github.com/markgoho/doula-cloud/issues/284) (building it). The in-product half is closed: once signed up, **Billing** states `One Credit covers one Engagement`, `One Credit costs $20.00`, and runs every ledger row through a label instead of printing the raw enum (TB-G2's in-product half and TB-G3, both closed).

### Stage 3 — Sign up

**Thinking**: "Fine, I will spend one minute." **Pain points**: she has none of Maya's motivation. Every field costs her.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 5 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **3.1** — Open `/signup`.
- **3.2** — Answer three question pages ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)): First name, Last name, Email, Password; then the Practice name (optional); then the state she works from and Timezone.
- **3.3** — Press **Create your Practice** on "Check your answers" (`POST /api/staff/signup`). The Practice, the Staff record, and a membership holding Owner + Admin + Doula are created in one statement (`signup.go:152`).

This stage is genuinely cheap and is the strongest leg of her journey. Four short pages, each saying why it asks, and no email confirmation step before she is in.

- **3.3-a** — Look for anything telling her roles exist. Signup itself no longer says; the first screen after it does: "You hold every role — Owner, Admin and Doula. Anyone you invite holds only the roles you choose." (moved there by #2049). The role-scoped **Staff** nav item leads to the roster (TB-G7, closed).

**Abandon point**: "Practice name" asks her to name a business she may not think of as one. Lower risk since #1536: the field says "(optional)", comes after her own name, and its hint says an empty field gives the Practice her own name. Worth watching.

### Stage 4 — The first screen — moment of truth

**Thinking**: "Show me the thing I came for." **Pain points**: none remaining. The old seven-link menu is gone ([#452](https://github.com/markgoho/doula-cloud/issues/452), closed): Clients, Billing and Staff moved into the shell's persistent, role-scoped top bar, Plan Templates, Contract Template and Payments moved under **Settings**, and this screen is now `OverviewHub`'s zero-Client state, which names the work rather than the filing cabinet and offers one action.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 5 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **4.1** — Land on `/practices/[practiceId]`.
- **4.2** — Read "Nothing is here yet, because no Client is. Add one and this becomes the Client's birth plan, the visits with the Client, and the contract and invoices between the Client and your Practice." and follow **Add your first Client**.

**Abandon point**: none remaining (TB-G4, closed by [#287](https://github.com/markgoho/doula-cloud/issues/287)).

### Stage 5 — Kick the tires

**Thinking**: "Let me put a fake client in and see what happens." **Pain points**: she must invent a client to see any real screen.

**Budget**:

* **Screens**: 5 (Hick's Law, Flow)
* **Decisions**: 3 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **5.1** — On her empty Practice, press **Add your first Client**. Intake is one question (ADR-0017): the name. The date of birth, email, phone, address and whatever the Practice put on its own Client Field Template are added later from the Client's record.
- **5.2** — Answer the name question and press **Save and continue**. The save is **free** and creates a Client and nothing else — no Engagement, no credit spent — and opens the Start work form for that Client.
- **5.2-b** — **Start work with {name}** on that form. This is the act that costs, and it says so before she commits: "Starting work with {name} uses 1 Credit. This Practice has 3 Welcome credits. After this, it has 2." ([#1612](https://github.com/markgoho/doula-cloud/issues/1612)). She holds Owner, so asking and approving collapse into one act, and the Engagement is created and the Credit locked there; she lands on the Engagement's page, which says "Work with {name} started." A trial still has a size — three Engagements, not three Clients — but it is now named on screen rather than discovered at a paywall.
- **5.3** — Open the Engagement from the Clients list.
- **5.4** — See On call, Birth outcome, Contract, Visits, Care Plan, Birth Plan, Invoices, Offers, Messages and Activity on one page. This is the first moment the product looks like doula work — three presses past the first screen.

### Stage 6 — Judge the exit

**Thinking**: "If I hate this in six months, do I get my clients back?" **Pain points**: none remaining. Both halves of her stated question are now answered from **Settings**.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 2 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **6.1** — Look for an export.

**Abandon point**: none remaining. **Settings** offers a whole-Practice data export and **Delete this Practice**, a 30-day countdown before erasure (TB-G5, closed, split across [#288](https://github.com/markgoho/doula-cloud/issues/288) (export) and [#871](https://github.com/markgoho/doula-cloud/issues/871) (deletion)).

### Stage 7 — Judge the way in (migrating owner)

**Thinking**: "I have two years of clients in a spreadsheet and a Drive folder." **Pain points**: Tasha *is* the migrating owner. Her existing data is the reason switching is expensive.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 3 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **7.1** — Look for an import.

**Abandon point**: there is no import (TB-G6, ruled out of v1 on [#175](https://github.com/markgoho/doula-cloud/issues/175)). Every Client must be typed by hand. Each row can be reproduced now, one question at a time, with every column the Practice adds to its own Client Fields (MO-G3, closed), but two years of rows cannot arrive together.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| TB-G1 | 1 | Both | No marketing site exists. Her journey starts on a page that has not been built. | [#284](https://github.com/markgoho/doula-cloud/issues/284) |
| TB-G2 | 2 | Experience | **Narrowed; the in-product half is closed** ([#285](https://github.com/markgoho/doula-cloud/issues/285)): **Billing** now states `One Credit costs $20.00` before purchase. No price is published on the marketing site — that remainder moved to [#868](https://github.com/markgoho/doula-cloud/issues/868) (what the site says) and [#284](https://github.com/markgoho/doula-cloud/issues/284) (building it). | [#285](https://github.com/markgoho/doula-cloud/issues/285) (in-product, closed), [#868](https://github.com/markgoho/doula-cloud/issues/868), [#284](https://github.com/markgoho/doula-cloud/issues/284) (marketing site) |
| TB-G3 | 2 | Interaction | **Closed.** The Billing screen now states what a Credit buys ("One Credit covers one Engagement") and its origin column runs every value through a label instead of printing the raw enum. | [#286](https://github.com/markgoho/doula-cloud/issues/286) |
| TB-G4 | 4 | Experience | **Closed.** The first screen is now `OverviewHub`'s zero-Client state: it names the birth plan, the visits, the contract and the invoices a first Client will bring, and offers one action, not a menu. | [#287](https://github.com/markgoho/doula-cloud/issues/287) |
| TB-G5 | 6 | Interaction | **Closed, split in two.** **Settings** offers a whole-Practice data export and **Delete this Practice**, a 30-day countdown before erasure. | [#288](https://github.com/markgoho/doula-cloud/issues/288) (export), [#871](https://github.com/markgoho/doula-cloud/issues/871) (deletion) |
| TB-G6 | 7 | Interaction | No data import for a migrating owner. **Closed without a build**: ruled out of v1 on [#175](https://github.com/markgoho/doula-cloud/issues/175), first priority after launch. A row can be retyped by hand since MO-G3 closed. | [#289](https://github.com/markgoho/doula-cloud/issues/289) |
| TB-G7 | 3 | Experience | **Closed.** Signup grants Owner + Admin + Doula, and the first screen after it says so ([#878](https://github.com/markgoho/doula-cloud/issues/878); moved off signup by [#2049](https://github.com/markgoho/doula-cloud/issues/2049)). The Staff screen speaks the team's words, `Owner, Admin, Doula`, not the schema's ([#262](https://github.com/markgoho/doula-cloud/issues/262), closed), and a role-scoped **Staff** nav item signposts the roster from every screen. | [#290](https://github.com/markgoho/doula-cloud/issues/290) |
| TB-G8 | 3 | Interaction | Signup fails its budget: `**Screens**: 2 (Hick's Law, Flow)` — measured 5 (four question pages since [#2049](https://github.com/markgoho/doula-cloud/issues/2049), then Overview); `**Decisions**: 5 (Hick's Law, Miller's Law)` — measured 6 on "Check your answers". Second walk, 2026-10-27. | [#2181](https://github.com/markgoho/doula-cloud/issues/2181) |
| TB-G9 | 3, 4 | Experience | Nothing says her Practice was created. `**Confirmation**: yes (Peak-End Rule)` — measured no on stage 3 and on stage 4: Overview says what to do next, and only the Practice's name in the top bar implies it exists. Second walk, 2026-10-27. | [#2182](https://github.com/markgoho/doula-cloud/issues/2182) |
| TB-G10 | 4 | Experience | The top bar puts ten destinations over an empty Practice. `**Decisions**: 5 (Hick's Law, Miller's Law)` — measured 13: ten destinations, the account menu, the pilot feedback link and **Add your first Client**. The body itself offers one action. Second walk, 2026-10-27. | [#2183](https://github.com/markgoho/doula-cloud/issues/2183) |
| TB-G11 | 5 | Interaction | `**Decisions**: 3 (Hick's Law, Miller's Law)` — measured 8 on the Start work form and 4 on the name question, page body only. Second walk, 2026-10-27. | [#2184](https://github.com/markgoho/doula-cloud/issues/2184) |
| TB-G12 | 6 | Both | Both exit acts need a second factor first, and sit at the bottom of the Settings hub. `**Screens**: 2 (Hick's Law, Flow)` — measured at least 5 for a new Owner (Settings, three enrollment steps, Settings again); `**Decisions**: 2 (Hick's Law, Miller's Law)` — measured 14 on Settings. Second walk, 2026-10-27. | [#2185](https://github.com/markgoho/doula-cloud/issues/2185) |

Also hit here, filed on their owning maps: **MO-G3** (Client takes name and email only — closed; it was why TB-G6 could not be worked around by hand).
