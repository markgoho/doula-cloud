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

A bare **Template** is not a domain term. The domain's templates are always written
qualified — **Plan Template**, **Client Field Template** — and an unqualified "Template"
means the Atomic Design layer in `app/src/lib/components/templates/`, per
[ADR-0018](docs/adr/0018-templates-are-a-design-system-layer-with-two-named-exits.md).

**Practice**:
A tenant business. May be a solo doula or a multi-doula business with non-doula Staff.
_Avoid_: Business, tenant, org

**Staff**:
A person who works at a Practice. One person is one Staff record, however many Practices they work at; what they are at each Practice is a separate Membership. Holds one or more roles (e.g. Doula, Admin, Owner) via that Membership; the same person's roles may differ across Practices.
_Avoid_: User, employee, member

**Work State**:
The US state a Staff member works from. A fact about the person, not about any one Membership, and not a mailing address — where they differ, it is where she works, not where she lives. Self-reported, never verified, and the only thing that makes a Practice's sales tax computable: the taxable share of a Credit purchase is the Practice's New York-located Staff over all its Staff. **Hers alone to assert and hers alone to change** — an Owner and an Admin read it on the roster and cannot write it, so it is stated once at onboarding and corrected on her own account screen, never by the Practice. Every assertion is kept, including one that repeats the value unchanged: saying "still New York, as of today" is a real act, because the date it carries is the only signal that the answer might be stale. A correction applies **from that day forward** and re-prices nothing — a Credit purchase records the tax it actually charged, so an earlier receipt stands. Decided on [#415](https://github.com/markgoho/doula-cloud/issues/415) and [#437](https://github.com/markgoho/doula-cloud/issues/437).
_Avoid_: Location, address, region, state (a bare "state" in this codebase is a lifecycle state)

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
One Practice's record of the pregnant/birthing person it serves — not the person herself, who lives in the **Portal Account**. Belongs to exactly one Practice, and no Client fact crosses a Practice. Does not (yet) cover a partner or support person — portal access for a second person is a future extension of Client, not a new entity. Her record has two layers: a **structural** core the system itself reads and depends on — her name, the address the Practice reaches her at, her phone number and her home address — and whatever **Client Field Template** fields her Practice has chosen to add beside it. Only a first name is ever demanded of a staff member; everything else in the core may be missing, because intake is often typed on a phone by someone on call and a fake value is worse than an empty one. She has two names and the difference is load-bearing: her **legal name** is what a Contract and an Invoice call her, and her **preferred name** is what every screen and every Message calls her. Nothing in the structural core asserts that she is pregnant, so the record is equally true of a postpartum-only Engagement and of a Client whose pregnancy ended in loss. The two layers do not overlap: a Practice-defined field may never restate, extend or shadow a structural fact, because a fact the system sends email with cannot live in two places. A Practice asking to record a middle name is therefore not asking for a field, it is saying the structural name has the wrong shape. The address the Practice reaches her at is the Practice's own contact detail, not her login: she chooses what she signs into her **Portal Account** with, and the two are separate fields that need never match. Two records of one woman at one Practice can only be made deliberately, and one of them can be **absorbed** into the other while it still has no Engagement, no Engagement Request and no portal access behind it: its facts fold into the record already on file, and it survives only as a pointer at the record that kept them. Where both records carry history, nothing merges them and the product says so. Decided on [#727](https://github.com/markgoho/doula-cloud/issues/727). Decided on [#369](https://github.com/markgoho/doula-cloud/issues/369), [#370](https://github.com/markgoho/doula-cloud/issues/370) and [#374](https://github.com/markgoho/doula-cloud/issues/374). See [ADR-0015](docs/adr/0015-three-facts-on-an-engagement-the-person-lives-in-the-login.md).
_Client says_: nothing of her record — for v1 the portal shows her none of it, not even the structural core, because she has no way to correct what is wrong there and telling her doula through a Message is one line. She meets only the name the portal already addresses her by. Decided on [#370](https://github.com/markgoho/doula-cloud/issues/370); reopens when she gains a correction path.
_Avoid_: Patient, customer, mom

**Client Field Template**:
A Practice's own list of the extra facts it records about every Client it serves, beyond the structural core — a referral source, a favourite colour, whatever that Practice works by. Sibling of the **Plan Template**, and drawn from the same six field types, but it differs from one in the way that matters: a Client's values are read **live** against the Practice's current list, never snapshotted like a **Plan Instance**. A Plan Instance is a document with a date on it, so what it said then is the point; a Client record is a running description of a person, so what it says now is the point. Removing a field **archives** it rather than deleting what was recorded. Every field is staff-only — none is ever shown to the Client in the portal, and a fact she should see is evidence it wants to be structural instead. Decided on [#369](https://github.com/markgoho/doula-cloud/issues/369).
_Client says_: nothing — she never meets the term or the fields.
_Avoid_: Custom fields, intake form (the second is a client-filled form, which Doula Cloud does not have)

**Portal Account**:
One person's login to the client portal. Reaches many Clients, at most one per Practice — it is the only thing in the model that knows two Client records are the same woman, and she is the one who told it, by accepting each Practice's invite. Legible only from a portal session, never from a staff one. It is not a **Staff** identity and never becomes one: the same human may hold both — a doula who is a Client somewhere, including at the Practice she works at — and the two are separate records that nothing in the model joins, so no query can learn they are the same person. See [ADR-0015](docs/adr/0015-three-facts-on-an-engagement-the-person-lives-in-the-login.md). Decided on [#168](https://github.com/markgoho/doula-cloud/issues/168).
_Client says_: nothing — she meets a login, never the term.
_Avoid_: User, account, portal user

**Engagement**:
The relationship between a Client and a Practice, from onboarding through the end of care, centered on one baby (born or unborn) and belonging to one Client for life. Deliberately generic so it fits both birth-doula and postpartum-doula work. Declares its **kind** — `birth` or `postpartum`, what the Practice sold — and carries a nullable due date. Carries a status: `intake` — the Client has agreed to work with the Practice and setup is under way, care has not started; `active` — care is booked or happening; `completed` — care has ended, for whatever reason. Two staff-only facts sit beside the status and never merge into it: a **birth outcome** (how the birth went) and an **ending reason** (why care stopped). There is no lead or prospect: an Engagement exists only for a person the Practice has taken on. It comes into existence when an **Engagement Request** is approved, and never before — which is what keeps the Credit locking at creation now that saving a Client is free. Decided on [#393](https://github.com/markgoho/doula-cloud/issues/393). See [ADR-0015](docs/adr/0015-three-facts-on-an-engagement-the-person-lives-in-the-login.md).
_Client says_: my care ("Your care" as a heading). Never "my pregnancy" — too narrow for postpartum-only work, and wrong for a Client whose pregnancy ended in loss. Each status has one fixed Client label, the same for every Client: `intake` → Getting started, `active` → Ongoing, `completed` → Care ended. A status that cannot be labelled kindly for **every** Client means the status set is missing a value; that is a gap in the model, never a conditional label. Kind, the birth outcome and the ending reason are staff-only and have no Client word at all.
_Avoid_: Pregnancy (too narrow — excludes postpartum-only work), Case, Relationship, Lead (nothing in the product holds an enquiry that has not been taken on)

**Engagement Request**:
A staff member asking her Practice to start paid work with a Client. Its own record, never a status on the **Engagement**, because an Engagement that does not yet exist cannot sit in front of every query that reads them, and because a **Credit** locks when an Engagement is created — so creation *is* the approval. Any Staff member may ask except a **contractor** Doula, who originates nothing at a Practice she does not belong to; work reaches her as an **Offer**. An Owner or an Admin skips the asking and starts directly, and where the asker already holds approval authority the two are one act, so a solo Practice is the same rule rather than a special case. The asker states the kind and the due date, because she is the one who did the intake call, and the approver agrees or refuses exactly what was described rather than amending it. A refusal is durable and carries a reason, so "why is there a Client here with no work?" always has an answer; the asker may withdraw her own; and nothing expires, because a Request holds no secret and never leaves the Practice. While one is pending the Doula can do no work at all — Messages, Contracts and Visits all hang off an Engagement that is not there yet. Decided on [#393](https://github.com/markgoho/doula-cloud/issues/393). See [ADR-0017](docs/adr/0017-twelve-columns-a-practice-defined-layer-and-an-engagement-that-is-asked-for.md).
_Client says_: nothing — she never learns that her Practice approved anything, only that her care has begun.
_Avoid_: Lead, prospect, enquiry (an Engagement Request is a person the Practice has already taken on, only not yet paid for), Draft engagement, Pending engagement (there is no Engagement until it is approved)

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
_Client says_: **visits** — "Your visits" as a heading, her own phrasing ("when she comes over", "when Maya came"). Settled on [#433](https://github.com/markgoho/doula-cloud/issues/433): a Client sees the visits on her own Engagement, past and scheduled, with who is coming. There is no client-facing surface yet and a Visit still carries no date ([#250](https://github.com/markgoho/doula-cloud/issues/250)), so the word is decided ahead of the screen rather than after it.
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
A bill issued against a Contract. Every Invoice a Client is ever sent by one Practice bills the same **Stripe Customer**: a Client has at most one per connected Stripe account, made the first time that Practice bills her and reused by every bill after it. A Practice that re-connects under a new Stripe account gives her a second one there, because a Customer only exists inside the account that made it. The mapping is a record in this database rather than something inferred, which is also what lets a simulation run make her Customer itself — against a Stripe test clock — and leave the product with nothing to create. Decided on [#780](https://github.com/markgoho/doula-cloud/issues/780).
_Avoid_: Bill

**Payment**:
Money received against an Invoice.
_Avoid_: Transaction

**Message**:
Staff-to-client, bidirectional, in-app communication tied to an Engagement — one continuous thread per Engagement, not split by topic. May carry an image or PDF attachment. Immutable once sent (no edit, no delete) and kept indefinitely as part of the Engagement's permanent record. Delivered via push-triggered fetch: a content-free push notification wakes the client, which then fetches the real content — not a substitute for a phone call in a time-critical situation. See [ADR-0002](docs/adr/0002-message-transport-push-triggered-fetch.md).
_Avoid_: Chat, DM (implies a general-purpose messenger, not an Engagement-scoped record)

**Notification**:
A push-delivered alert to a person, on any channel (browser Web Push or email) — content-free, pointing at the product rather than carrying it. Two voices, fixed by who receives it, never by subject matter: a **Platform Notification** is Doula Cloud speaking as itself, to a Staff member or Owner; a **Practice Notification** is Doula Cloud speaking as the Practice, to that Practice's Client. Neither voice's `From`, subject, or body may carry a Client name, Engagement detail, or the Practice's own name — for v1, no Notification of either voice names a Practice at all. What a recipient may decline differs by channel, and not by preference: a Client can mute **push** for one Engagement (#303), but she can turn off **no email Notification at all** — every email kind Doula Cloud sends is transactional or relationship mail, and none can be declined while keeping a working account, so `notification_channel` holds one value, `'push'`. See [ADR-0009](docs/adr/0009-notification-is-one-term-two-voices-keyed-by-recipient.md), [ADR-0011](docs/adr/0011-notification-sending-identity-is-one-shared-domain.md), [ADR-0029](docs/adr/0029-email-suppression-is-address-keyed-and-outbox-wide.md).
_Avoid_: Message (a Notification is never in-app, bidirectional, immutable, or Engagement-scoped — it is a one-way alert on an outside channel)

**Email Suppression**:
A durable, address-keyed block on further Notification email, triggered by a Mailgun complaint or hard bounce — not a Client choice, and not scoped to a Notification's voice, channel, or Engagement. Because every email-sending outbox shares one Mailgun domain, one suppressed address is unreachable by all of them at once. A bounce-caused suppression can be **cleared** by an Owner or Admin of a Practice the address belongs to — at Mailgun as well as locally, since either one alone still refuses the send. A complaint-caused one never is. See [ADR-0029](docs/adr/0029-email-suppression-is-address-keyed-and-outbox-wide.md).
_Avoid_: Opt-out, unsubscribe (CAN-SPAM's opt-out rules never reach this mail at all — suppression is Mailgun's own deliverability protection reacting to what happened, not a right a Client exercises)

**Activity**:
The record of what has happened to something and who did it — the answer to "how did this thing come to be?", which `CLAUDE.md` makes a standing expectation of every feature. One **Activity entry** names the thing it happened to (a Client, an Engagement, a Membership), what happened, when, and the actor. An actor is one of three kinds and only one is a Staff member: a **Staff** member did it, the **Client** did it (she signed the Contract, she paid the invoice), or **Doula Cloud** did it with nobody asking (an invite email went out, an Offer was superseded when another Doula accepted first). Append-only and never edited or deleted, because an audit trail that can be rewritten is not one. Read through the same gate as the thing it describes — an employed Doula who may not read a Contract's money does not read the invoice entries either, or the ledger becomes a way round the gate. Decided on [#433](https://github.com/markgoho/doula-cloud/issues/433). See [ADR-0022](docs/adr/0022-one-activity-log-with-a-subject-and-three-kinds-of-actor.md).
_Client says_: not the word — a Client reads her own Activity under "Everything that has happened", and only her own: her documents, her money, her visits, never who inside the Practice did what.
_Avoid_: Audit log, event log, history (all three are true and all three are engineering words; "System" as an actor name — the product acting on its own behalf is called Doula Cloud)

**Erasure**:
Destroying a Client's personal data because she asked, honoring US state right-to-delete law. It is the one act in this product that leaves the record less complete on purpose, and it is not **ending** — an Attachment ends, an Engagement completes, a Contract is voided, and all three leave more of the record than they found. Erasure is the other thing. Only the **Owner** may run it, the same seat that throws the MFA switch. Her record is redacted where it sits and never deleted: the row keeps its id, so every Message, Contract, Visit, Invoice and Engagement Request that names her still resolves and the Practice keeps the financial and clinical record it is obliged to keep. Her **Activity** is not edited — it is shredded, by destroying the key its entries were sealed under, so the fact that something happened and who did it survives while what changed becomes unreadable. Outside Doula Cloud, every Stripe Customer she ever had is deleted at once — one per connected Stripe account since [#780](https://github.com/markgoho/doula-cloud/issues/780), plus any older one recorded on an Invoice from when each bill raised its own — and her transactions are redacted once Stripe will allow it, 90 days after they were taken; her portal login is deleted and the session holding it with it. What is *not* erased is free text a person typed by hand — a Message, a signed Contract's prose, a Plan Instance answer — and that is a stated limitation, not an oversight. Decided on [#394](https://github.com/markgoho/doula-cloud/issues/394). See [ADR-0027](docs/adr/0027-erasure-redacts-in-place-and-shreds-the-key.md).
_Client says_: delete my information — she asks the Practice, never the product; there is no button of her own.
_Avoid_: Deletion (nothing is deleted — the row and every record naming her stay), Anonymization (a stable pseudonym is a re-identification key against the free text erasure does not scrub), Ending, Closing, Archiving (all three are the opposite act: they preserve)

**Credit**:
A unit of Doula Cloud's own billing, owned by the **Practice** — never by a Staff member, whatever their roles or Employment type. An Owner or an Admin buys them — an Admin who may approve an Engagement, and who already reads the balance and the ledger, may also top it up. One Credit covers one **Engagement**: it locks when the Engagement is created, and a locked Credit is spent. Credits do not expire, and a Practice may hold them for as long as it likes. A purchased Credit that is still unspent is refundable for three years from the day it was bought, at the price paid; a Credit granted free of charge is not refundable at all, and a spent one never is. Credits are not fungible for that purpose — which purchase a Credit came from decides what refunding it is worth, and the oldest are spent first. One costs **$20.00**, the same whatever kind of Engagement it opens and whatever the Practice charges its own Client. A Credit is also **granted**, never only sold: a **founding grant** is what a Practice joining the pilot receives, three for each Staff member it has on the day it joins, and it is a different thing from the **signup bonus** any new Practice receives. A founding grant is issued by Doula Cloud itself, counted once and never topped up, and it names the person who issued it — "who gave this Practice its Credits?" is a question the ledger answers. Neither grant expires; both are spent before purchased Credits are. Decided on [#374](https://github.com/markgoho/doula-cloud/issues/374), [#390](https://github.com/markgoho/doula-cloud/issues/390) and [#439](https://github.com/markgoho/doula-cloud/issues/439).
_Avoid_: Token, seat, point

**Practice Page**:
The public page Doula Cloud publishes for a **Practice** that has no website of its own, at one address assigned once and never moved. It is what that Practice declares to Stripe, and it carries what Stripe asks a business to show: the name, what the Practice offers, a cancellation or refund position, and one Owner to contact. A Practice answers the website question one of two ways — her own address, or a Practice Page — and the two are exclusive. A Practice Page is **published** by an Owner and **live** only once Doula Cloud has fetched it and found it there; between the two it is neither, and the product says so rather than claiming success. A Practice that switches to her own website keeps her address but no longer has a page at it. Decided on [#440](https://github.com/markgoho/doula-cloud/issues/440), [#441](https://github.com/markgoho/doula-cloud/issues/441) and [#443](https://github.com/markgoho/doula-cloud/issues/443).
_Avoid_: Profile, microsite, landing page, listing

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
One of nine named people the product is designed and tested against, each standing for a distinct way of arriving at Doula Cloud. A Persona is the person behind a Staff or Client record, not the record itself, and may hold several roles or none. The nine are fixed: a person a simulation needs who is not one of them is an **Extra**, not a tenth Persona. Defined in [docs/personas/](docs/personas/).
_Avoid_: User type, actor, role

**Extra**:
A named person in a **World** who is not a **Persona** — a name, a Practice, a role, an **Employment type**, and one reason to open the app, and nothing beyond that. An Extra is a person and not a row: she is invited and she accepts like anybody else, because a **World** is seeded through the product rather than into the database. What she lacks is an interior life — no journey map, no test plan, and no **Friction log** written in her voice. She exists so that a Practice is the size a real one is, and so that the people a Persona works beside are people rather than volume. Decided on [#761](https://github.com/markgoho/doula-cloud/issues/761).
_Avoid_: NPC, background user, filler, dummy staff (all four say row, and an Extra walks through the same door everyone else does)

**Journey**:
The path one Persona takes toward a single goal, from where they arrive to a stated end state, described in two layers: an **experience layer** (what the Persona thinks and feels, and what hurts) and an **interaction layer** (the concrete steps through the product). A Journey is not a task flow — the interaction layer alone is a task flow. Each Persona has exactly one primary Journey. Defined in [docs/journeys/](docs/journeys/).
_Avoid_: Flow, use case, scenario

**Friction log**:
What one Persona's walk through one simulation Run emits: an ordered record of the acts she attempted and what each one cost her. It holds two registers that are never mixed — an **observed** one, third person, where every claim carries evidence and a measured timing, and a **narrated** one, in her own voice, which interprets an observed act and asserts nothing about the code. It is **heuristic evaluation, never user research**: a Persona is a hypothesis, and a Friction log describes one agent walking one seeded world once, never what users do. Distinct from a filed finding, which is what the log is the input to. Defined in [docs/simulation/](docs/simulation/).
_Avoid_: Diary, session notes, user feedback, research (all four claim a person was there)

**Sighting**:
One encounter with a defect during a **Run** — one act, by one cast member, at one moment, identified by the Run, the person, and the entry id. A Sighting is an observation and never a work item: it lives in a **Friction log** and is cited inside the **Finding** it belongs to. Several Sightings of one defect are not several problems, and their number describes how often that **World** put a person in front of it, never how much the defect matters. Decided on [#766](https://github.com/markgoho/doula-cloud/issues/766).
_Avoid_: Occurrence, hit, repro, instance (the first three read as a bug report, and a Sighting is not one)

**Finding**:
A defect a **Run** exposed, as one thing to be fixed — however many **Sightings** of it there were. Its identity is the tracker issue, and it has no other id: a Run cites the gap ids the journey maps own and never mints one beside them. Two Sightings are one Finding when the same change would answer both. A Finding carries every Sighting of it, states plainly what happened, and carries no severity and no priority, because pre-launch everything found is fixed. Distinct from a **Friction log** entry, which is the observation a Finding is filed from. Defined in [docs/simulation/findings.md](docs/simulation/findings.md). Decided on [#766](https://github.com/markgoho/doula-cloud/issues/766).
_Avoid_: Issue (that is the tracker's word for the record, not the defect), gap (a gap belongs to a Journey and carries a gap id), defect report, ticket

**Run**:
One pass of the simulation harness: a **World**, a cast of **Personas** and **Extras**, and a stated span of simulated time walked end to end, producing a **Friction log** per Persona. Reproducible from its seed, and identified by the day it started in real time — so a Run is a thing you can repeat against a product that has moved on, and compare against the Run before it. A bare "run" elsewhere in this repo means an execution of the test suite; this term is always qualified in prose where both could be meant.
_Avoid_: Simulation (the harness, not one pass of it), session, test run

**World**:
The starting state a **Run** walks, described as businesses that exist *outside* Doula Cloud rather than as rows: the Practices, the **Personas** and **Extras** in them, and the work each Practice is already carrying on the day it meets the product. A World is **seeded through the product, never into the database** — an agency's roster and book get in the way a real one would, because whether they can is the question a Run exists to ask. It is an input the harness accepts and not a constant, so two Runs are comparable only if they walked the same one. Distinct from a **Fixture**, which is content a surface is *measured* with; a World is a place people work. Defined in [docs/simulation/worlds/](docs/simulation/worlds/). Decided on [#761](https://github.com/markgoho/doula-cloud/issues/761).
_Avoid_: Seed data, test data, scenario, fixture (a Fixture is the longest realistic value for a field; a World is an agency with a history)

**Day zero**:
The moment one Practice's **World** meets the product — the day its Owner signs up and starts moving a business that already exists into an empty tenancy. It belongs to a Practice and not to a **Run**, so a World may hold several, staggered. It is the one path no test plan has ever walked, and the order the product forces on that day is an observation rather than something a Run scripts.
_Avoid_: Onboarding (that is the product's own word for a first session, and day zero may take many), migration, go-live, import (nothing imports — a person types it in)

**Calendar**:
How much work a **World** does over a **Run**'s span, and when: the counts a Practice carries and takes on, the shape those spread into across the weeks, and the things that go wrong. A World says *who*; its Calendar says *how much, when, and what breaks*. Every quantity in it is a parameter the harness accepts rather than a constant, which is what makes a Run repeatable at a different size, and every quantity is an **estimate** until a real Practice's book has been read — a Calendar sizes a Run and never describes an industry. Defined in [docs/simulation/calendar.md](docs/simulation/calendar.md). Decided on [#765](https://github.com/markgoho/doula-cloud/issues/765).
_Avoid_: Schedule (that is the jump schedule, one part of a Calendar), volume, workload, timeline

**Probe**:
A moment in a **Run** where two cast members deliberately act on the same object at the same simulated instant — the only place a Run is genuinely simultaneous rather than interleaved. Each one is named, placed on the **Calendar**, and states the object contended for, so it reads in both people's **Friction logs** as ordinary entries carrying one shared id. A Probe is walked once and what happened is recorded; repeating it to see whether it flakes, or running many at once, is load testing and is out of scope. Decided on [#765](https://github.com/markgoho/doula-cloud/issues/765).
_Avoid_: Race, stress test, concurrency test, load probe (all four imply repetition or synthetic pressure; a Probe is two people and one object)

**Intrinsic layout**:
Layout resolved from the content and the space available to it, rather than selected by the author from a set of named widths. The anchor term for every layout entry below: a component is laid out intrinsically when it is correct wherever it is placed — a full page, a narrow column, an embedded surface — without being told which of those it is in. See [ADR-0024](docs/adr/0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md) and [ADR-0025](docs/adr/0025-layout-is-verified-across-the-continuum.md).
_Avoid_: Responsive design (in the common sense — see **Responsive**), adaptive design, mobile-first, breakpoint-driven

**Available space**:
The room a component is actually given by its parent. The only measurement an intrinsic layout reacts to, and the reason a component never asks how large the window is: a component in a sidebar has little available space on the widest screen there is.
_Avoid_: Viewport width, screen size, device width, mobile, desktop

**Content floor**:
The available space below which a named thing stops fitting — a two-line address, a four-figure amount, a full practice name. **The only layout number an author writes**, and a last resort: a configuration change the browser can already resolve from a component's own quantities — a rail's own width, a column's own cap, a sibling's own minimum — needs no floor, and one is written only where no such quantity exists to express the change. It is discovered from the content, never chosen from a set, and it lives where it is used. It is a **fixed point**, not a range: the lowest available space at which the wide configuration is still acceptable, so it carries no margin of its own — a floor with room to spare is a floor chosen, not measured. It is measured with emergency wrapping neutralized, since a rescue such as breaking a word mid-line changes what happens after content stops fitting, not when it stops fitting, and a floor measured through one is a property of the rescue rather than of the content. It is also measured in one named environment: the same text rasterizes to a different width on a different platform, so "the smallest space this still fits in" is not one number until a platform is chosen, and everywhere else is asked only whether that number still gives the content enough room. Ordinary wrapping is a different question, and belongs to what "still acceptable" means rather than to how a floor is measured: a short, author-controlled string — a label, a button's name — wrapping onto another line is itself the content no longer fitting, while a long value is a Practice's own data of arbitrary length, and wrapping it is the correct outcome rather than the failure a floor watches for. Everything downstream of it — the points at which a layout changes configuration — is resolved by the browser, is plural, and is deliberately left unnamed: a moment you can name is a moment you can list, and a list of them is a set of widths. A `@container` condition's literal is a content floor too, which is what stops a container query from hiding a device width. **320px is not a content floor.** It is a conformance commitment derived from WCAG 1.4.10 at 400% zoom, and no component may derive its floor from it.
_Avoid_: Breakpoint (in every form, including "implicit breakpoint" — it names a moment, and an author looking for a moment has stopped looking for a floor), min-width, threshold

**The continuum**:
The full unbroken range of available space a component must be correct across, as against a set of points within it. The word exists so that "a set of widths" is audibly wrong: the continuum is not 320, 480 and 768.
_Avoid_: Width matrix, size range, breakpoint set

**Quantum layout**:
Every Layout's term for the goal state: a layout that exists in several configurations at once, with the browser resolving which one appears from the space available — the author writes none of them as a choice. Its opposite is a component with exactly one configuration at every available space, which is the ordinary failure. Useful because it makes that failure sayable in a sentence.
_Avoid_: Variant, layout mode, view

**Responsive**:
Adapting to a **stated user preference** — color scheme, reduced motion, contrast, print — and never to available space. Deliberately narrower than the common industry meaning, which is a layout chosen from named device widths; that is the thing this repo does not do. Permitted when naming someone else's artifact, such as the ONS responsive table.
_Avoid_: Using it for anything space-related — that is **intrinsic layout**

**The continuum check**:
The automated pass asserting that nothing needs more room than it is given, at any available space, naming no width. Covers both a component, through the style guide's own demo registry, and a route, through the route's **fixture**. It is one artifact with **the drag surface**: every subject the check sweeps can be watched on the surface, and both read the same **fixture**, so a screen is described once and reported under one name.
_Avoid_: Width matrix, size sweep, responsive test

**The drag surface**:
The style-guide surface with a handle a person drags, so a subject can be watched passing through its configurations continuously rather than inspected at chosen sizes. The human half of **the continuum check**, over everything it covers — a component through its style-guide page, a route through the `page.fixture.ts` beside it. A route reads more than props, so the surface hands it the fixture's own parameters and answers its requests from the fixture too; what a person drags is what the check sweeps.
_Avoid_: Width matrix, size panel, breakpoint preview

**Fixture**:
The content a surface is measured with. It holds the longest realistic value for every field a Practice types into itself — never a representative one, because a surface measured on polite content is a surface nobody will ever see. A component's fixture is its style-guide page; a route's is the `page.fixture.ts` beside it, which also names the route's parameters and answers its requests, because a route reads more than props. Its row set must also realize every state a field renders differently, not only its longest value — one row at every field's busiest state, one at every field's emptiest, a third only where a field's own render branches a third way (`.claude/rules/svelte-tests.md`, ADR-0025).
_Avoid_: Mock data, sample data, dummy content

**Fluid step**:
A design token whose value is a `clamp()` between a floor and a ceiling, growing continuously with the space its consumer is given rather than holding one number everywhere. Every type size and every spacing step is one. The growth term is a container unit (`cqi`), never a viewport unit, so a fluid step answers **available space** like everything else here — the same token is smaller in a rail than on a full page, by design. Its floor and ceiling are `rem`, so a person's own font-size setting still moves it (WCAG 1.4.4).
_Avoid_: Responsive type, fluid typography (the industry term is viewport-derived and this is not), t-shirt size

**The ramp**:
The span of available space across which a **fluid step** grows, 320px to 1920px, shared by the type scale and the spacing scale so the two cannot drift apart. Outside it a step is pinned: below 320px to its floor, above 1920px to its ceiling. The upper end is a plateau on purpose — past it, more room buys more content, not larger letters. It is a property of the scale, not of any component, and it is the one place in this repo where two widths are written down.
_Avoid_: Breakpoint range, min/max viewport, screen sizes

**Growth factor**:
How much a **fluid step** grows from floor to ceiling, expressed as a multiple: 1.2 for type, 1.5 for spacing. It is chosen once for a whole scale rather than per step, so the relationships inside the scale hold at both ends of **the ramp**. Its ceiling is 2.5, which is what keeps text **able to reach** 200% within the 500% a browser zooms to: at deep zoom every step sits on its `rem` floor, so reaching twice a value that may be as large as the ceiling needs a zoom of 2 × the growth factor, and 2 × 2.5 = 5. WCAG 1.4.4 asks that 200% be reachable, not that the browser's 200% setting render it — a fluid step renders 1.6× to 2.0× there, and the repo commits to reachability plus never shrinking, not to the detent.
_Avoid_: Scale ratio, modular ratio (those name the relationship between neighboring steps, which is a different number this repo does not use)

**Containment context**:
An element that declares itself the thing its descendants measure **available space** against, so a container query and a container unit inside it answer that element rather than the page. `body` is one, which is what stops a **fluid step** meaning "the window" in the components that sit outside every other one. It is not a layout of its own and it names no width; it only decides which box the question is asked of.
_Avoid_: Query root, breakpoint scope, responsive container

**The pairing**:
The rule that a **containment context** declares the base size on its children, never on itself. A container unit resolves against the nearest _ancestor_ container, so an element that declares a container never resolves against its own; and `font-size` inherits as a computed length rather than as the token, so text that merely inherits carries whatever size was computed _outside_ the container it now sits in. One declaration on the container's children re-resolves the token there, and everything below inherits a length that answers **available space**. Every containment context in the repo owes it, and a source gate holds them to it. Its absence is invisible on a narrow screen, where the outside and the inside are nearly the same number, and opens up on a wide one.
_Avoid_: Font-size reset, cascade fix, inherit override
