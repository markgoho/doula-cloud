# Camille Boyd — coming back to the same Practice, for different work

- **Persona**: [returning-postpartum-client.md](../personas/returning-postpartum-client.md)
- **Goal**: a second Engagement at a Practice that already knows her, doing
  postpartum work only, without starting from scratch
- **Entry point**: she calls Priya. In the product, she arrives as a second Engagement on the Client the Practice already holds — not as a second invitation, which is what her old path assumed
- **Done looks like**: two Engagements at one Practice — one closed and still
  readable, one live and postpartum — reachable from one portal account

Her persona file calls `clients` having no `practice_id` a sign the schema supports her. It does, and since [#309](https://github.com/markgoho/doula-cloud/issues/309) so does the login: one Portal Account reaches many Clients, at most one per Practice (ADR-0015), and every Engagement it reaches is listed together. Stages 5 and 6 below are written against that. The stages either side of them still tell the story their own gaps told before those gaps closed, and are held at [#1241](https://github.com/markgoho/doula-cloud/issues/1241).

## Moment of truth

**Stage 6 — both of her Engagements, in one list, under the login she already has.** The portal root lists every Engagement her Portal Account reaches, and names each one **"{Practice}, started {date}"** (`engagementLabel`) — so her 2024 birth and this postpartum work sit side by side at Rooted Birth Collective and are told apart by the one fact that is true for every Client. What used to be her moment of truth was the opposite of this: a `409` at the accept, refusing her for being a returning customer. Stage 5 is where that refusal used to land, and it is now the stage where the product recognizes her instead.

## Words

| Domain term | What Camille says | Note |
| --- | --- | --- |
| Engagement | "this time", "last time" | The register says **my care** / "Your care". She is the one Client who needs the word in the plural, and the portal root list is where it has one: one line per Engagement, each labeled by Practice and start date |
| Engagement status | "we're done" / "we're going" | She needs `completed` on the old one and something postpartum-shaped on the new one. Neither can be set (**MO-G4**) |
| Client | "you have all this already" | Two `clients` rows, one person (CB-G1) |
| Postpartum | "just the nights" | The product has a `postpartum` **status**, not a kind of work. Her whole Engagement is that word, and there is nowhere to put it (CB-G2) |
| Birth Plan | "not this time" | Offered to her anyway (CB-G5) |

## Stages

### Stage 1 — Two years ago, the first Engagement ends

**Thinking**: nothing — it is over and it went well.
**Pain points**: it does not end. No code runs `UPDATE engagements` (**MO-G4**), so
the record of her first birth still reads `intake`. Nothing in the product
distinguishes a finished Engagement from a brand-new one.

- **1.1** — No step. There is nothing to click.

### Stage 2 — She calls Priya

**Thinking**: "They know me."
**Pain points**: outside the product entirely — and that is the finding. Every fact
she is relying on (who she is, who her doula was, how her first birth went) lives
in a message thread and in Priya's memory, not in a field.

- **2.1** — No step. The product is not involved.

### Stage 3 — The Practice types her in again

**Thinking**: "Wait, you're asking me for my email?"
**Pain points**: `POST /api/practices/{id}/clients` **always inserts a new
`clients` row** (`engagement/create.go`). There is no lookup by email, no client
search, and no endpoint that adds an Engagement to an existing Client — Client and
Engagement are created in one indivisible request, by design ("there is no way to
create a Client without one"). It takes name and email only (**MO-G3**), so
nothing from her first Engagement can be carried over even by hand. A second Client
credit is consumed for a person the Practice already paid for (**MO-G9**).

- **3.1** — Priya opens `/practices/[practiceId]/clients` and finds Camille's
  existing record. **It is a dead end** — the list is a read surface; nothing on it
  opens a second Engagement.
- **3.2** — She creates a new Client with the same name and email. Two `clients`
  rows now exist for one person (CB-G1).

### Stage 4 — Declaring it postpartum-only

**Thinking**: "They know it's not a birth this time, right?"
**Pain points**: there is nowhere to say so. `engagements` has no type or kind
column, only `status`, and every Engagement is created at `intake` — the create
handler names the constant and states there is no create-time way to set another.
`CONTEXT.md` says Engagement is "deliberately generic so it fits both birth-doula
and postpartum-doula work"; this stage is the test of that claim, and generic turns
out to mean **silent** (CB-G2). The nearest approximation — moving status to
`postpartum` — is unavailable anyway (**MO-G4**), and would say she has given birth
under this Engagement, which she has not.

- **4.1** — No step. There is nothing to click.

### Stage 5 — The second invitation, and the login she already has

**Thinking**: "I already have a login for this."
**Pain points**: none she meets. The refusal this stage was named for is gone: [#309](https://github.com/markgoho/doula-cloud/issues/309) dropped the table-wide `UNIQUE` on `client_portal_users.identity_uid` that `00006` gave it, and [#819](https://github.com/markgoho/doula-cloud/issues/819) put ADR-0015's own narrower rule in its place — one row per Portal Account per Practice. What she meets instead is that **Priya is stopped first, and correctly**: an invitation is raised per Client, not per Engagement (`invite()` reads `client_portal_users` by `client_id`), and after ADR-0017 Camille is one Client with a second Engagement, so `POST .../portal-invite` answers **409 "this client already has portal access"**. There is nothing to accept because she can already get in. The accept-side refusal that remains — **"you already have portal access at this practice -- sign in instead of accepting a new invitation"**, raised when the sign-in address's Portal Account already reaches a Client at this Practice — is reachable only down the branch where the Practice answered ADR-0017's duplicate screen with *a different person* and saved a second Client record for her (CB-G3, closed).

- **5.1** — Priya calls `POST .../portal-invite` on the new Engagement and is told Camille already has portal access. The Notification email carrying the link is the product's own now, not a hand-delivered copy (**RA-G1**, closed).
- **5.2** — Camille signs in the way she always does — a sign-in link to the address she already uses — and reaches both Engagements. Accepting an invitation is one **Continue** button on `/portal/accept-invite?token=…` (ADR-0026: the invitation is the first sign-in link, and a Client has no password), so there is no "I already have an account" fork to choose any more.
- **5.3** — Down the duplicate-Client branch only: she presses **Continue** on the second invitation and is refused with the one refusal left, told to sign in rather than accept.

### Stage 6 — One account, two Engagements — moment of truth

**Thinking**: "Which one has the new thing in it?"
**Pain points**: none. Her Portal Account reaches her Client, `engagements_identity_visibility` (`00082`) makes every Engagement that Client holds readable before any one of them is chosen, and `decidePortalLanding` sends a person with more than one to the portal root list rather than into one of them. The list, and the persistent way back to it in the authenticated chrome, both read `engagementLabel` — **"{Practice}, started {date}"** — so two Engagements at Rooted Birth Collective are two distinguishable lines rather than two identical ones (CB-G4, closed with [#310](https://github.com/markgoho/doula-cloud/issues/310)).

- **6.1** — Sign in → the root list, holding her 2024 birth Engagement and her postpartum one.
- **6.2** — Open one, then use the chrome's way back to the list and open the other. No sign-out, and no second account.

### Stage 7 — Offered a Birth Plan she does not need

**Thinking**: "Why is that there?"
**Pain points**: the portal home renders the **Birth Plan** link unconditionally.
There is no way to mark an Engagement as having no Birth Plan, because there is no
way to mark an Engagement as anything (CB-G2). Following the link says "No Birth
Plan has been created for this Engagement yet" — which promises one is coming
(CB-G5). Priya may fill one in just to clear the empty state, which puts a
labour-preferences document on a postpartum Engagement.

- **7.1** — Open the portal home → **Birth Plan** and **Contract** links.
- **7.2** — Open the Birth Plan link → the "not created yet" empty state.

### Stage 8 — Nothing came with her

**Thinking**: "I'm explaining all of this again."
**Pain points**: Messages are one continuous thread **per Engagement**
(`CONTEXT.md`), and Plan Instances are per Engagement by design (ADR-0001's
snapshot rule). That is correct scoping and it means her history does not travel.
There is no view — for her or for Priya — of a person's Engagements over time, so
"they know me" is true of Priya and false of the product (CB-G6).

- **8.1** — Read her new, empty message thread.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| CB-G1 | 3 | Interaction | A returning Client cannot be re-used. `POST /api/practices/{id}/clients` always inserts a new `clients` row — no lookup by email, no client search, no add-an-Engagement-to-this-Client endpoint — so one person becomes two Client records and consumes two credits. | [#307](https://github.com/markgoho/doula-cloud/issues/307) |
| CB-G2 | 4, 7 | Both | An Engagement cannot declare what kind of work it is. No type or kind column; every Engagement is created at `intake` with no create-time alternative. `CONTEXT.md`'s claim that Engagement "fits both birth-doula and postpartum-doula work" holds only if nobody needs to know which it is. | [#308](https://github.com/markgoho/doula-cloud/issues/308) |
| CB-G3 | 5 | Interaction | **Closed.** A person who already holds a Portal Account now accepts a further invitation through it: `00081_portal_account_reuse.sql` dropped the table-wide `UNIQUE` on `client_portal_users.identity_uid`, and [#819](https://github.com/markgoho/doula-cloud/issues/819) replaced it with ADR-0015's own rule, `UNIQUE (identity_uid, client_id)`. What is left is not a gap but a deliberate refusal, in a different place and different words: a second Client record for her at the *same* Practice is refused at the accept with "you already have portal access at this practice -- sign in instead of accepting a new invitation", and a second invitation on her existing Client is refused at the invite with "this client already has portal access". | [#309](https://github.com/markgoho/doula-cloud/issues/309) |
| CB-G4 | 6 | Interaction | **Closed.** The portal root lists every Engagement a Portal Account reaches, the authenticated chrome carries a persistent way back to that list, and both label an Engagement with `engagementLabel` — "{Practice}, started {date}" — so two at one Practice are told apart by when each began. | [#310](https://github.com/markgoho/doula-cloud/issues/310) |
| CB-G5 | 7 | Both | The Birth Plan link is unconditional. An Engagement with no birth in it still shows it, and the empty state ("No Birth Plan has been created for this Engagement yet") reads as a promise rather than as *not applicable*. | [#311](https://github.com/markgoho/doula-cloud/issues/311) |
| CB-G6 | 8 | Both | Nothing shows a person's Engagements over time. Messages and Plan Instances are correctly Engagement-scoped, and nothing sits above them — so neither Camille nor her Practice can see that this is the second time. | [#312](https://github.com/markgoho/doula-cloud/issues/312) |

Also hit here, filed on their owning maps: **MO-G4** (her first Engagement cannot
be closed and her second cannot move), **MO-G3** (name and email only, so nothing
about her carries over), **MO-G9** (a second credit for a Client the Practice
already paid for), **RA-G1** (the invite arrives by hand), **NH-G4** (the
unconditional "Welcome to {practiceName}" heading), and
[#212](https://github.com/markgoho/doula-cloud/issues/212) (raw `engagement_status`
and "Engagement" in portal copy).

## Open decisions

Not gaps, and not `journey-gap` issues. Model questions here are out of scope for
this effort and are parked on
[#224](https://github.com/markgoho/doula-cloud/issues/224).

- **Is "postpartum-only" a kind of Engagement, a Plan Template choice, or a
  Practice's own service list?** CB-G2 says the fact cannot be recorded. It does
  not say where the fact belongs, and the answer changes CB-G5 (which links the
  portal shows) and the shape of the Plan Template model (ADR-0001).
- **Does one identity hold many Clients, or does one Client hold many Engagements?** ~~CB-G1 and CB-G3 are the same problem seen from two tables.~~ **Settled: both.** ADR-0015 answers the first — a Portal Account reaches many Clients, at most one per Practice, which is what [#309](https://github.com/markgoho/doula-cloud/issues/309) built and [#819](https://github.com/markgoho/doula-cloud/issues/819) made the table's own rule. ADR-0017 answers the second — a Client is found before one is added, and a further Engagement is asked for against the record she already is. They were decided together, as this said they should be.
