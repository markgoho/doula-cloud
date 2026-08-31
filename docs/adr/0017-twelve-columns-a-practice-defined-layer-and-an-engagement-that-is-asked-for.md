# Twelve columns, a Practice-defined layer, and an Engagement that is asked for

A Client had a name and an email. This document gives her a record, gives a Practice a way to add
the facts we did not think of, gives a returning Client a way to be found rather than retyped, and
splits the one act that created her from the act that starts paid work.

It was chartered on the wayfinder map
[Client intake: a real client record, and reusing an existing Client for a new Engagement](https://github.com/markgoho/doula-cloud/issues/332),
and collects six closed decisions —
[#369](https://github.com/markgoho/doula-cloud/issues/369),
[#370](https://github.com/markgoho/doula-cloud/issues/370),
[#371](https://github.com/markgoho/doula-cloud/issues/371),
[#373](https://github.com/markgoho/doula-cloud/issues/373),
[#374](https://github.com/markgoho/doula-cloud/issues/374),
[#393](https://github.com/markgoho/doula-cloud/issues/393) — into one buildable model. Nothing in the
model sections is decided here; each is traceable to its ticket. Where a later ticket reversed an
earlier one, this document says so rather than presenting the result as if it arrived in one pass.

It **departs from [ADR-0001](0001-practice-defined-plan-templates.md)** on one point, deliberately,
and the departure is stated in full below: a Plan Instance snapshots the field definitions it was
created against; a Client's Practice-defined values are read **live**. The two patterns sit next to
each other and look alike, so the difference is not left to be inferred.

It **completes [ADR-0015](0015-three-facts-on-an-engagement-the-person-lives-in-the-login.md)'s
handoff** on intake — `clients.practice_id`, lookup-before-insert, and the name rule — and preserves
that document's sentence *a Credit locks when an Engagement is created* by making creation the
approval rather than by amending the sentence.

Read rules throughout are stated against
[ADR-0008](0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md), never
ADR-0006, which is superseded in full.

## The model, in one pass

A **Client** is one Practice's record of a person. Her record has two layers:

- A **structural core** of twelve columns, the same at every Practice, which the product itself
  reads — for an invoice, a Contract merge field, an Offer, a search.
- A **Practice-defined layer**: fields a Practice adds for itself, from ADR-0001's same six field
  types, which the product stores and displays and never interprets.

A Practice-defined field **may never restate or shadow a structural fact.** A Practice asking for a
middle name is not asking for a custom field; it is telling us the structural `name` has the wrong
shape.

A Client is **found before she is created.** Intake begins with a search, and the search is the only
door to it. Saving her is **free** and starts nothing.

An **Engagement Request** is the ask for paid work with her. Approval creates the Engagement and
locks the Credit. Until approval there is no Engagement anywhere in the system.

Every change to her record is one row in **`client_events`**, holding a diff of both sides, and the
actor is always named.

## The two layers, and the departure from ADR-0001

The field list is **per-Practice**, above the fixed core. Not fixed, because we cannot know in
advance what a Practice needs to hold about a woman; not per-Doula, because that contradicts the
settled rule that a Client is *one Practice's* record — if two Doulas at one Practice hold different
facts about the same woman, the record stops being the Practice's ([#369](https://github.com/markgoho/doula-cloud/issues/369)).

A **Client Field Template** belongs to a Practice, one per Practice, drawn from ADR-0001's same six
field types (short text, long text, single-select, multi-select, checkbox, section header). It is
**empty by default** — a new Practice is not handed guesses about its own practice — and an Owner or
an Admin edits it. Removing a field **archives** it rather than deleting it, so a Client who already
holds a value does not silently lose it.

### The departure

ADR-0001 snapshots. Each Plan Instance stores the field definitions it was created against, so
editing a template never rewrites a Client's completed Birth Plan.

**A Client's Practice-defined values are read live.** The Practice's current template is the one
rendered, always.

The two are opposite because the things are opposite. A Plan Instance is a **document**: it was
filled in on a date, it was agreed to, and changing what it says afterwards is falsification. A
Client record is a **live description of a person**: her address today is the one we want, and a
record that renders against a template from four years ago is a record nobody can correct.

The consequence is that archived-not-deleted carries real weight here, where in ADR-0001 the
snapshot would have covered it. An archived field's stored values survive and the Client detail page
still shows them, labelled and marked as no longer collected.

### One layer, or two, on the Engagement

Practice-defined fields are **Client-only** for now. Duplicating the whole mechanism onto the
Engagement buys a case nobody in the pilot has stated, and it is additive later. It becomes worth
building the first time the pilot asks for a per-pregnancy fact — hospital of choice, referral
source — that is wrong to hang on the person rather than on the piece of work.

## The twelve structural columns

`practice_id` — a Client belongs to exactly one Practice. This is the column ADR-0015 specified and
nothing implemented, and it is what makes lookup-before-insert possible at all: without it,
`clients_select` reaches a row only through an Engagement at the current Practice, so Practice B can
never find the row Practice A created and always inserts a fresh one.

`given_name`, `family_name`, `preferred_name` — the name **splits into three**, and only
`given_name` is required. Three, not one, because four system paths read a Client's name and they
divide cleanly into **document** and **conversation**:

| Path | Reads |
| --- | --- |
| Stripe invoicing | the legal name |
| The `client_name` Contract merge field | the legal name |
| Every screen, and the Clients sort | the preferred name |
| The Message thread | the preferred name |

Only `given_name` is required because the first thing a Doula has, after a phone call, is a first
name — and a form that refuses to save until it has a surname is a form that loses the record. An
Offer needs a first initial in isolation, which is a fifth reason the name cannot stay one column.

`email`, **nullable**. This is the sharpest reversal in the collected set: `clients.email` was `NOT
NULL` and it is relaxed. A Practice is given a woman by a hospital referral with a phone number and
no address; there is no honest value to type. Two paths must therefore **refuse** rather than send to
an empty string — the portal invite outbox and Stripe invoicing — and both refusals are named in the
handoff below.

`phone`.

`address_line1`, `address_line2`, `address_locality`, `address_region`, `address_postal_code` — a
**structured** address, not one text blob, because `address_locality` is a fact the product reads:
an Offer's `client_area` derives from it instead of being retyped by an Admin on every send.

`date_of_birth`, nullable — and this column **moved**. [#370](https://github.com/markgoho/doula-cloud/issues/370)
put date of birth in the Practice-defined layer, where it looked like an ordinary
nice-to-have. [#371](https://github.com/markgoho/doula-cloud/issues/371) moved it into the
structural core and reversed that, for a reason #370 could not have seen: the match query for a
returning Client needs it, ADR-0001's field-type palette has **no date type**, and a free-text date
cannot be matched. A Practice-defined field the product must interpret is a contradiction, so the
fact had to change layers. It is recorded as a move rather than smoothed over, because a reader who
finds date of birth sitting among the structural columns will otherwise wonder why a fact this
optional is not just another custom field.

That is twelve. The **intake note**, **pronouns** and an **emergency contact** stayed
Practice-defined: the product reads none of them, and each is a fact some Practices keep and others
do not.

### What the core does not assert

Checked against a postpartum-only Engagement and against a loss: **nothing structural asserts she is
pregnant.** The due date is not here — [#353](https://github.com/markgoho/doula-cloud/issues/353)
moved it onto the Engagement, nullable, where it belongs to one piece of work rather than to a
person. A woman who has two Engagements four years apart has two due dates and one record.

## Who reads a Client

**One new row on ADR-0008's read table, for the whole record — both layers.**

| | Owner | Admin | Doula (employee) | Doula (contractor) | Doula — offered, not accepted |
| --- | --- | --- | --- | --- | --- |
| Client record — all twelve columns and every Practice-defined value | ✓ | ✓ | ✓ | on her attached Clients | ✗ |

One row rather than a rule per field, and this is a decision rather than an omission. A per-field
audience setting would be a real feature — a permission surface, a settings screen, a migration —
bought for a case nobody has stated. Every field on a Client is **staff-only**, so the row is
uniform and inherits ADR-0008's existing enforcement unchanged.

The **contractor cell reads the home address**, which looks generous next to ADR-0008's protection
of an Offer's thinness. It is correct: a contractor attached to an Engagement is going to that
woman's house. The thinness protection lives in the **✗ cell** — a Doula who has been offered work
and not accepted it reads nothing of the Client, and sees only what the Practice wrote into the
Offer: a first initial, a general area, a due date, a fee, free-text terms. That cell, not a
narrower address rule, is what stops an Offer from leaking a woman's identity to a stranger.

**Nothing of a Client record is shown in the portal for v1.** The Client sees none of her own
record. She has no way to correct it, and Messages already carries *I moved*. This is recorded as a
**v1 decision** rather than as a settled fact about the model: it reopens the moment she is given a
screen for her own contact details. [#374](https://github.com/markgoho/doula-cloud/issues/374) went
looking for the correction path that would have forced it open sooner and found the opposite — her
portal **login** was never `clients.email`, and is already hers (see below) — so the ruling stands.

## Finding a returning Client

**Search is the first screen of intake, and the only door to it.** There is no top-level *Add a
Client* action. The route is *Clients → Find or add a Client → search → her record → Request
Engagement start* ([#539](https://github.com/markgoho/doula-cloud/issues/539) relabelled the
Clients list's link, which had read only *Add a Client* even though it served both errands), and
the empty state carries whatever was typed into a new record, so searching costs a staff member
nothing when the woman is genuinely new.

The match query runs on **name, date of birth, email and phone, within `clients.practice_id`**. It
is a within-Practice lookup and that is the settled rule, not a limitation to work around: a Client
is one Practice's record of a person, and the person lives in her Portal Account. Cross-Practice
recognition was ruled out on [#351](https://github.com/markgoho/doula-cloud/issues/351) — an email
lookup that answers *yes* tells an agency that a named woman is already another agency's client.

**The same query runs again at save.** Search is a habit and habits are skipped; the check at write
time is the one that cannot be. Any hit stops the write and forces a choice:

- **This is her.** Her existing record is kept, and what was just typed applies as a recorded
  **edit** — listed before it is applied, so nobody overwrites an address by accident.
- **No, a different person.** The write proceeds. This is now the only way a duplicate can be made.

The prompt prints **name and history unrestricted** inside the Practice. There is nothing to protect
here: the Clients list already names everyone, so the lookup leaks nothing a staff member could not
read one click away.

**A second live Engagement warns, never refuses.** A birth package's few postpartum visits do not
stop the same woman buying a whole postpartum package.

## What an edit records, and what an edit may not do

### One row per edit, holding a diff

`client_events`: one row per act, carrying a **JSONB diff of both sides**. Not the column-pair shape
of `practice_membership_events` or ADR-0015's `engagement_events` — those hold four facts and six,
and this record holds twelve structural columns plus an open set of Practice-defined values that
grows whenever an Admin adds a field. A column pair per fact does not reach that, so the **act
becomes the row** and the diff holds only what changed.

Structural fields are keyed on their column name. Practice-defined fields are keyed on their **field
id**, with the label **as it read at the time** stored alongside — so an event stays legible after
the field is renamed or archived.

Two event types: `created` as well as `updated`, so her history starts at intake rather than at the
first correction.

**The actor is always named.** `actor_kind` is `staff` or `system`, with a `CHECK` binding it to
`actor_staff_id`, so no row ever reads as *we do not know who did this*. The Client editing her own
record is excluded for v1 by the portal ruling above, and lands as a third value when she is not.

Read rule: the **same single ADR-0008 row** as the record it describes. Practice-tier RLS on its own
`practice_id`, never reachable from a portal session, and **append-only** — `GRANT SELECT, INSERT`,
no `UPDATE`, no `DELETE`. This supersedes an incidental phrase in
[#371](https://github.com/markgoho/doula-cloud/issues/371) — *viewable by an Owner or Admin* — which
was written before the read rule was decided.

**Changes to the field list are audited too, in their own table**, because *why does this Client hold
a value in a field that is not on the form?* has no other answer. They are not `client_events` rows:
they audit a Practice's settings, not a Client.

### The name rule is blocked at the act

A Client's name may be **corrected** and may not be **substituted**. Fixing *Sara* to *Sarah* is a
spelling fix; changing *Sarah Beck* to *Nadia Haddad* is pointing an existing record, its Engagement
history and its Messages, at a different human being.

**Prevention is impossible and detection is not**, because a spelling fix and a substitution are the
same `UPDATE` statement. So the rule is enforced where it can be: the match query from #371 re-runs
on every edit, and **the edit does not save** if the result would match a different Client. One
deliberate override exists — *No, a different person* — and there is deliberately **no** *this is
her* click on the edit path, because merging two records is out of scope.

The general rule this instance belongs to, which the build should apply elsewhere:

> **Block when the flagged state is a mistake with a correct alternative. Warn when it is legitimate
> and common.**

That is why the same map both blocks a name substitution and *warns, never refuses* on a second
concurrent Engagement. It is also why a second **pending** Engagement Request for the same Client
*and the same kind* is blocked outright by a unique index, while a second live **Engagement** is
not — and why that index is keyed on the kind rather than on the Client alone, so the woman buying
both a birth package and a postpartum package is never caught by a rule aimed at a duplicate.

**No delete and no deactivate on a Client.** An Engagement is a permanent record, and the real need
behind *remove her* is `entered_in_error` on the Engagement, which belongs with the ADR-0015
remainder. The one case that reasoning did not reach — a Client whose Request was refused, who has
no Engagement to mark — is answered under the Request below, and the answer is still nothing new.

## Who may write

Four writes, not three. Saving a Client became free, so **create** and **request** separated.

| | Owner | Admin | Doula (employee) | Doula (contractor) |
| --- | --- | --- | --- | --- |
| Create a Client | ✓ | ✓ | ✓ | **✗** |
| Edit a Client | ✓ | ✓ | ✓ | on her attached Clients |
| Request an Engagement start | ✓ (starts directly) | ✓ (starts directly) | ✓ | **✗** |
| Approve a Request | ✓ | ✓ | ✗ | ✗ |

**Edit follows read.** Whoever may read a Client may edit her, the contractor included on her
attached Clients. She is at that woman's house; she is the person who learns the address changed.

### A contractor originates nothing

A contractor Doula cannot create a Client at a Practice she contracts for, and cannot request an
Engagement there. Both fall out of one sentence: **a Client she typed into someone else's Practice
would be a record of a woman who was never theirs.** Work at that Practice reaches her as an
**Offer**, and the Offer is already the signal — which is also why she does not request even for a
Client she is attached to.

What she actually needs is her **own Practice**. Her *Add a Client* screen is therefore **a door
that only explains**: work here arrives as an Offer, and here is how to set up a Practice of your
own. It links to plain `/signup` as a stopgap. The real route — a trial, Stripe Connect sequencing,
what the Practice she contracts for learns, which Practice she lands in when she signs in — is an
acquisition decision with its own tree, chartered as
[#395](https://github.com/markgoho/doula-cloud/issues/395).

**Her route to a Client she is attached to is the Clients list itself, not a search.**
[#539](https://github.com/markgoho/doula-cloud/issues/539) settled this: `ListHandler` already
narrows a contractor's Clients list to her own attachments (the read-table row above), so she needs
neither *Find or add a Client* nor a search to reach one — the list she already sees on landing is
that route, in full. The Clients list therefore hides *Find or add a Client* from her outright
rather than relabelling it, mirroring the same load-time gate `clients/search/+page.ts` uses for
#501's door. Hiding the control removes the explain-only door's only link for her, so an empty
Clients list gives it back: the empty state names why (work reaches her as an Offer) and links to
that door.

### Two corrections to things the repo believed

**An Admin may buy Credits.** `api/internal/billing/purchase.go:33` goes `RequireOwner` to
`RequireOwnerOrAdmin`, and `CONTEXT.md`'s *only an Owner buys them* was wrong. An Admin who may
approve a Request, and who already reads the balance and the ledger, must be able to top up the
balance she approves against.

**`clients.email` and a Client's login were never the same field.**
`portal/accept-invite/+page.svelte:23,43` lets her type **any** address into
`createUserWithEmailAndPassword`, and accept keys on `identity_uid`. So `clients.email` is *the
address the Practice reaches her at*, and stays staff-editable forever; her login is hers already,
and this document does not touch it. That is what removed the correction path #374 went looking
for, and it is why the portal ruling above survived.

**Editing that address revokes any pending portal invite.** `api/internal/portalinvite/outbox.go:69`
reads the address **live at send**, so an unsent invite would otherwise mail a live token to
whatever was just typed.

### Enforced at both seams

Every rule that a policy can express is enforced **both** at the endpoint and in RLS.
`app.current_staff_id` is already set on every staff request, so a policy can see the caller's
Membership and its employment type — there is no excuse for a role gate that lives only in Go. Two
rules cannot be expressed as a policy and are enforced at the endpoint alone: the match-query
refusal, and the invite revoke.

A `completed` Engagement does **not** freeze a Client's record. Her address still changes after her
care ends, and editing her record can never manufacture an Engagement, because the Credit locks at
approval regardless.

## The Engagement Request

**An Engagement Request is its own entity.** It is not a status on `engagements`, and this document
adds no `engagement_status` value, changes no transition, and touches no existing status.

ADR-0015 had already ruled the neighbouring case: *a Credit locks when an Engagement is created*,
and *if the pilot wants a pipeline it arrives as a new entity with its own decision, never as a
status value.* Splitting intake into a free save and a paid start came afterwards, so this document
either preserved that sentence or amended it. A separate record preserves it **literally** —
creation is now the approval. A `requested` status would have broken it twice: the Credit would no
longer lock at creation, and `intake` carries a fixed Client-portal label under ADR-0005 (*Getting
started*) that a woman would read for work nobody has agreed to pay for yet.

The shape is already in the repo twice — `engagement_offers` and `practice_invitations` are both
thin decision records with their own state enum, beside the thing they eventually grant. A Request
carries six facts and one decision; it duplicates none of the Engagement's status, birth outcome,
ending reason or freeze rule.

**The structural payoff:** Messages, Contracts and Visits all hang off an `engagement_id`. While a
Request is pending that id does not exist, so a pending Request enables nothing **by shape** rather
than by a gate someone has to remember to write, and no half-live Engagement sits in front of the
Clients list, the portal, or the completion cascade.

### The requester describes the work; the approver does not amend it

The Doula states the **kind** (`birth` or `postpartum`) and the due date as part of the ask. She did
the intake call and is the only person who has heard either fact. This is where
[#308](https://github.com/markgoho/doula-cloud/issues/308)'s create-time kind control lives: on the
intake screen, not on the approval screen.

**The approver approves or refuses exactly what was described.** Amending the kind or the due date
before approving would make *approved* ambiguous — the record would say she agreed to a request
whose text no longer matches the ask — and it duplicates an edit path ADR-0015 already provides on
the Engagement itself. One act, one meaning.

### What the approver reads

Which Client, and whether she is new or already known; which Doula asked, and when; the kind and due
date; the Credit cost and the **balance after**; the requester's note; and the Client's **existing
Engagements, past and present**. The history is there because a returning Client is the case this
whole map exists for, and #371 already ruled that name and history print unrestricted inside the
Practice.

The second-live-Engagement warning appears in **both** seats: at request time so the Doula can
reconsider before stopping her own work for a wait, and at approval time because the approver is
spending the Credit and may know something the Doula does not.

### While a Request is pending

The Client's record shows a pending-request block naming who asked and when, and the actions that
need an Engagement are **not drawn at all** — an action that cannot work does not belong on the
page, and four dead buttons discovered one at a time is a trap. The pending state also shows in the
Clients list.

**A requester may withdraw her own pending Request, and a Request never expires.** Withdraw exists
because the alternative route out of a typo is asking an Admin to refuse it, which stamps a false
*refused* on a woman's permanent record. Expiry was refused on the ground that separates a Request
from an Offer or an Invitation: those expire because they are bearer secrets sitting in an inbox. A
Request holds no secret and reaches nobody outside the Practice, so expiring it would only delete a
real ask a busy Admin has not got to yet.

### Refusal, and the Client it leaves behind

Refusal is a **recorded state with a required reason**. Not a deletion, which leaves the Doula
unable to tell a refusal from an Admin who has not looked; not silence, which is indistinguishable
from a forgotten ask. The reason is required rather than optional because a bare *refused* generates
exactly the corridor conversation this record exists to prevent.

**Nothing new is built for the Client a refusal leaves behind.** No archive, no deactivate, no
delete. Archive would invent the concept the edit rules above deliberately refused, with its own
audit rows, its own permission and a rule for un-archiving. Delete is how the same woman gets typed
in twice next year, which is the failure this map exists to end — she must stay in the intake
search, and that is the point rather than a side effect.

What is wrong is the **list**, not the record. The Clients list gains a default filter — *Clients
with work* — and a way to see everyone. That list was already being rebuilt: `engagement/list.go:74`
returns one row per Client+Engagement pair, so a Client with two Engagements appears twice and the
screen has to become Client-shaped regardless.

### Credits

**No Credit is reserved when a Request is made.** Reserving prices a Credit at the moment of asking,
which is the *pay for every enquiry that went nowhere* model ADR-0015 ruled out; and with three
Doulas holding Requests against two Credits, the first two asks would lock what the third cannot
have. The balance is true only at the moment of approval.

**Approval into an empty balance fails without consuming the Request.** The Request stays pending,
`billing.ErrNoCreditsRemaining` and its out-of-Credits Notification are reused unchanged, and the
approver's screen offers *Buy Credits* inline and returns her to the same approval. Nothing the
Doula typed is lost, and the balance-after figure normally shows the problem before the click.

### The audit trail is the Request row

`client_events` carries `created` and `updated`; ADR-0015's `engagement_event_type` has no `created`
value. So nothing in either table records *this Engagement began because she asked and he agreed* —
and the Request row is where that fact lives, once, as the no-fact-written-twice rule requires.

A Request is asked once and answered once, so four terminal states fit in columns rather than in an
events table. There is **no `engagement_request_events` table** — that is the shape a thing needs
when it changes repeatedly over years — and **no mirrored `client_events` row**. The Client detail
page merges her `client_events` rows with her Request rows into one history, which is a read
concern, not a storage one.

### The solo Practice

Where the requester already holds approval authority, the Request and the approval are **one act** —
one rule, rather than a special case for a Practice of one. The row is still written, created and
decided in the same instant by the same person.

Under the alternative, *how did this Engagement come to be?* has two different answers depending on
who did it, and the Client detail page's history has a hole for every Engagement an Owner started —
at a solo Practice, all of them. One insert buys a uniform audit.

It does not read as approving oneself. An Owner's or Admin's button says **Start Engagement** and
its confirm step names the Credit cost and the balance after, which reads as a purchase
confirmation, because that is what it is. A Doula's button says **Request Engagement start**.

## Concrete schema

One goose migration, or a small ordered set. Nothing here exists in `api/db/migrations/` today. It
overlaps ADR-0015's specified migration and takes exactly the part intake writes; the remainder —
the status set, birth outcome, ending reason, the immutability trigger, `portal_accounts` and the
accept-time 409 — stays with that document and is not built here.

### `clients` — a Practice, twelve columns, and a values blob

```sql
ALTER TABLE clients ADD COLUMN practice_id uuid NOT NULL REFERENCES practices (id);

ALTER TABLE clients DROP COLUMN name;
ALTER TABLE clients ADD COLUMN given_name          text NOT NULL;
ALTER TABLE clients ADD COLUMN family_name         text;
ALTER TABLE clients ADD COLUMN preferred_name      text;
ALTER TABLE clients ALTER COLUMN email DROP NOT NULL;
ALTER TABLE clients ADD COLUMN phone               text;
ALTER TABLE clients ADD COLUMN address_line1       text;
ALTER TABLE clients ADD COLUMN address_line2       text;
ALTER TABLE clients ADD COLUMN address_locality    text;
ALTER TABLE clients ADD COLUMN address_region      text;
ALTER TABLE clients ADD COLUMN address_postal_code text;
ALTER TABLE clients ADD COLUMN date_of_birth       date;

-- Practice-defined values, keyed on field id. Shaped like
-- plan_instances.answers, but read against the Practice's *live*
-- template rather than a snapshot -- see the ADR-0001 departure above.
ALTER TABLE clients ADD COLUMN field_values jsonb NOT NULL DEFAULT '{}'::jsonb;
```

**Two policies collapse onto `practice_id`.** `clients_select`'s `EXISTS` subquery against
`engagements` becomes a plain column comparison, the same shape as
`engagements_practice_visibility`. `clients_insert` — the chicken-and-egg policy that existed only
because a Client had no Practice of her own at insert time — is **replaced, not deleted**: a new
`WITH CHECK` verifies the caller's Membership is not a contractor Doula's. A `clients` `UPDATE`
policy follows `clients_select`'s shape, since whoever may read may edit. `00026`'s two
`client_portal_users` policies that route through `engagements` to reach a Practice collapse the
same way.

There is **no `created_by` column**. It existed in an earlier draft only to let a contractor read a
Client she created, and she no longer creates one; `client_events`' `created` row already names who
did intake.

### `client_field_templates` — shaped like `plan_templates`

```sql
CREATE TABLE client_field_templates (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    fields      jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX client_field_templates_one_per_practice
    ON client_field_templates (practice_id);
```

One row per Practice, `fields` holding the ordered list with each entry's id, label, type and
archived flag. Empty by default — no seeded fields, unlike ADR-0001's seeded plan templates, because
we have no default guess about a Practice's own intake. Changes to `fields` are audited in their own
table, not in `client_events`.

### `client_events` — the act is the row

```sql
CREATE TYPE client_event_type  AS ENUM ('created', 'updated');
CREATE TYPE client_event_actor AS ENUM ('staff', 'system');

CREATE TABLE client_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id    uuid NOT NULL REFERENCES practices (id),
    client_id      uuid NOT NULL REFERENCES clients (id),
    event_type     client_event_type NOT NULL,
    diff           jsonb NOT NULL,
    actor_kind     client_event_actor NOT NULL,
    actor_staff_id uuid REFERENCES staff (id),
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT client_events_actor CHECK (
        (actor_kind = 'staff'  AND actor_staff_id IS NOT NULL)
        OR (actor_kind = 'system' AND actor_staff_id IS NULL)
    )
);

CREATE INDEX client_events_client ON client_events (practice_id, client_id, created_at);
CREATE INDEX client_events_diff   ON client_events USING gin (diff);

GRANT SELECT, INSERT ON client_events TO app_runtime;   -- no UPDATE, no DELETE
```

`diff` holds both sides of each changed fact: structural fields keyed on their column name,
Practice-defined ones on their field id with the label as it read at the time.

### `engagement_requests` — the ask

```sql
CREATE TYPE engagement_request_state AS ENUM
    ('pending', 'approved', 'refused', 'withdrawn');

CREATE TABLE engagement_requests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id   uuid NOT NULL REFERENCES practices (id),
    client_id     uuid NOT NULL REFERENCES clients (id),
    kind          engagement_kind NOT NULL,
    due_date      date,
    note          text,
    state         engagement_request_state NOT NULL DEFAULT 'pending',
    requested_by  uuid NOT NULL REFERENCES staff (id),
    requested_at  timestamptz NOT NULL DEFAULT now(),
    decided_by    uuid REFERENCES staff (id),
    decided_at    timestamptz,
    reason        text,
    engagement_id uuid REFERENCES engagements (id),
    CONSTRAINT engagement_requests_decision CHECK (
        (state =  'pending' AND decided_by IS NULL     AND decided_at IS NULL)
     OR (state <> 'pending' AND decided_by IS NOT NULL AND decided_at IS NOT NULL)
    ),
    CONSTRAINT engagement_requests_refusal_reason CHECK (
        state <> 'refused' OR reason IS NOT NULL
    ),
    CONSTRAINT engagement_requests_approval_engagement CHECK (
        (state = 'approved') = (engagement_id IS NOT NULL)
    )
);

-- At most one pending Request per Client *per kind*, so two Doulas
-- cannot both ask for the same woman's birth package and spend two
-- Credits on one piece of work -- while a Client buying a birth package
-- and a postpartum package at intake is still one visit to the screen.
-- Same partial-index idiom as engagement_offer_outbox_one_pending (00041).
CREATE UNIQUE INDEX engagement_requests_one_pending
    ON engagement_requests (client_id, kind)
    WHERE state = 'pending';

GRANT SELECT, INSERT, UPDATE ON engagement_requests TO app_runtime;   -- no DELETE
```

The index is keyed on `(client_id, kind)` rather than on `client_id` alone, and the difference is
the block-versus-warn rule applied carefully. Two Doulas asking for the same woman's birth package
is a mistake with a correct alternative — one of them should be looking at the other's pending
Request — so it is blocked. A Client buying **a birth package and a postpartum package at intake**
is the same legitimate, common pair that made a second live Engagement *warn, never refuse*, and a
`client_id`-only index would force her Doula to wait for one approval before asking for the other.
The narrower key blocks the duplicate and leaves the pair alone.

Practice-tier RLS on its own `practice_id`. `engagement_id` back-references the row approval created,
which lets the Client detail page say *this Engagement began with a request on 2 March* without a
second write. A `withdrawn` Request records `decided_by` as the requester herself, which is honest —
she decided it — and keeps the `CHECK` uniform.

### `engagements` — two of ADR-0015's columns, and no more

```sql
ALTER TABLE engagements ADD COLUMN kind     engagement_kind NOT NULL;
ALTER TABLE engagements ADD COLUMN due_date date;
```

`engagement_kind` is ADR-0015's enum, created here because this is the effort whose intake screen
writes it. Nothing else from ADR-0015's `engagements` section is built here.

`kind` is `NOT NULL` **with no database default**, exactly as ADR-0015 specifies it: a default would
be a second opinion about what the Practice sold, silently applied by whatever forgets to pass it,
and the intake control is the only thing that knows. This **departs from a charting note on
[#332](https://github.com/markgoho/doula-cloud/issues/332)** which called this map's slice *all
additive, all nullable* — that note was written before either document said otherwise, and #393 has
since made the requester state the kind on every path that creates an Engagement, so there is no
create path left that could produce a null. Existing rows are backfilled in the migration; there is
no production data, so this is fixtures only.

`due_date` is nullable, because a postpartum-only Engagement has none.

### `engagement_request_outbox` — the approver's Notification

The same shape as `staff_invite_outbox` (00038) and `engagement_offer_outbox` (00041), row for row:
a pending/sent/dead-lettered status, an attempt count, a `next_attempt_at`, a partial unique index
on one pending row per Request, no RLS, and the notification-worker read door on
`engagement_requests`. It mails **every Owner and every Admin at the Practice**, per ADR-0010 —
queued, never sent inside the request. There is no single approver: at a fourteen-doula agency the
authority is held by several people, and mailing one of them picked by some rule means a Request
waits on whichever person happens to be away. One outbox row per recipient, so the partial unique
index is keyed on `(request_id, staff_id)` rather than on the Request alone. A pending Request stops
a Doula from doing any work at all, so the wait is a stopped Doula rather than a background
inconvenience — and a collapsed request-and-approval mails nobody, because it was decided the
instant it was made.

## Rejected alternatives

**A per-Doula field list.** It contradicts the settled rule that a Client is *one Practice's* record
of a person: if two Doulas at one Practice hold different facts about the same woman, the record
stops being the Practice's. The case that motivates it — a contractor bringing her own intake
questions to every agency she works for — is her **own Practice's** field list, not a per-Doula list
inside someone else's.

**Snapshotting a Client's Practice-defined values**, the way ADR-0001 snapshots a Plan Instance.
Rejected because a Client record is a live description of a person, not a document that was agreed
to on a date. See the departure above.

**A per-field read audience.** A real feature — permission surface, settings screen, migration —
bought for a case nobody has stated. Every field on a Client is staff-only, so one ADR-0008 row does
the whole job.

**A `requested` value in `engagement_status`.** It breaks ADR-0015 twice over: the Credit stops
locking at creation, and `intake` carries a Client-facing label under ADR-0005 that would be shown
for work nobody has agreed to pay for. It also puts a not-yet-real Engagement in front of every
query that reads the table.

**Reserving a Credit when a Request is made.** It prices a Credit at the moment of asking, which is
the model ADR-0015 ruled out by name. Filed for a rainy day rather than refused forever: it returns
if a Practice ever reports Requests racing for a last Credit.

**Letting the approver amend the kind or due date before approving.** It makes *approved*
ambiguous, and duplicates an edit path ADR-0015 already gives the Engagement.

**Expiring an Engagement Request.** An Offer and an Invitation expire because they are bearer
secrets in an inbox. A Request holds no secret and never leaves the Practice, so expiry would only
delete a real ask a busy Admin has not reached.

**Archiving or deleting a Client with no Engagement.** Archive invents the deactivate concept this
document refused; delete is how the same woman is typed in twice next year. The list is filtered
instead.

**Merging two duplicate Client records.** There is no production data, so no duplicate exists to
merge outside fixtures, and once lookup-before-insert lands a duplicate takes a staff member
actively pressing *No, a different person*. Building merge now is building a repair tool for damage
the same release prevents.

**Cross-Practice recognition of a returning Client.** Ruled out on
[#351](https://github.com/markgoho/doula-cloud/issues/351): an email lookup that answers *yes* tells
an agency that a named woman is already another agency's client.

## Open axes — named, not decided here

- **What a Client sees of her own record.** Nothing, for v1. This is a v1 decision, not a fact about
  the model, and it reopens the moment she is given a screen for her own contact details.
- **Portal login itself** — alternatives to email and password, and whether a Client may change her
  own login. Surfaced while establishing that `clients.email` and her Identity Platform credential
  are two different fields. Nothing here waits on it: her login already works and is already hers.
- **Erasing a Client's data on request** — GDPR's right to erasure and its US state equivalents,
  filed as [#394](https://github.com/markgoho/doula-cloud/issues/394). Her data sits on nine tables
  and two external services, some under a legal obligation to keep, and the sharp question is
  whether erasure may break the append-only grant that `client_events`,
  `practice_membership_events` and `engagement_events` all share. The research underneath waits on a
  business fact: which jurisdictions bind the pilot Practices.
- **Practice-defined fields on an Engagement.** Additive later; revisit when the pilot asks for a
  per-pregnancy fact that is wrong to hang on the person.
- **Reserving a Credit at request time**, as above.
- **A third `client_event_actor` value** for the Client editing her own record, when she can.
- **Onboarding a contractor Doula into a Practice of her own** — chartered as
  [#395](https://github.com/markgoho/doula-cloud/issues/395). This document ships the explainer and
  a link to `/signup`.
- **Undoing an intake typed against the wrong person.** The record to undo is the Engagement and the
  Credit, not the Client, so it belongs with the ADR-0015 remainder where `entered_in_error` is
  already parked.

## What the build effort must charter

This document ends at a decision and a specified migration. What the build inherits, written down so
the handoff is not implicit:

**The migration**, as above.

**The BFF write surface.** Create with lookup-before-insert; edit, with the match query re-run and
the invite revoke; the Client detail read; the search; and the Request endpoints — request, approve,
refuse, withdraw. `api/internal/engagement/create.go` writes the Client and the Engagement in one
transaction and spends a Credit at the end; that splits into a **free** Client save and a separate
Request, with the Credit moving to approval.

**One Go helper computing the legal name and the preferred name**, so the document/conversation rule
lives in one place across `payments`, `contracts`, `message` and `engagement`.

**Two paths must refuse a Client with no email** rather than send to an empty string:
`api/internal/portalinvite/outbox.go:69` and `api/internal/payments/invoice.go:322`.

**The `client_name` Contract merge key stays exactly as it is** and resolves to the legal name, so
already-seeded templates keep working.

**The Offer form prefills `client_area` from `address_locality`** instead of an Admin retyping it on
every send — `api/internal/offer/create.go:117-122`.

**`api/internal/billing/purchase.go:33`** goes `RequireOwner` to `RequireOwnerOrAdmin`.

**The Clients list becomes Client-shaped**, with a default *Clients with work* filter.
`api/internal/engagement/list.go:74` returns one row per Client+Engagement pair today, so a Client
with two Engagements appears twice.

**The screens**: the search that fronts intake, intake itself, Client detail, Client edit, the Client
block on the Engagement page, the Client Field Template settings screen, the approval screen, and
the contractor's explain-only *Add a Client*. No portal screen among them.

**The portal due date** — the portal shows a created date; it shows the Engagement's due date.

**Two acceptance criteria elsewhere become wrong.**
[#252](https://github.com/markgoho/doula-cloud/issues/252)'s third criterion says the Client record
is *rendered on the Engagement page*; there is a Client detail page now, and the Engagement page
carries a block. Both #252 and several tickets on the map name **ADR-0006** in their read criteria;
substitute ADR-0008, which supersedes it in full.
