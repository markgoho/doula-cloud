# Employment type gates the Practice; Attachment gates the Engagement

Supersedes [ADR-0006](0006-read-follows-the-role.md)'s read table. ADR-0006 keeps
its own argument — why read-follows-write and one-practice-one-view both lost, and
why the Staff roster and Templates cells fall where they do — and gains a pointer to
this document. Only its table is superseded.

This ADR was chartered as ADR-0007 on the wayfinder map
[Who a Staff member is to a Practice, and which work is theirs](https://github.com/markgoho/doula-cloud/issues/225)
and its tickets. It is numbered 0008 because
[ADR-0007](0007-connect-account-state-is-two-capabilities-and-a-requirements-list.md)
was committed first, for the Stripe Connect account model, while this map's tickets
were still open. Every reference inside those tickets to "ADR-0007" means this
document.

It collects six closed decisions —
[#226](https://github.com/markgoho/doula-cloud/issues/226),
[#227](https://github.com/markgoho/doula-cloud/issues/227),
[#228](https://github.com/markgoho/doula-cloud/issues/228),
[#229](https://github.com/markgoho/doula-cloud/issues/229),
[#230](https://github.com/markgoho/doula-cloud/issues/230),
[#231](https://github.com/markgoho/doula-cloud/issues/231) — into one buildable
model. Nothing in the model sections below is decided here; each is traceable to its
ticket. The concrete schema fills gaps those tickets deliberately left for this
document, and is marked where it does.

## The model, in one pass

A **Staff** person is one row however many Practices they work at. What they are to
one Practice is a **Membership**: the roles they hold, and an **Employment type**
(`employee` or `contractor`) that is not a role — a role says what a person *does*,
employment type says what they *are to the business*. Joining is an **Invitation**,
mailed to an address, redeemed once, expiring, revocable.

Employment type governs one thing: whether a Doula's Membership carries an *ambient*
grant over the whole Practice, or nothing at all. An `employee` Doula reads and
writes every Engagement; a `contractor` reads and writes only what she is
**attached** to. Attachment is a separate fact from Membership — a row per
(Engagement, Doula) — and it has two origins: **accrued** (she did something herself)
and **granted** (someone decided she is on it). Only a granted attachment reaches;
an accrued one is a record of work, never a key. An **Offer** is how a contractor's
attachment is granted through her own agreement — she accepts or declines, and
acceptance mints the granted attachment. An employee may also be offered work, for
the claim rather than the reach: she already has the reach.

An Offer may reach someone who is not Staff at all yet, carrying an Invitation, so
one link joins her to the Practice and puts the job in front of her at once — and she
must be able to read enough to decide before she has an account.

## Concrete schema

Every table and enum below is new or changed by this model. `amount_cents`,
`gen_random_uuid()` defaults, and `timestamptz NOT NULL DEFAULT now()` audit columns
follow this repo's existing conventions (`00024_invoices.sql`, `00007_visit.sql`).
RLS on every new table with no `practice_id` column follows the `EXISTS` subquery
shape `visits_practice_visibility` already sets (`00007_visit.sql`).

### `staff` — one line changes

```sql
ALTER TABLE staff ALTER COLUMN identity_uid SET NOT NULL;
```

`UNIQUE` already held (`00002_practice_staff_tenancy.sql:21`) and was never the bug
([#226](https://github.com/markgoho/doula-cloud/issues/226)). It returns to
`NOT NULL` because the pending, identity-less row this nullability existed for no
longer exists once `practice_invitations` replaces it below.

### `practice_role` — the rename, independent of everything else

```sql
ALTER TYPE practice_role RENAME VALUE 'office_manager' TO 'admin';
```

Settled as a word only, while charting this map. `employment_type` living in its own
enum ([#227](https://github.com/markgoho/doula-cloud/issues/227)) means this
statement touches nothing else here and can migrate on its own schedule.

### `employment_type` — new enum, new column

```sql
CREATE TYPE employment_type AS ENUM ('employee', 'contractor');

ALTER TABLE practice_memberships
    ADD COLUMN employment_type employment_type NOT NULL;
```

Every Membership carries one, the founding Owner's included
([#227](https://github.com/markgoho/doula-cloud/issues/227)). `NOT NULL` is free:
[#226](https://github.com/markgoho/doula-cloud/issues/226) already removes the
half-formed membership this nullability would have needed to accommodate.

### `practice_invitations` — new table

```sql
CREATE TYPE invitation_status AS ENUM ('pending', 'accepted', 'revoked', 'expired');

CREATE TABLE practice_invitations (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id      uuid NOT NULL REFERENCES practices (id),
    address          text NOT NULL,
    roles            practice_role[] NOT NULL,
    employment_type  employment_type NOT NULL,
    token_digest     text NOT NULL,
    status           invitation_status NOT NULL DEFAULT 'pending',
    invited_by       uuid NOT NULL REFERENCES staff (id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    expires_at       timestamptz NOT NULL,
    revoked_by       uuid REFERENCES staff (id),
    revoked_at       timestamptz,
    accepted_staff_id uuid REFERENCES staff (id)
);
```

`token_digest` is a SHA-256 digest, following the precedent
`00028_sessions.sql:9-11` set for the same reason: a leaked read of this table hands
nobody a usable credential. Default expiry is 7 days
([#226](https://github.com/markgoho/doula-cloud/issues/226)). `roles` and
`employment_type` ride the Invitation, not a later `PATCH` — RA-G8's zero-role
membership stays abolished. `accepted_staff_id` is filled at accept, whether it
resolves to a brand-new `staff` row or an existing one who already has an account.

No `staff` row exists at invite time. `InviteHandler`'s current insert
(`api/internal/staffauth/invite.go:58`) and the three RLS policies it depends on
(`00004:42-85`) are replaced, not extended.

`authn.VerifiedToken` gains the email claim
([#226](https://github.com/markgoho/doula-cloud/issues/226)). It was deliberately
held to identity alone — "Identity Platform provides identity only -- no custom
claims" (`authn.go:229-236`) — and accept must compare the caller's verified address
against `practice_invitations.address`, so the interface widens on purpose. The
Firebase token already carries `email`; only the Go interface changes.

### `engagement_attachments` — new table

```sql
CREATE TYPE attachment_origin AS ENUM ('accrued', 'granted');

CREATE TABLE engagement_attachments (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id   uuid NOT NULL REFERENCES engagements (id),
    staff_id        uuid NOT NULL REFERENCES staff (id),
    origin          attachment_origin NOT NULL,
    attached_by     uuid NOT NULL REFERENCES staff (id),
    attached_at     timestamptz NOT NULL DEFAULT now(),
    ended_at        timestamptz,
    ended_by        uuid REFERENCES staff (id),
    fee_amount_cents bigint,
    fee_terms       text
);

CREATE UNIQUE INDEX engagement_attachments_one_open
    ON engagement_attachments (engagement_id, staff_id)
    WHERE ended_at IS NULL;
```

Shape settled by [#228](https://github.com/markgoho/doula-cloud/issues/228):
universal, several per Engagement, Doula-only, `origin` upgrades `accrued` →
`granted` and never the reverse, `attached_by` equals `staff_id` when accrued.
Ending is `ended_at`, never a delete, on four triggers — she drops out, the Practice
takes her off, the Engagement completes, or her Membership ends.

`fee_amount_cents` and `fee_terms` are this document's own addition: #228 left the
fee unplaced, pending the Offer ticket's answer to whether acceptance *creates* an
attachment or *is* one
([#228](https://github.com/markgoho/doula-cloud/issues/228)). #229
answered "creates", with the fee "copied onto the Attachment at acceptance, so
nothing can later rewrite what she agreed to." `CONTEXT.md`'s **Attachment** entry
already states this; the columns here are what makes it real. Both are `NULL` for an
accrued attachment and for any granted attachment opened without an Offer (an Admin
naming an employee on a Visit).

### `engagement_offers` — new table

```sql
CREATE TYPE offer_state AS ENUM
    ('offered', 'accepted', 'declined', 'withdrawn', 'superseded', 'expired');

CREATE TABLE engagement_offers (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id         uuid NOT NULL REFERENCES engagements (id),
    staff_id              uuid REFERENCES staff (id),
    invitation_id         uuid REFERENCES practice_invitations (id),
    employment_type       employment_type NOT NULL,
    amount_cents          bigint,
    terms                 text,
    client_first_initial  text NOT NULL,
    client_area           text NOT NULL,
    due_date              date NOT NULL,
    state                 offer_state NOT NULL DEFAULT 'offered',
    offered_by            uuid NOT NULL REFERENCES staff (id),
    offered_at            timestamptz NOT NULL DEFAULT now(),
    expires_at            timestamptz NOT NULL,
    decided_at            timestamptz,
    decided_by            uuid REFERENCES staff (id),
    access_code_digest    text,
    access_code_sent_at   timestamptz,

    CONSTRAINT offer_target_named
        CHECK (staff_id IS NOT NULL OR invitation_id IS NOT NULL),
    CONSTRAINT offer_fee_matches_employment_type
        CHECK (
            (employment_type = 'contractor' AND amount_cents IS NOT NULL)
            OR (employment_type = 'employee' AND amount_cents IS NULL)
        )
);

CREATE UNIQUE INDEX engagement_offers_one_open_per_staff
    ON engagement_offers (engagement_id, staff_id)
    WHERE state = 'offered' AND staff_id IS NOT NULL;

CREATE UNIQUE INDEX engagement_offers_one_open_per_invitation
    ON engagement_offers (engagement_id, invitation_id)
    WHERE state = 'offered' AND invitation_id IS NOT NULL;
```

The state set, the fan-out-with-first-yes-wins rule the two partial indexes only
half enforce (the accept handler owns the race — see Mechanism), the six-state
lifecycle, `expires_at` defaulting to 7 days, and completion closing every open Offer
as `withdrawn` are all [#229](https://github.com/markgoho/doula-cloud/issues/229).
The four decidable facts — Client first initial, general area, exact due date, fee —
and their exemption from Contract or Client joins (they are typed into the Offer at
send, a copy, not a view) are [#230](https://github.com/markgoho/doula-cloud/issues/230).
The Offer is *never* refreshed after it is sent; a changed fact means withdraw and
re-offer.

This document makes the following calls, not made in either ticket:

- **`staff_id` and `invitation_id` are both nullable, exactly one is required.** An
  Offer to an existing Doula membership sets `staff_id` at creation. An Offer to an
  email address sets `invitation_id` at creation and `staff_id` stays `NULL` until
  the Invitation is accepted and the accept handler back-fills it — the read and
  accept paths below never need to wait on that backfill, since both key off
  `invitation_id` until it happens.
- **`employment_type` is snapshotted onto the Offer**, not read live off the target's
  Membership (which may not exist yet for the email path). This is what lets the fee
  `CHECK` constraint run entirely inside the row, and it matches [#230](https://github.com/markgoho/doula-cloud/issues/230)'s
  "the Offer is a copy" principle rather than being an exception to it.
- **`due_date`, `client_area`, and `client_first_initial` are typed in by the
  sender at send time**, not derived from any Engagement or Client column.
  `engagements` carries no due date ([#229](https://github.com/markgoho/doula-cloud/issues/229)
  cites `00005_client_engagement.sql:11`), and [#230](https://github.com/markgoho/doula-cloud/issues/230)'s
  "as the Client gave it" is Renata transcribing what she was told, the same way
  `terms` is her own words. `client_first_initial` may be pre-filled from
  `clients.name` in the UI, but the column holds what was actually sent, immutably.
- **`decided_by` is `NULL` exactly once: the system-driven `withdrawn` on Engagement
  completion.** Every other terminal state — a real decline, a real withdrawal, a
  supersession the accept handler writes, an expiry job — names the staff member or
  the process that caused it, except completion's cascade, which by construction has
  no human actor at the moment it fires. This is the one row where `CLAUDE.md`'s
  audit-trail expectation is satisfied by "no actor" being the true, recorded answer
  rather than by a synthetic system account. A build session must not invent a
  system `staff` row to avoid a `NULL` here — that would misrecord a human action
  that did not happen.

  **Amended by the build ([#317](https://github.com/markgoho/doula-cloud/issues/317)):
  three states, not one.** The rule above survives; "exactly once" does not. Two
  more terminal states turn out to have no human actor either, and both are
  recorded the same way rather than given a fabricated one:

  - **A pre-account decline.** [#230](https://github.com/markgoho/doula-cloud/issues/230)'s
    reader has no `staff` row yet — that is the whole point of the token-plus-code
    read — so there is nobody to name. Declining must not require joining a
    Practice in order to say no to it, so the decline is recorded with
    `decided_at` set and `decided_by NULL`. The audit answer is the row itself:
    the Invitation it names and the moment it was decided identify the holder of
    a mailed token and a mailed code.
  - **Expiry.** This document's own text already granted "an expiry job" the
    alternative of naming *the process* rather than a person; with no such job
    built (expiry flips on the way past, at read and decision time, following
    `acceptInvite`'s precedent), there is no process to name either. `decided_at`
    records when it was found stale; `decided_by` stays `NULL`.

  What has not changed is the prohibition: no synthetic `staff` row, in any of the
  three cases.
- **`access_code_digest` and `access_code_sent_at` are populated only when
  `invitation_id IS NOT NULL`.** They hold the SHA-256 digest of the six-digit code
  [#230](https://github.com/markgoho/doula-cloud/issues/230) requires before the
  pre-account Offer read opens, mailed to the Invitation's address. An existing
  Staff member's Offer needs no code: she reads it through her session, and identity
  is already proven the ordinary way.

RLS on both new tables is a single `ALL`-command `EXISTS` policy against
`engagements`, the same shape `visits_practice_visibility` and
`engagement_attachments` use — no chicken-and-egg case, since both are always
created under an Engagement that already exists.

## The read table — five columns, superseding ADR-0006's four

Inside one Practice, RLS-fenced as ADR-0006 already established.

| | Owner | Admin | Doula (employee) | Doula (contractor) | Doula — offered, not accepted |
| --- | --- | --- | --- | --- | --- |
| Engagements, Visits, Messages | all | all | all at the Practice | only those she is attached to | ✗ |
| Plan Instances — Care Plan and Birth Plan | ✓ | ✓ | ✓ | on her Engagements | ✗ |
| Contract — scope (Visit counts, dates, on-call terms) | ✓ | ✓ | ✓ | on her Engagements | ✗ — whatever the Practice wrote into the Offer's `terms` |
| Contract — money, and Invoice history | ✓ | ✓ | ✗ | **her own agreed fee only, on her Engagements — never the Practice's price** | ✗ |
| Plan Template and Contract Template | ✓ | ✓ | ✓ | ✓ | ✗ |
| Credit balance and ledger | ✓ | ✓ | ✗ | ✗ | ✗ |
| Staff roster | ✓ | ✓ | ✗ | ✗ | ✗ |
| Her own Offer row | — | — | — | — | Client first initial, general area, exact due date, her fee, free-text terms |

Two changes from ADR-0006's table, both corrections recorded on
[#230](https://github.com/markgoho/doula-cloud/issues/230) rather than re-argued
here:

- **On-call terms are not a structured field anywhere in the model.** ADR-0006's own
  prose named them as something a contractor reads; they live in free-text `terms`
  on the Contract and on the Offer, never a schema field, because the product has
  never seen a structured shape for them.
- **The accepted contractor's Contract-money cell narrows.** ADR-0006 gave her "the
  Contract's money and Invoice history" because "she is negotiating it." The
  negotiation moved onto the Offer's `amount_cents`
  ([#229](https://github.com/markgoho/doula-cloud/issues/229)), so the rule is now
  general: a contractor reads the price *she* is paid, offered or accepted, and
  never what the Practice charges the Client. Two different facts that both look
  like "the price."

**The fifth column is additive, not a ceiling.** A person reads the union of what
each state she holds grants — employment type grants the ambient set or nothing,
each granted attachment grants its own Engagement, an open Offer grants exactly one
Offer row. A contractor who already holds other attachments, or who is already
Staff and reads Templates by virtue of being Staff at all, loses none of that by
also holding an open Offer. And a fifth-column ✗ is not a ceiling on an
**employee** holding an open Offer either — she already reads and writes
everything; the Offer settles her claim and nothing else
([#229](https://github.com/markgoho/doula-cloud/issues/229),
[#230](https://github.com/markgoho/doula-cloud/issues/230)).

## The write table — new content; ADR-0006 covered reads only

| | Doula (employee) | Doula (contractor) |
| --- | --- | --- |
| Engagements, Visits, Messages, Plan Instances, Contract actions | every Engagement at the Practice | only those she is attached to |

Settled on [#227](https://github.com/markgoho/doula-cloud/issues/227), opened as a
real disagreement with the user's initial position (attached-only for every Doula)
and closed in ADR-0006's favour on citation of its own 3am-coverage argument
(`0006:65`). **This default is provisional**, named on
[Questions for the pilot groups](https://github.com/markgoho/doula-cloud/issues/243)
as subject to confirmation once a pilot agency states whether cover involves
paperwork first. [#244](https://github.com/markgoho/doula-cloud/issues/244) is the
open ticket asking whether a Practice may switch this default to
attachment-required for everyone; this ADR's schema does not need to change for
that answer, since the mechanism already asks "does she hold a granted attachment"
and a Practice-level toggle only changes what the answer defaults to for an
`employee`.

## Enforcement mechanism

Chosen by prototyping two rival shapes against real handlers on
[#231](https://github.com/markgoho/doula-cloud/issues/231)
(branch [`prototype/231-read-gate`](https://github.com/markgoho/doula-cloud/tree/prototype/231-read-gate),
throwaway). **Both land, at different seams.**

**Mount seam — `GatedRouter`.** Every `GET` is mounted through
`router.Get(pattern, roles, handler)`, which panics at startup if `roles` is empty —
a forgotten declaration fails the binary before it serves a single request, not
silently at request time. An endpoint that is genuinely open to any Staff member
(none exist among today's rows, but the shape must allow one) declares `AnyStaff`
explicitly, so a table walk over the route registry can tell "declared open" apart
from "nobody declared anything." This is the `rlsguardrail`-shaped guardrail test
ADR-0006 asked for: one table-driven test walking `Routes()`, asserting every entry
carries a non-empty declaration — proved on the prototype against the real
`billing.GetBalanceHandler` (Owner 200, Doula 403).

**The token-authenticated Offer read is the one route outside `staffauth`
entirely** — [#230](https://github.com/markgoho/doula-cloud/issues/230)'s
pre-account read, authenticated by the Invitation's token and the access code rather
than a session. `GatedRouter` only reaches routes mounted behind
`staffauth.Middleware`, so this route cannot be silently ungated by omission the way
a forgotten role declaration is caught — it must be **declared exempt, by name**, in
the same registry the guardrail test walks, so the test sees a deliberate entry
rather than an absence. A build session must add this exemption in the same change
that mounts the route; mounting it anywhere the registry does not see is exactly the
failure mode this whole ticket exists to close.

**Query seam — `Reader` + typed views.** `staffauth.ResolveReader` is the only
constructor of an unexported-field `Reader`, so a query function that requires one
as a parameter cannot be called without having gone through role resolution.
`contracts.ReadContract(reader, full)` returns one of two Go types —
`ContractScope` or `ContractFull` — chosen by role, so an unredacted Contract record
never leaves the `contracts` package unshaped. This is the only mechanism that
solves the Contract's scope-without-money case, which `GatedRouter` alone cannot: a
whole-endpoint yes/no cannot make one handler return two shapes of the same record.
Proved on the prototype: Owner gets `ContractFull` with `price` reachable, Doula
gets `ContractScope` with no `price` key present at all.

`Reader` has no registry of its own — nothing stops a brand-new endpoint querying
the database directly instead of going through a `Reader`-gated accessor, and a
guardrail test cannot verify "only one exported read accessor per gated type"
without knowing every package's exports by name. `GatedRouter`'s startup panic is
therefore the backstop that actually closes a forgotten endpoint; `Reader` is what a
route reaches for once it is already inside the gate, whenever its response shape
varies by role. Contract is the only case known today.

**A finding, not a build detail.** Shape B's Contract split needs money-vs-scope
tagging on merge-field keys that does not exist anywhere today —
`extractMergeFields` only parses `{{key}}` placeholders with no schema-level
distinction between a money field and a scope field. The prototype hardcoded two
demo keys (`price`, `total_due`). A real build needs that tagging to live somewhere
real — a column on the Contract Template, or a naming convention the parser
enforces — before the query seam can ship for Contract. ADR-0006's table did not
anticipate this cost; it is new surface the read-gating mechanism itself creates.

**The write-side seam, from [#228](https://github.com/markgoho/doula-cloud/issues/228)
and [#231](https://github.com/markgoho/doula-cloud/issues/231).** The same
per-request facts a read gate needs — this request, this Engagement, this member's
roles — are what an attach decision needs, so attaching is not a second
hand-maintained list. A write endpoint under an Engagement attaches the acting
Doula by default, `accrued`, with `attached_by` equal to her own `staff_id`, and
**only** the acting Doula — an Owner or Admin acting on an Engagement is never
attached by it, and a Doula merely *named* in a write's payload (an Admin scheduling
her onto a Visit) is not the actor and does not accrue; that is a **granted**
attachment, written explicitly by the Visit-create and Offer-accept handlers outside
the seam. The seam must never mint `granted` by accident — that is the one rule
that keeps [#227](https://github.com/markgoho/doula-cloud/issues/227)'s no-backfill
promise and [#244](https://github.com/markgoho/doula-cloud/issues/244)'s future
toggle from being silently defeated by a Doula's first write.

## Rejected alternatives

- **A pending `staff` row kept at invite time, re-pointed on accept**
  ([#226](https://github.com/markgoho/doula-cloud/issues/226)). Smaller delta, but
  makes `staff` permanently hold two different things — a person and an invitation
  to a person who may not exist yet — and a contractor at three Practices makes that
  conflation normal rather than transitional.
- **Requiring Identity Platform's own address-verification** on top of delivery-is-
  the-proof ([#226](https://github.com/markgoho/doula-cloud/issues/226)). Proves the
  same fact delivery already proved and puts a dead-end round trip in front of every
  joiner.
- **A boolean `is_contractor` column** ([#227](https://github.com/markgoho/doula-cloud/issues/227)).
  Contradicts ADR-0006's own named values and forecloses a third employment type for
  no saving.
- **Employment type meaningful only where roles include Doula**
  ([#227](https://github.com/markgoho/doula-cloud/issues/227)). Needs a nullable
  column and a read rule for the null state — the same trap the zero-role
  membership was abolished to avoid.
- **An employee reads all Engagements but writes only where attached**
  ([#227](https://github.com/markgoho/doula-cloud/issues/227)). Defensible, and
  rescues the 3am-coverage case more elegantly than the flat default chosen — pushed
  instead to the per-Practice setting [#244](https://github.com/markgoho/doula-cloud/issues/244)
  may build.
- **Backfilling attachments from logged activity on an employment-type flip**
  ([#227](https://github.com/markgoho/doula-cloud/issues/227)). Invents a second,
  implicit origin for an attachment and has to guess; an attachment nobody created
  is exactly what the audit-trail expectation exists to prevent. (Deferred, not
  rejected: prompting the Owner to pick on the flip, pre-ticked from her activity —
  produces ordinary `granted` attachments through the ordinary mechanism, needs
  nothing here.)
- **Attachment derived from any read, not only a write**
  ([#228](https://github.com/markgoho/doula-cloud/issues/228)). Within a month,
  every doula who had browsed would be attached to most Engagements, destroying the
  one question universal attachment exists to answer — who is actually on this
  birth.
- **A single `voided` state distinct from `ended`**, for a mistaken attachment
  ([#228](https://github.com/markgoho/doula-cloud/issues/228)). Deferred, not
  rejected — an attachment opened and closed the same afternoon reads as a
  correction for now; revisit if the pilot shows the record read closely enough for
  the difference to matter.
- **One row for Offer and Attachment together**
  ([#229](https://github.com/markgoho/doula-cloud/issues/229)). Fails against
  [#228](https://github.com/markgoho/doula-cloud/issues/228)'s shape: attachment
  also opens by accrual and by an Admin naming an employee, neither of which has an
  Offer, so a merged row either stands attachment up anyway for those origins or
  invents fake offers for employees nobody offered anything — and one row would
  have to carry `ended_at` and `declined`/`withdrawn`/`superseded`/`expired` at
  once.
- **Refusing to let an employee be offered work at all**
  ([#229](https://github.com/markgoho/doula-cloud/issues/229)). Argued on
  "acceptance grants an employee nothing," which assumed attachment is permission;
  [#228](https://github.com/markgoho/doula-cloud/issues/228) put permission third
  behind "my Engagements" and "who is on this birth," both worth as much to an
  employee as a contractor.
- **A cap on Offer fan-out** ([#229](https://github.com/markgoho/doula-cloud/issues/229)).
  Sharpened instead of capped — the real line is push (an Offer, addressed to her,
  that expires) versus pull (a list she browses for work nobody offered her); N
  Offers are still N Offers.
- **Countering an Offer's fee** ([#229](https://github.com/markgoho/doula-cloud/issues/229)).
  A counter is haggling; withdraw-and-reoffer records the same history and what
  offering-over-assignment protected was agreement, which accept/decline gives in
  full.
- **Membership-first, always — no Offer to a bare email address**
  ([#229](https://github.com/markgoho/doula-cloud/issues/229)). Overridden
  deliberately: pre-launch is exactly when to build the extra mile that gets more
  people onto Doula Cloud without the wait. The "second front door" objection is
  answered by the Offer carrying the same Invitation record, not a parallel one.
- **The fifth read column as a key** — an open Offer partially unlocking Engagement
  and Contract endpoints — instead of a copy
  ([#230](https://github.com/markgoho/doula-cloud/issues/230)). Would have created
  a partly-open state in the API for the first time, given every future `GET`
  author two things to remember instead of one, and handed
  [#231](https://github.com/markgoho/doula-cloud/issues/231) two rules to enforce
  instead of one self-contained record.
- **Refreshing the Offer's copy when the source changes, or prompting a one-click
  reissue** ([#230](https://github.com/markgoho/doula-cloud/issues/230)). The thin
  content barely moves in practice, and a fee that can change while she is reading
  it is exactly what the frozen, accepted-fee copy exists to prevent.
- **Confirming the recipient's address, or binding the read to the first browser
  that opens it**, in place of the emailed code
  ([#230](https://github.com/markgoho/doula-cloud/issues/230)). Address confirmation
  stops nothing a forwarded email doesn't already carry; browser binding stops the
  legitimate recipient as surely as it stops a forward — opened on her phone at
  9am, unopenable on her laptop at lunch.
- **A `GatedRouter`-only mechanism, or a `Reader`-only mechanism**
  ([#231](https://github.com/markgoho/doula-cloud/issues/231)). Neither alone
  satisfies the ticket's own acceptance criteria: `GatedRouter` alone cannot express
  Contract's scope-without-money split; `Reader` alone has no registry a guardrail
  test can walk, so a brand-new endpoint can bypass it entirely and nothing notices.

## Open axes — named, not decided here

- **Whether a contractor's read *narrows* rather than stops when an Engagement
  completes** — keeping what she produced (her Visits, her Messages) while losing
  the live Client record. Live in
  [#228](https://github.com/markgoho/doula-cloud/issues/228) and explicitly not
  taken: it is a sixth read state with no column in either table above. If the
  pilot wants it, it is a new column, not a reinterpretation of "ended."
- **Whether an Admin may change a Membership's employment type**, as an Owner may.
  Left open pending the pilot ([#227](https://github.com/markgoho/doula-cloud/issues/227)).
- **The ambient-write default for an employee Doula is provisional**, pending
  [#243](https://github.com/markgoho/doula-cloud/issues/243) and
  [#244](https://github.com/markgoho/doula-cloud/issues/244) — see the write table
  above.
- **What a Credit is spent on**, and whether attachment changes it (TB-G3, live on
  another map).
- **Worker classification** — whether the law treats a contractor Doula as one, and
  who she invoices — raised on [#230](https://github.com/markgoho/doula-cloud/issues/230),
  researched on [#249](https://github.com/markgoho/doula-cloud/issues/249). Nothing
  in this model waits on it; the read and write rules are the same either way.

## Cost

ADR-0006 named its own cost as new read-gating surface: every read handler serving
gated data needs a role check, and the checks are many and easy to forget. This
model does not remove that cost; it adds three of its own.

**The mechanism now has a compile-time backstop** (`GatedRouter`'s startup panic),
which converts "forgotten" from a silent production hole into a binary that will
not start — a real reduction in the original cost, paid for by every `GET` needing
a declared role list instead of an implicit one.

**The query seam has no such backstop.** `Reader`-gated typed views close the
Contract case but rely on a per-package convention — one exported read accessor per
gated type — that no test can verify without hand-maintained knowledge of every
package's exports. A second gated-response case (if one appears) inherits this gap
until a stronger test exists.

**The write side is gated for the first time.** ADR-0006 covered reads only;
`employee` versus `contractor` writes and the attachment-minting seam are new
surface with no precedent in this codebase to lean on, and the merge-field
money-vs-scope tagging Contract's split needs does not exist yet either — both are
real work this ADR's mechanism section names rather than discovers mid-build.

**The contractor cells of both tables are untestable until the migrations land.**
`employment_type` is not a real column until the migration above runs, so the
contractor and offered-not-accepted columns have no data to gate on. The Owner,
Admin, and employee columns are expressible today; the rest of this document
describes the target the build tickets bring the schema up to.
