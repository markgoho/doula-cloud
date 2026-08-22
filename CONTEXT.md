# Doula Cloud

Multi-tenant CRM for doula practices — Practices employ Staff who work Engagements with Clients through pregnancy and postpartum, tracked through Visits, plans, and billing.

## Language

Each entry may carry a `_Client says_:` line. This is the **Client register** — the
word a Client reads in the portal for a term the team says differently. Doula Cloud is
**one bounded context**; the register translates at the UI edge and does not create a
second model. It is **binding on the client portal** and advisory everywhere else. It
holds single words and short phrases, never sentences. Where the model cannot hold a
fact at all, the register stays silent and the fact becomes a `journey-gap` issue. See
[ADR-0005](docs/adr/0005-one-context-client-register-at-the-ui-edge.md).

**Practice**:
A tenant business. May be a solo doula or a multi-doula business with non-doula Staff.
_Avoid_: Business, tenant, org

**Staff**:
A person who works at a Practice. Holds one or more roles (e.g. Doula, Admin, Owner) via their membership at that Practice; the same person's roles may differ across Practices.
_Avoid_: User, employee, member

**Doula**:
A role a Staff member holds, not a separate entity. A Staff member with the Doula role is the one who works Engagements and Visits with Clients.
_Avoid_: Provider, practitioner

**Owner**:
A role a Staff member holds, carrying full authority over a Practice — its Staff, its Plan Templates, and its billing. A Practice has at least one Owner.
_Avoid_: Admin, superuser, account holder

**Admin**:
A role a Staff member holds, covering the business side of a Practice — Clients, Contracts, Invoices, and scheduling. Narrower than Owner despite the name, and independent of Doula: an Admin may hold neither, either, or both of the other roles.
_Avoid_: Office manager, Administrator, superuser

**Employment type**:
What a Staff member is to a Practice — `employee` or `contractor` — held on their membership, independent of their roles. A role says what a person does; employment type says what they are to the business, so the two are orthogonal and either value may pair with any roles. It governs how much of a Practice a contractor Doula may read. See [ADR-0006](docs/adr/0006-read-follows-the-role.md).
_Avoid_: Staff type, worker type (both read as a role)

**Client**:
The pregnant/birthing person a Practice serves. Does not (yet) cover a partner or support person — portal access for a second person is a future extension of Client, not a new entity.
_Avoid_: Patient, customer, mom

**Engagement**:
The relationship between a Client and a Practice, spanning intake through postpartum, centered on one baby (born or unborn). Deliberately generic so it fits both birth-doula and postpartum-doula work. Carries a status: `intake`, `active`, `postpartum`, or `completed`.
_Client says_: my care ("Your care" as a heading). Never "my pregnancy" — too narrow for postpartum-only work, and wrong for a Client whose pregnancy ended in loss. Each status has one fixed Client label, the same for every Client: `intake` → Getting started, `active` → Ongoing, `postpartum` → Postpartum, `completed` → Care ended. A status that cannot be labelled kindly for **every** Client means the status set is missing a value; that is a gap in the model, never a conditional label.
_Avoid_: Pregnancy (too narrow — excludes postpartum-only work), Case, Relationship

**Offer**:
A Practice proposing an Engagement to a contractor Doula, which she accepts or declines. Her acceptance is what attaches her to the Engagement, and attachment is what she reads by ([ADR-0006](docs/adr/0006-read-follows-the-role.md)). Only contractors are offered work: an employed Doula reads every Engagement at the Practice already, so there is nothing for her to accept. A refusal is durable — a Practice must be able to tell "she said no" from "she has not answered". Not yet in the model (LV-G6); decided on [Lena Vasquez's journey map](docs/journeys/contractor-doula.md).
_Client says_: nothing — the Client never meets the term, and never learns which Doulas refused her Engagement.
_Avoid_: Assignment (the product's other word for who is doing the work, and it carries no agreement — a Visit's `staff_id` is assigned, not offered), Invitation (already means joining a Practice)

**Visit**:
A scheduled meeting between a Doula and a Client within an Engagement. May be the birth itself.
_Client says_: no client-facing surface today — the portal shows no Visit, so no Client word is settled.
_Avoid_: Appointment (reads clinical/medical-provider)

**Care Plan**:
Internal working notes on how the Practice will support a Client's Engagement. Structure is defined per-Practice via a Plan Template (see below); staff-only, never shown in the Client portal. **Staff-only** means not the Client — it does not exclude any Staff role, and an Admin reads it ([ADR-0006](docs/adr/0006-read-follows-the-role.md)).
_Avoid_: Plan, notes

**Birth Plan**:
A standalone document of the Client's labor/delivery preferences, meant to be handed to a third party (e.g. hospital staff). Distinct from Care Plan. Structure is defined per-Practice via a Plan Template; staff-drafted, with a read-only view in the Client portal and a print stylesheet for handoff.
_Avoid_: Plan (ambiguous with Care Plan)

**Plan Template**:
A Practice's own field definitions (short text, long text, single-select, multi-select, checkbox, section header) for its Care Plan or Birth Plan, editable via a settings screen. One template per Practice per plan type, shared across all its Engagements. New Practices get a seeded default. See [ADR-0001](docs/adr/0001-practice-defined-plan-templates.md).
_Avoid_: Form, schema (schema is an implementation term, not domain language)

**Plan Instance**:
A Client's filled-out Care Plan or Birth Plan for an Engagement. Stores a snapshot of the Plan Template's field definitions as they were at creation, so later template edits never alter or break an already-completed plan.
_Client says_: nothing — the term is internal. A Client reads the Birth Plan Instance and calls it her Birth Plan; she never meets the concept itself.
_Avoid_: Response, submission

**Contract**:
The signed agreement governing an Engagement.
_Avoid_: Agreement

**Invoice**:
A bill issued against a Contract.
_Avoid_: Bill

**Payment**:
Money received against an Invoice.
_Avoid_: Transaction

**Message**:
Staff-to-client, bidirectional, in-app communication tied to an Engagement — one continuous thread per Engagement, not split by topic. May carry an image or PDF attachment. Immutable once sent (no edit, no delete) and kept indefinitely as part of the Engagement's permanent record. Delivered via push-triggered fetch: a content-free push notification wakes the client, which then fetches the real content — not a substitute for a phone call in a time-critical situation. See [ADR-0002](docs/adr/0002-message-transport-push-triggered-fetch.md).
_Avoid_: Chat, DM (implies a general-purpose messenger, not an Engagement-scoped record)

**Credit**:
A unit of Doula Cloud's own billing, owned by the **Practice** — never by a Staff member, whatever their roles or Employment type. Only an Owner buys them. What a Credit is spent on is not yet settled (TB-G3).
_Avoid_: Token, seat, point

**Persona**:
One of eight named people the product is designed and tested against, each standing for a distinct way of arriving at Doula Cloud. A Persona is the person behind a Staff or Client record, not the record itself, and may hold several roles or none. Defined in [docs/personas/](docs/personas/).
_Avoid_: User type, actor, role

**Journey**:
The path one Persona takes toward a single goal, from where they arrive to a stated end state, described in two layers: an **experience layer** (what the Persona thinks and feels, and what hurts) and an **interaction layer** (the concrete steps through the product). A Journey is not a task flow — the interaction layer alone is a task flow. Each Persona has exactly one primary Journey. Defined in [docs/journeys/](docs/journeys/).
_Avoid_: Flow, use case, scenario
