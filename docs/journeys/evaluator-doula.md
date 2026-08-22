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

**Stage 4 — the first screen after she creates a Practice.** She has spent her
fifteen minutes to get here. This screen is the product's one chance to say
"doula" and "birth plan" back to her. Today it is a menu of seven admin links.

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

**Abandon point**: no price is published anywhere, in the product or outside it
(TB-G2). The only money surface inside the app sells "credits", which are not
explained (TB-G3).

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

**Abandon point**: "Practice name" asks her to name a business she may not think
of as one. Low risk, worth watching.

### Stage 4 — The first screen — moment of truth

**Thinking**: "Show me the thing I came for."
**Pain points**: she gets `Welcome to {practice name}` and seven links: Clients,
Billing, Invite a Staff member, Staff, Plan Templates, Contract Template,
Payments. Six of the seven are administration. The words "birth plan" and "visit"
do not appear. Nothing here is about supporting a birth.

- **4.1** — Land on `/practices/[practiceId]`.
- **4.2** — Choose a link with no guidance on which one comes first.

**Abandon point**: this screen (TB-G4). It is an empty filing cabinet, not proof.

### Stage 5 — Kick the tyres

**Thinking**: "Let me put a fake client in and see what happens."
**Pain points**: she must invent a client to see any real screen.

- **5.1** — `/practices/[practiceId]/clients/new`, enter a name and an email.
- **5.2** — Press **Add Client** (`POST /api/practices/{id}/clients`). This creates
  a Client **and** an Engagement at status `intake`; there is no way to create one
  without the other.
- **5.3** — Open the Engagement from the Clients list.
- **5.4** — See Visits, Care Plan, Birth Plan, Contract, Invoices, and Messages on
  one page. This is the first moment the product looks like doula work — and it is
  four clicks past the point where she was deciding whether to leave.

### Stage 6 — Judge the exit

**Thinking**: "If I hate this in six months, do I get my clients back?"
**Pain points**: this is half of her stated question and the product does not
answer it anywhere.

- **6.1** — Look for an export.

**Abandon point**: there is no export of any kind — no CSV, no download, no
account deletion (TB-G5).

### Stage 7 — Judge the way in (migrating owner)

**Thinking**: "I have two years of clients in a spreadsheet and a Drive folder."
**Pain points**: Tasha *is* the migrating owner. Her existing data is the reason
switching is expensive.

- **7.1** — Look for an import.

**Abandon point**: there is no import (TB-G6). Every Client must be typed by hand,
and the only fields that exist to type are name and email — so the spreadsheet
cannot be reproduced even manually.

## Gaps found

| ID | Stage | Layer | Gap |
| --- | --- | --- | --- |
| TB-G1 | 1 | Both | No marketing site exists. Her journey starts on a page that has not been built. |
| TB-G2 | 2 | Experience | No price is published anywhere, in the product or outside it. |
| TB-G3 | 2 | Interaction | The Billing screen sells "credits" with no explanation of what a credit buys. |
| TB-G4 | 4 | Experience | The first screen after Practice creation is an admin link menu. It shows no doula-specific value at the moment of truth. |
| TB-G5 | 6 | Interaction | No data export. "Can I get out again?" is unanswerable. |
| TB-G6 | 7 | Interaction | No data import for a migrating owner, and Client creation accepts only name and email. |
| TB-G7 | 3 | Experience | Signup grants Owner + Admin + Doula silently. She never learns that roles exist, so she cannot evaluate the product for her second doula. |

Also hit here, filed on their owning maps: **MO-G3** (Client takes name and email
only — which is also why TB-G6 cannot be worked around by hand).
