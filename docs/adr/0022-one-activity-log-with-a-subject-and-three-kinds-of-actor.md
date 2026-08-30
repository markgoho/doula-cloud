# One Activity log, with a subject and three kinds of actor

`CLAUDE.md` sets the audit trail as a standing expectation on every feature: *"A user must be able to
answer 'how did this thing come to be?' — who sent the invoice, when a person accepted an invitation,
when an employment type changed. Anything that changes state records who did it and when."* The shape of
that record was deliberately left to each feature.

Two features have now chosen, four migrations apart, and they chose the same shape:

- **`practice_membership_events`** (`00039`), built explicitly for that rule, Practice- and
  membership-scoped.
- **`client_events`** (`00042`), Client-scoped: `event_type`, a `jsonb` diff, `actor_kind`,
  `actor_staff_id`, `created_at`, `GRANT SELECT, INSERT` and no `UPDATE` or `DELETE`.

Neither can answer the other's question, and an Engagement — which needs events from both plus its own —
can answer neither. Decided on the wayfinding map
[Holistic application design](https://github.com/markgoho/doula-cloud/issues/405), ticket
[#433](https://github.com/markgoho/doula-cloud/issues/433), while designing the two Engagement detail
pages.

## One log, keyed by subject

There is one append-only table. A row names **what it happened to** — `subject_kind` and `subject_id` —
rather than living in a table named after that thing. `practice_membership_events` and `client_events`
fold into it and are dropped.

The alternative was a third table, `engagement_events`, matching the two that exist. It is cheaper this
week and it is how the schema reaches five event tables by January: every new record type mints another,
and the Practice-wide feed [the design brief](../design/brief.md) promises has to `UNION` all of them in
a fixed order with no shared cursor — which is where pagination quietly breaks. Doing this now costs one
migration and moves no production data, because there is none.

The cost is honest and is accepted: `client_events` already has a writer from the intake work on
[#423](https://github.com/markgoho/doula-cloud/issues/423), so this is a real change to shipped code and
not a rename.

## Three kinds of actor, and only one is a staff member

`client_events.actor_kind` is `staff | system`. That is one short. Designing the Engagement ledger put
eleven real events on a page and they fell into three groups:

- **A staff member did it.** Mark Goho raised the invoice; Tasha Lin edited the care plan.
- **The Client did it.** Amara signed the Contract; Amara paid the invoice. This is not a system event
  with a staff actor missing — `contracts` already records `signer_full_name`, `signed_at` and
  `signer_ip`, so the product already knows the Client acted, it simply has nowhere to say so.
- **Doula Cloud did it, with nobody asking.** An invite email went out; an Offer was superseded when
  another Doula accepted first.

So `actor_kind` is `staff | client | system`, and the third one **displays as "Doula Cloud"**, never as
"System". The product acting on its own behalf has a name, and it is the product's name; "System" is an
engineering word on a screen a doula reads at 3am.

## The log is read through the same gate as the thing it describes

An audit trail that leaks is a way round a permission gate, so the ledger filters by role exactly as its
page does. [ADR-0008](0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md) gives
an employed Doula no read on Contract money or Invoice history; her Engagement ledger therefore omits the
invoice and payment entries an Owner sees. A contractor reads her own agreed fee and never the Practice's
price, so hers omits them too while keeping the Offer she accepted.

This is the strongest argument for one queryable log rather than six tables merged in the browser: the
filter has to run where the rows are, and a client-side merge cannot enforce a rule it has already
fetched past.

## Time is relative near, absolute far, and never destroyed

An entry displays a relative time under seven days — *12 minutes ago*, *2 hours ago*, *Yesterday, 9:31am*,
*Tuesday, 4:40pm* — and an absolute one beyond it: *14 Aug 2026, 8:12pm*. That is what a person reads.

The clock is **12-hour with a lowercase `am`/`pm` and no periods**, because this product is used in the
United States by people who say "she came at two in the morning". The brief's tabular-figures rule still
applies to the numerals; `am`/`pm` is not a numeral and does not need to align.

The exact instant is **carried underneath and never replaced** — the rendered element keeps the full
timestamp as its machine-readable value, so a screen reader, a hover and a copy-paste all get the real
one. An audit trail whose only surviving form is *"2 hours ago"* is not an audit trail. This is the one
place where the ledger's two jobs — being readable and being evidence — actually conflict, and both are
served rather than one traded for the other.

## The fourth event table, and why it stays

`staff_work_state_events` (`00043`) is the same shape again — append-only, `GRANT SELECT, INSERT` and no
`UPDATE` or `DELETE`, a subject, a previous and a next value, an actor and a timestamp — and it does
**not** fold in. It is not an oversight, and it is not a preference: `activity` cannot hold its rows.

`activity.practice_id` is `NOT NULL`, and the one policy above gates `INSERT` as well as `SELECT`. A work
state is a fact about a *person*, not about a Membership — `00043` argues that at length, and it is why
that table carries no `practice_id` — and its only writer, `PUT /api/staff/work-state`, is mounted
**outside** the Practice-scoped middleware on purpose, so `app.current_practice_id` is unset while it
runs. An `INSERT` into `activity` from there is refused by its own policy.

The two ways out are both worse than the fourth table. Making `practice_id` nullable removes the single
comparison the whole table's isolation rests on. Writing one row per Membership makes a contractor doula
on three rosters assert three times, and gives a Practice that hires her *next* year no history at all,
because the fan-out happened before it existed.

So the rule this ADR sets is about Practice-scoped history, which is all the history the product had
when it was written. A fact about a person, visible to every Practice that person works for, is a
different subject, and `00043`'s own `EXISTS` policy is what scopes it. Recorded on
[#459](https://github.com/markgoho/doula-cloud/issues/459), which built that table's first reader.

## Considered and rejected

- **A third `engagement_events` table.** Rejected above: cheaper now, five tables by January, and no
  shared cursor for the Practice-wide feed.
- **Deriving the ledger by joining the tables that already carry timestamps.** Rejected on the evidence:
  today that yields messages and one signature. Contract sent, Contract voided, Visit reassigned, Plan
  edited, Offer superseded and Engagement completed are all bare `UPDATE`s with no actor and no time, so
  the join returns a ledger that is mostly silence about the events people would actually ask after.
- **Keeping `actor_kind` at `staff | system`** and recording a Client's signature as a system event.
  Rejected: it makes the product the author of an act a person took, which is the one thing an audit
  trail exists not to do.
- **"System" as the display name.** Rejected as jargon, per
  [ADR-0005](0005-one-context-client-register-at-the-ui-edge.md)'s instinct even though that rule binds
  only the portal.
