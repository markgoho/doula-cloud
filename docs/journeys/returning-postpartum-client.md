# Camille Boyd — coming back to the same Practice, for different work

- **Persona**: [returning-postpartum-client.md](../personas/returning-postpartum-client.md)
- **Goal**: a second Engagement at a Practice that already knows her, doing
  postpartum work only, without starting from scratch
- **Entry point**: she calls Priya. In the product, she arrives as a second portal
  invite
- **Done looks like**: two Engagements at one Practice — one closed and still
  readable, one live and postpartum — reachable from one portal account

Her persona file calls `clients` having no `practice_id` a sign the schema supports
her. It does. **The API and the portal do not**, and they fail in three different
places for three different reasons.

## Moment of truth

**Stage 5 — "a portal account already exists for this identity."** The exact
string the accept-invite endpoint returns (`portalinvite/accept.go`, HTTP 409) when
she tries to claim her second invite with the account she already has. She is being
refused for the crime of being a returning customer. The root is Stage 3, where the
Practice had no way to reach her existing record; this is where it becomes
irrecoverable.

## Words

| Domain term | What Camille says | Note |
| --- | --- | --- |
| Engagement | "this time", "last time" | The register says **my care** / "Your care". She is the one Client who needs the word in the plural, and the portal has no plural |
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

### Stage 5 — The second invite refuses her — moment of truth

**Thinking**: "I already have a login for this."
**Pain points**: `client_portal_users.identity_uid` is `UNIQUE` across the table
(`00006_client_portal_users.sql`), and her second `clients` row gets its own
pending portal row. Accepting that invite with her existing account runs
`UPDATE client_portal_users SET identity_uid = …` against a value already held by
her first row, so the endpoint returns **409 "a portal account already exists for
this identity"** and the page prints exactly that (CB-G3). The only way forward is
a second account under a different email address.

- **5.1** — Priya calls `POST .../portal-invite` on the new Engagement and sends
  the link by hand (**RA-G1**).
- **5.2** — Camille opens `/portal/accept-invite?token=…`, chooses "I already have
  an account", signs in — and is refused.
- **5.3** — She creates a second account with a different email.

### Stage 6 — Two accounts, one person

**Thinking**: "Which one has the new thing in it?"
**Pain points**: each account resolves to exactly one `clients` row, so each shows
one Engagement, and neither can see the other. Even if CB-G1 and CB-G3 were closed
and both Engagements sat under one identity, the portal still could not carry her:
the Engagement chooser exists **only on the login and accept-invite screens**, and
the authenticated layout's entire chrome is a sign-out button. And the chooser
labels each Engagement by `practiceName` alone
(`login/+page.svelte`), so two Engagements at Rooted Birth Collective render as two
identical links (CB-G4).

- **6.1** — Sign in as account A → her 2024 birth Engagement, still `intake`.
- **6.2** — Sign out. Sign in as account B → her postpartum Engagement. There is no
  other route between them.

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

| ID | Stage | Layer | Gap |
| --- | --- | --- | --- |
| CB-G1 | 3 | Interaction | A returning Client cannot be re-used. `POST /api/practices/{id}/clients` always inserts a new `clients` row — no lookup by email, no client search, no add-an-Engagement-to-this-Client endpoint — so one person becomes two Client records and consumes two credits. |
| CB-G2 | 4, 7 | Both | An Engagement cannot declare what kind of work it is. No type or kind column; every Engagement is created at `intake` with no create-time alternative. `CONTEXT.md`'s claim that Engagement "fits both birth-doula and postpartum-doula work" holds only if nobody needs to know which it is. |
| CB-G3 | 5 | Interaction | A person who already has a portal account cannot accept a second invite. `client_portal_users.identity_uid` is `UNIQUE`, so the second claim 409s with "a portal account already exists for this identity" and the only workaround is a second account under a different email. |
| CB-G4 | 6 | Interaction | There is no Engagement switcher inside the portal. The chooser appears only on the login and accept-invite screens, the authenticated layout offers only sign-out, and the chooser labels an Engagement by `practiceName` alone — so two at one Practice would be indistinguishable. |
| CB-G5 | 7 | Both | The Birth Plan link is unconditional. An Engagement with no birth in it still shows it, and the empty state ("No Birth Plan has been created for this Engagement yet") reads as a promise rather than as *not applicable*. |
| CB-G6 | 8 | Both | Nothing shows a person's Engagements over time. Messages and Plan Instances are correctly Engagement-scoped, and nothing sits above them — so neither Camille nor her Practice can see that this is the second time. |

Also hit here, filed on their owning maps: **MO-G4** (her first Engagement cannot
be closed and her second cannot move), **MO-G3** (name and email only, so nothing
about her carries over), **MO-G9** (a second credit for a Client the Practice
already paid for), **RA-G1** (the invite arrives by hand), **NH-G4** (the
unconditional "Welcome to {practiceName}" heading), and
[#212](https://github.com/markgoho/doula-cloud/issues/212) (raw `engagement_status`
and "Engagement" in portal copy).

## Open decisions

- **Is "postpartum-only" a kind of Engagement, a Plan Template choice, or a
  Practice's own service list?** CB-G2 says the fact cannot be recorded. It does
  not say where the fact belongs, and the answer changes CB-G5 (which links the
  portal shows) and the shape of the Plan Template model (ADR-0001).
- **Does one identity hold many Clients, or does one Client hold many
  Engagements?** CB-G1 and CB-G3 are the same problem seen from two tables. Fixing
  the API to reuse a `clients` row makes CB-G3 disappear; fixing
  `client_portal_users` to allow many rows per identity leaves the duplicate
  records in place. They should be decided together, not separately.
