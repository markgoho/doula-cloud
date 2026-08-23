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
A person who works at a Practice. One person is one Staff record, however many Practices they work at; what they are at each Practice is a separate Membership. Holds one or more roles (e.g. Doula, Admin, Owner) via that Membership; the same person's roles may differ across Practices.
_Avoid_: User, employee, member

**Membership**:
What one Staff person is to one Practice — the roles they hold there and their Employment type. A person holds a separate Membership per Practice, and working at more than one Practice is normal, not an oddity: a contractor Doula is the ordinary case. A Membership always carries at least one role, and it is the only thing that makes a person Staff at that Practice. It is created at Practice signup for the founding Owner, or by accepting an Invitation.
_Avoid_: Seat, account, affiliation

**Invitation**:
A Practice asking one person to join it as Staff, naming the email address it is sent to and the roles and Employment type the Membership will carry. Accepting it creates that Membership; the person supplies their own name. An Invitation expires, may be revoked by an Owner, and may be accepted only once. Distinct from an Offer, which is about a single Engagement, not about joining the Practice.
_Avoid_: Invite link, signup, Offer (an Offer is work, not membership)

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
What a Staff member is to a Practice — `employee` or `contractor` — held on their Membership, independent of their roles. Every Membership carries one, including the founding Owner's: `employee` means *inside the business*, not literally on a payroll. A role says what a person does; employment type says what they are to the business, so the two are orthogonal and either value may pair with any roles. An Owner may change it, and the change takes effect at once — it grants and withdraws the ambient reach over the whole Practice, and never touches an **Attachment**. Flipping an `employee` to `contractor` therefore leaves her holding accrued Attachments to everything she ever worked, and she keeps none of it: an accrued Attachment is a record, not a key, and only a granted one reaches. See [ADR-0006](docs/adr/0006-read-follows-the-role.md).
_Avoid_: Staff type, worker type (both read as a role)

**Client**:
The pregnant/birthing person a Practice serves. Does not (yet) cover a partner or support person — portal access for a second person is a future extension of Client, not a new entity.
_Avoid_: Patient, customer, mom

**Engagement**:
The relationship between a Client and a Practice, spanning intake through postpartum, centered on one baby (born or unborn). Deliberately generic so it fits both birth-doula and postpartum-doula work. Carries a status: `intake`, `active`, `postpartum`, or `completed`.
_Client says_: my care ("Your care" as a heading). Never "my pregnancy" — too narrow for postpartum-only work, and wrong for a Client whose pregnancy ended in loss. Each status has one fixed Client label, the same for every Client: `intake` → Getting started, `active` → Ongoing, `postpartum` → Postpartum, `completed` → Care ended. A status that cannot be labelled kindly for **every** Client means the status set is missing a value; that is a gap in the model, never a conditional label.
_Avoid_: Pregnancy (too narrow — excludes postpartum-only work), Case, Relationship

**Offer**:
A Practice asking a Doula to take an Engagement, which she accepts or declines. Her acceptance opens a granted **Attachment**. The Offer is the same record for any Doula, but it does a different job either side of **Employment type**: for a **contractor** it is her only door in — with no ambient reach over the Practice she cannot touch an Engagement she has not been offered, so nothing ever attaches her by accident — while for an **employee**, who reads and writes every Engagement already, it grants no reach and instead settles the claim: she is on this birth, it is in "my Engagements", and "who is on this birth" has her name. Offering is therefore *mandatory* for a contractor and *optional* for an employee, who may simply be assigned.
A Practice may offer one Engagement to several Doulas at once and take the first yes; the rest are superseded, which is a different fact about the Practice from an Offer it withdrew. An Offer stops meaning anything on its own, because a birth will not wait. A refusal is durable — a Practice must be able to tell "she said no" from "she has not answered" — and a refusal ends nothing, because nothing opened. An Offer may be sent to someone who is not Staff yet: it carries an **Invitation**, so one link joins her to the Practice as a contractor Doula and puts the job in front of her, and joining survives losing the job. What an Offer must *say* to be decidable by an outsider is a separate question and not settled here. Not yet in the model (LV-G6); decided on [#229](https://github.com/markgoho/doula-cloud/issues/229) and [Lena Vasquez's journey map](docs/journeys/contractor-doula.md).
While an Offer is open, it is the *only* thing the Doula reads: a copy made when it was sent, not a window onto the Practice. The Engagement, the Contract, the Plans and the roster all refuse her exactly as they refuse a stranger, because only an **Attachment** reaches. The copy carries the Client's first initial and general area, the due date, her fee, and whatever else the Practice chooses to write in its own words — deliberately thin, because a Practice offers work as *here is a client, due this date, do you want her*, and because naming a Client to someone who then declines has told an outsider that a named woman in that neighbourhood is pregnant and using this agency. She reads her own fee and never what the Practice charges the Client; those are two different facts that both look like "the price". The copy does not change after it is sent — a Practice wanting different terms withdraws the Offer and sends another — and once the Offer ends, however it ends, the Client's details stop being served at once. What survives is the fact of the asking: who asked, when, for how much, and what she answered. Decided on [#230](https://github.com/markgoho/doula-cloud/issues/230).
_Client says_: nothing — the Client never meets the term, and never learns which Doulas refused her Engagement.
_Avoid_: Assignment (the product's other word for who is doing the work, and it carries no agreement — a Visit's `staff_id` is assigned, not offered; an employee may be assigned, a contractor never), Invitation (that is joining a Practice — an Offer may carry one, but it is not one), Job posting (nothing is browsable; an Offer is addressed to a person the Practice chose)

**Attachment**:
The record that a Doula is on an Engagement — the answer to "who is working this birth", and the only answer a Doula has to "which of these forty Engagements are mine". It has two origins, and the difference is load-bearing. An **accrued** attachment opens by itself the first time she *does* something on the Engagement herself — logs a Visit, sends a Message, edits a Plan Instance, acts on a Contract. Merely reading never attaches. A **granted** attachment is someone deciding she is on it, and the two ways in are not symmetric. An Admin may attach an **employee** directly — naming her on a Visit is granted, not accrued, because she has done nothing — and this is what lets a Practice say who is on an Engagement before that person has touched it. A **contractor** can only be attached by her own acceptance of an **Offer**: nobody can put an outsider on a Client's birth without her agreement. Where an Offer carried a fee, the accepted fee is part of the attachment, so what she agreed to cannot be rewritten afterwards. Only a **granted** attachment grants reach — a contractor reads and writes the Engagements she holds one for, and nothing else. An accrued attachment grants nothing: it belongs to a Doula whose **Employment type** already reaches the whole Practice, so it is a record of work, never a key. Attachment is for Doulas only; an Owner or Admin touching an Engagement is not attached to it.
An Attachment ends but is never deleted, and ending is not erasure — "she was on this from February to May" is more of the record than "she was on this", not less. It ends when the Practice takes her off, when she drops out, when the Engagement is completed, or when her Membership at the Practice ends. A contractor's reach stops the moment it does; the Visits she worked are untouched, so the Practice keeps its record of who did what. Not yet in the model (RA-G4, LV-G4); decided on [#228](https://github.com/markgoho/doula-cloud/issues/228).
_Client says_: my doula — the Client meets the person, never the record.
_Avoid_: Assignment (the product's other word, and it carries no agreement — a Visit's `staff_id` is assigned, not accepted), Membership (that is joining the Practice, not joining a piece of its work)

**Visit**:
A scheduled meeting between a Doula and a Client within an Engagement. May be the birth itself. A Visit's `staff_id` is who is doing *that meeting*; an **Attachment** is who is on the *Engagement*. They are different facts and neither replaces the other — naming a Doula on a Visit is one of the things that attaches her.
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

**Sandbox**:
The Stripe environment Doula Cloud develops and walks against — separate
data, separate API keys, real API and real webhooks, no money movement.
Stripe used to call this **test mode** and renamed it; keys from a Sandbox
still start `sk_test_`, and older tickets and Stripe's own URLs still say
`test`. Say Sandbox. A Sandbox is an environment, not a toggle, so "which
Sandbox" is always a real question. See [docs/environment.md](docs/environment.md).
_Avoid_: Test mode, test account, staging

**Connected account**:
The Stripe account a Practice owns and Doula Cloud onboards it into, so
Clients can pay that Practice directly. A Stripe **Accounts v2** Account
carrying the **merchant** configuration and a full Stripe dashboard —
Stripe refuses to create the older Accounts v1 shape for a new
integration. Its state is two capability statuses, `card_payments` and
`stripe_balance.payouts`, each one of `active` / `pending` / `restricted`
/ `unsupported`, plus a list of outstanding **requirements**. There is no
single "connected / not connected" fact to read, which is why the Payments
screen reports five states rather than two. See
[ADR-0007](docs/adr/0007-connect-account-state-is-two-capabilities-and-a-requirements-list.md).
_Avoid_: Standard account, merchant account, Stripe account (ambiguous — the platform has one too)

**Thin event**:
A Stripe v2 webhook delivery that carries no object — only an event id, a
type, and a reference to the resource that changed. The receiver fetches
the current state itself. Distinct from a v1 **snapshot event**, which
embeds the object. A Stripe event destination carries one payload kind or
the other, never both, which is why Doula Cloud has two Connect webhook
routes.
_Avoid_: Webhook payload, notification (both blur the thin/snapshot distinction that matters here)

**Persona**:
One of eight named people the product is designed and tested against, each standing for a distinct way of arriving at Doula Cloud. A Persona is the person behind a Staff or Client record, not the record itself, and may hold several roles or none. Defined in [docs/personas/](docs/personas/).
_Avoid_: User type, actor, role

**Journey**:
The path one Persona takes toward a single goal, from where they arrive to a stated end state, described in two layers: an **experience layer** (what the Persona thinks and feels, and what hurts) and an **interaction layer** (the concrete steps through the product). A Journey is not a task flow — the interaction layer alone is a task flow. Each Persona has exactly one primary Journey. Defined in [docs/journeys/](docs/journeys/).
_Avoid_: Flow, use case, scenario
