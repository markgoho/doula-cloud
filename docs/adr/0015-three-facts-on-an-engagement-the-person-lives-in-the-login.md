# Three facts on an Engagement; the person lives in the login

Supersedes one paragraph of [ADR-0005](0005-one-context-client-register-at-the-ui-edge.md) — the
one fixing a `_Client says_:` label for each of four `engagement_status` values. There are three
values now, and `postpartum` is not one of them. Everything else in ADR-0005 stands, including the
rule that produced this change: *if a status cannot be labelled kindly for every Client, the status
set is missing a value.* ADR-0005 named Nadia Haddad as the live instance and argued the status set
was short a terminal state for a loss. This document answers that argument, and the answer turned
out to be that the set was one value too **long**.

It amends nothing in [ADR-0001](0001-practice-defined-plan-templates.md). A Plan Instance is still a
snapshot per Engagement; this document adds only *when a Birth Plan is offered at all*, which
ADR-0001 never spoke to. It restates rather than extends
[ADR-0008](0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md) on what a
terminal state ends, and leaves ADR-0008's open axis open.

This ADR was chartered on the wayfinder map
[What an Engagement is: its lifecycle, its kind, and the person it belongs to](https://github.com/markgoho/doula-cloud/issues/351),
and collects four closed decisions —
[#352](https://github.com/markgoho/doula-cloud/issues/352),
[#353](https://github.com/markgoho/doula-cloud/issues/353),
[#354](https://github.com/markgoho/doula-cloud/issues/354),
[#355](https://github.com/markgoho/doula-cloud/issues/355) — into one buildable model. Nothing in the
model sections is decided here; each is traceable to its ticket. Where two tickets disagreed, this
document rules and says so.

## The model, in one pass

A **Portal Account** is one person's login. A **Client** is one Practice's record of that person. An
**Engagement** is one piece of work at one Practice. The cardinality is one rule: **a Portal Account
reaches many Clients, at most one per Practice; a Client has many Engagements, all at its own
Practice; an Engagement belongs to exactly one Client.** The Portal Account is the only thing in the
model that knows two Client records are the same woman, and *she* is the one who told it, by
accepting each Practice's invite. **No Client fact crosses a Practice.**

An Engagement carries **three separate facts about how it is going**, not one enum.

- A **status** says *where the work is*: `intake`, `active`, `completed`. It moves.
- A **birth outcome** says *what happened*: `live_birth`, `loss`, `unknown`, or not yet known. It is
  recorded once the fact is known and describes the birth alone. Staff-only.
- An **ending reason** says *why the work stopped*: `care_complete`, `client_withdrew`,
  `practice_ended`, `transferred`, `no_response`, `entered_in_error`. Staff-only, required at
  completion.

It carries a fourth fact about what it *is*: a **kind**, `birth` or `postpartum`, which is what the
Practice sold. And it carries a nullable **due date**, which the product already collects onto the
Offer and which belongs here.

`completed` is the only terminal status. Completion ends **reach** — granted Attachments and open
Offers — and ends nothing else: Messages both ways, Visits, the Contract and Invoices all stay open,
and the Client's portal reaches a `completed` Engagement forever. Recording the birth outcome freezes
the outcome and the date the pregnancy ended; `client_id` is immutable from creation. Together those
mean a different person or a different baby needs a new Engagement, which locks a new **Credit**.

## The collision this document was chartered to rule

The map held one tension open on purpose: `postpartum` is a status today, and the kind question
might argue it is a kind. **It is neither.**

The status column was carrying **two vocabularies at once** — the relationship (`intake` → `active` →
`completed`) and the care cycle (prenatal → birth → postpartum). Kind is a third question, distinct
from both: *what was sold*. Three questions, three homes.

`postpartum` leaves the status set and becomes a derived **Visit type** — prenatal before the
pregnancy ended, birth, postpartum after — because that is where the billing fact belongs. Medicaid
and insurance do not reimburse an Engagement; they reimburse visits, and each visit is one kind. The
likely reason `postpartum` became an Engagement status at all is that Visits could not be typed
(**PR-G6**): the status column was standing in for a missing field on another table.

An Engagement's phase stays answerable without it. It is post-birth exactly when its birth outcome is
recorded.

## The status set, and the concept the product does not have

The status set was never defined in words, and the transition rule could not be fixed until it was.

| Status | Meaning | `_Client says_:` |
| --- | --- | --- |
| `intake` | The Client has agreed and setup is under way; care has not started | Getting started |
| `active` | Care is booked or happening | Ongoing |
| `completed` | Care has ended, for whatever reason | Care ended |

`intake` is **onboarding**, not a pipeline stage. The Client has already agreed to work with the
Practice, and the Practice is gathering what it needs to *begin* — the intake call, her history, the
Contract, the first booking.

**The product has no lead or prospect concept**, and this ADR states that explicitly so `intake`
never quietly becomes one. Three facts already in the model rule that reading out:

- **A Credit locks when an Engagement is created** ([#332](https://github.com/markgoho/doula-cloud/issues/332),
  [#45](https://github.com/markgoho/doula-cloud/issues/45)). If `intake` held leads, a Practice would
  pay Doula Cloud for every enquiry that went nowhere. That is not the pricing model.
- **`intake` carries a binding Client-portal label** under ADR-0005 — "Getting started". Only a
  person with a portal login reads it, and a Practice does not hand a portal login to someone it has
  not taken on.
- **The freeze rule** treats an Engagement as one person and one baby for life. A lead pipeline needs
  the opposite: cheap, disposable, re-pointable rows.

If the pilot wants a pipeline it arrives as a new entity with its own decision, never as a status
value.

Every label above passes ADR-0005's test — one fixed label, true for every Client — on Nadia Haddad,
whose pregnancy ended in loss and whose bereavement care continued afterwards. "Ongoing" is true;
bereavement care is ongoing. "Care ended" is true; her care does end. The lies ADR-0005 found came
from `active` having to imply a continuing pregnancy and `completed` having to imply work that
finished as planned. Neither implication survives once the outcome is a separate fact.

`postpartum` was the one label that could not be written kindly for every Client, because it names a
state defined by having had a baby.

## The six directed moves

| Move | Ruling |
| --- | --- |
| `intake` → `active` | **Legal, and automatic** — see below |
| `active` → `completed` | Legal |
| `intake` → `completed` | Legal — an intake that never converts must be closable |
| `completed` → `active` | Legal **as a correction only** |
| `active` → `intake` | **Refused** |
| `completed` → `intake` | **Refused** |

`intake` → `completed` is legal on its own account rather than by omission: a Practice that opens an
Engagement and never converts it has already spent a Credit, and leaving that row `intake` forever is
the same dead column [#253](https://github.com/markgoho/doula-cloud/issues/253) found.

**Reopen is undo, not resumption.** `completed` → `active` exists so a wrong completion can be taken
back — a mis-click, or a Practice that closed a Client who then turned out still to need care. It is
**not** the path for a returning Client: she gets a **new** Engagement and it locks a new Credit, and
the freeze rule makes that structural rather than something people are asked to remember. Reopen
**unfreezes nothing** and carries **no time limit**; a window would only convert a fixable mistake
into an unfixable one at an arbitrary hour.

The ending reason and note **clear** when a status leaves `completed`, and are required again at the
next completion. The previous values survive on `engagement_events`, so reopening loses no history.

### One transition is automatic, and one never is

**`intake` → `active` happens by itself, the first time a Visit is scheduled.** A status a human must
remember to advance is paperwork, and this platform sells more time with clients and less time on
paperwork. A Practice that forgets leaves Hannah Sorensen reading "Getting started" through her
birth. Scheduling is the trigger because booking the first Visit is exactly the moment setup ends and
care begins: `intake` is "nothing on the calendar yet", `active` is "care is booked or happening".
The event records **the person who scheduled**, not a null system actor — a doula did that. The
manual move stays available, for a Practice that does not book Visits in the app.

**Nothing ever completes automatically.** The tempting trigger is "the birth outcome was recorded",
and Nadia forbids it directly: her bereavement Visits continue *after* the loss, so auto-completing
on a recorded outcome closes her Engagement in the middle of her care. The two directions are not
symmetric, and that is why they are ruled differently — auto-activation changes a label and ends
nothing, while completion ends Attachments and withdraws open Offers.

Where a nudge is wanted, it is a **prompt**: an Engagement with a recorded outcome and no Visit for
some weeks surfaces as *is this care finished?*, and a person still answers. This ADR names the
prompt as a surface worth having and does not specify its interval.

## Who may move a status

Against ADR-0006's roles as ADR-0008 supersedes them.

| | Owner | Admin | Doula (employee, attached) | Doula (contractor, attached) |
| --- | --- | --- | --- | --- |
| `intake` → `active` (manual) | ✓ | ✓ | ✓ | ✗ |
| `active` → `completed` | ✓ | ✓ | ✓ | ✗ |
| `intake` → `completed` | ✓ | ✓ | ✓ | ✗ |
| `completed` → `active` (correction) | ✓ | ✓ | ✗ | ✗ |
| Correct a frozen `birth_outcome` | ✓ | ✗ | ✗ | ✗ |

Reopen is Owner/Admin only because it is the move that reaches back past a finished record; an
employed Doula's job is the care, not the correction.

**The contractor cell is a product decision on a legally unsettled axis, not a claim about her
classification.** The instinct that a contractor Doula owns her relationship with the Client is real,
and [#249](https://github.com/markgoho/doula-cloud/issues/249)'s research supports it rather than
dismissing it: ADR-0008's ambient-versus-attached split is not *adjacent* to the legal control test,
it **is** that test expressed as data. Every test surveyed — IRS behavioral control, all six DOL/FLSA
factors, prong A of the California and Massachusetts ABC tests — turns on the hiring party's retained
authority to direct the work, so "the contractor may do nothing" is not a safe default; more Practice
control points *toward* employee, not away from it. The federal standard is genuinely unsettled —
three live frameworks as of 2026 — so no ruling here can be made *correct* by picking carefully.

The cell is ruled on an argument that holds whichever way classification lands: **"my work is done"
and "the relationship is over" are two facts, and the model already holds them separately.** Her
**Attachment** ending says her work finished, and ADR-0008 already lets her end it herself. The
Engagement reaching `completed` says the **Practice's** relationship with its Client finished, and a
Client is one Practice's record of a person, so closing that record is the Practice's act on its own
record. She marks her job done; she does not close the Practice's account with its Client.

This is a **cheap cell to move** — ADR-0006's own words for the Staff-roster cell it wrote the same
way. If the pilot's contractors experience it as the Practice reaching into their work, moving it is
a permissions change, not a model change. It was ruled rather than left open, because "nobody can
complete this Engagement" is not a state the product can ship in.

## The birth outcome, and where it does not reach

`live_birth`, `loss`, `unknown`, nullable. It describes **the birth**, and nothing else.

- `live_birth` — the baby was born alive.
- `loss` — the pregnancy ended without a living baby.
- `unknown` — the Engagement ended and the Practice never learned. This is the honest value for a
  Client who withdrew at twenty weeks; the birth happened, or did not, out of sight.
- `null` — not yet recorded.

A baby born alive is `live_birth` **even if the baby later dies**. A neonatal death is a separate,
later event and does not rewrite how the birth went.

**Staff-only. Never Client-facing.** No `_Client says_:` entry is needed, because ADR-0005's register
binds only what a Client reads. Nadia does not need the software to tell her what happened; she needs
it to stop asking her about a pregnancy.

It is **correctable**. A `loss` entered on the wrong Engagement silences a woman's Birth Plan and
marks her record; write-once would make that typo permanent. It is recorded **whenever it becomes
known**, not at the end — a postpartum-only Engagement records `live_birth` at intake, because the
baby was born before the Practice was hired.

**The date the pregnancy ended is recorded with it**, and is distinct from when a staff member typed
it in: those can be days apart, and Visits logged in between would type wrongly. This date is the
pivot the Visit-type derivation turns on, and it is equally true for a loss — postpartum care after a
loss is still postpartum. It is **not** a due date; **MO-G3** stays open and untouched.

### The model is columns on a row, not a timeline

This is stated so it cannot creep in later by accident. An Engagement's outcome is **columns on a
row**, not a timeline of dated events with status as a summary of the latest one. That timeline is a
materially different model. If it is coming — and *how an Engagement records events after the birth*
is a real open question — it arrives as its own decision with its own reasoning, rather than through
a fog line that quietly contradicts this document.

### Post-loss care is the same Engagement

Bereavement support after a loss continues on the **same** Engagement. It is one continuous
relationship, the record stays whole, and no Credit is spent — which matters, because a returning
Client's new Engagement locks a new Credit, and a second Engagement would charge a Practice for
supporting a woman who just lost a baby. This also settles the framing: the loss is an **outcome**,
not an ending.

## The ending reason

Track what each fact answers about a Client who leaves at twenty weeks:

- *Where is the work?* — it stopped. **Status** = `completed`.
- *How did the birth end?* — the Practice never learned. **Birth outcome** = `unknown`.
- *Why did the work stop?* — she withdrew. **Ending reason** = `client_withdrew`.

It is a separate fact rather than a value in the status column, because `withdrawn` as a status value
starts the slide back into one column doing two jobs, and every such value would need a kind Client
label for a distinction the Client should never be shown.

| Value | Meaning |
| --- | --- |
| `care_complete` | The work finished as agreed |
| `client_withdrew` | She stopped, for her own reasons |
| `practice_ended` | The Practice ended it — non-payment, a bad fit, safety |
| `transferred` | She moved to another provider |
| `no_response` | She stopped answering and was never formally closed |
| `entered_in_error` | The Engagement should never have existed |

Staff-only, never Client-facing, so ADR-0005 needs no register entry. Required on every move to
`completed`, so a `completed` row always carries both a birth outcome and an ending reason. Settable
and correctable by Owner or Admin. A nullable **`ending_note`** sits beside the enum, because the
Practice's actual account of why care stopped exists whether or not the schema holds it.

**This vocabulary is explicitly pending the pilot group.** `entered_in_error` is the one doing
something different from the other five — it says the record is junk rather than that care stopped,
and it may earn its own fact rather than a slot in this list. It stays until the pilot shows it needs
splitting.

### Completion requires both facts

**An Engagement may not reach `completed` while its birth outcome is null**, enforced as a database
`CHECK` and not only as a handler guard. Its cost is real: a Client who vanished during intake, whom
the Practice never got near a birth with, must still be marked `unknown`. That is the right cost.
`unknown` is the honest answer, not an admission of sloppiness, and this constraint is the only thing
that ever causes it to be written down. Without it the column stays null forever on every abandoned
Engagement, and *was this recorded, or did nobody ask?* becomes unanswerable.

## Kind: what the Practice sold

An Engagement carries a **kind** — `birth` or `postpartum` — as a `NOT NULL` column the Practice
sets. It is a fact about **what the Practice sold**, not about where the Client is in the birth
cycle, and the difference between those two questions is the whole reason the fact is stored rather
than derived.

**Deriving it was rejected.** The derivation looks unbeatable: pregnant when care starts, so a birth
is ahead → birth work; baby already here → postpartum work. It stores nothing, asks nobody, and
covers both journeys the evidence was filed from. It fails on one Client, and she is not an edge
case: **a woman seven months pregnant who hires a postpartum doula for after the baby comes.** Her
birth-cycle position says `birth`. Her agreement says `postpartum`. No comparison of dates can tell
those apart, because they answer different questions. *Where she is* and *what she bought* usually
agree, and when they disagree the sale is right.

The derivation survives as the **default**, never as the definition.

`birth` means a birth is part of this relationship — and the postpartum care that follows it, which
is why there is no third value. `postpartum` means the Practice was hired for postpartum work with no
birth in the Engagement. **`both` is rejected**: nearly every birth Engagement continues into
postpartum care, so in a three-value list `both` would be the everyday answer and `birth` would come
to mean "a birth after which we walked away." A list whose common case is named after an exception
gets filled in wrongly.

**Set by the Practice, defaulted, never typed in the common case.** The same intake control that
records *is the baby here yet?* supplies the default: recorded as already born → `postpartum`;
otherwise → `birth`. One control changes it, and the pregnant postpartum client is a single click.
This is the create-time alternative [#308](https://github.com/markgoho/doula-cloud/issues/308)
(CB-G2) asked for, without a form interrogation.

**Mutable in both directions, audited.** A Client who booked postpartum care may decide she wants a
doula at her birth; a Client who booked birth attendance may drop it and stay on for postpartum care.
The second is rarer, and it is still allowed, because the value is *defaulted* rather than typed and
a mis-set default is guaranteed.

**Never Client-facing.** No `_Client says_:` entry. ADR-0005's register binds words a Client
**reads**, and kind is a fact about what the Practice sells, not about her care. Camille knows she
hired a postpartum doula; the portal captioning her own life back to her adds nothing. She meets the
*consequence* of kind — a portal with no Birth Plan section — and never the word.

### The post-birth freeze on kind is a rule, not a constraint

**Ruled here**, because the map's own tickets disagreed.
[#353](https://github.com/markgoho/doula-cloud/issues/353) first specified the freeze as a database
constraint, then corrected itself to a semantic and UI rule;
[#355](https://github.com/markgoho/doula-cloud/issues/355), written later, cited #353's original text
and re-asserted database enforcement. This document takes #353's correction.

The product **stops offering** a `postpartum` → `birth` change once the birth outcome is recorded,
because attending a birth that has already happened means nothing. That is a statement about the
**upgrade**. Generalising it into a database constraint breaks one cohort completely: a
postpartum-only Engagement records `live_birth` **at intake**, so Camille's kind would be frozen from
the moment the row was created and would never get a correction window at all. That fights the
standing rule that an Engagement is editable in place, and it repeats the mistake the birth outcome
itself rejected — a typo made permanent. The escape hatch a constraint would leave (null the outcome,
fix the kind, re-record the outcome) is the cancel-and-recreate smell in a different shape.

So **`kind` is not frozen in the database.** Correcting a mis-set kind stays possible and audited,
exactly like every other correction on the Engagement. Nothing is lost by this: kind is not a route
to serving a second baby on a spent Credit. `client_id` immutability and the outcome freeze are what
close that door.

### The due date moves to the Engagement, nullable

Not strictly a kind question, and it turned out to be the same question. `engagement_offers.due_date`
is `date NOT NULL` today, so the product already collects a due date — onto the **Offer**, not the
Engagement. Two consequences follow that nobody had filed:

- **A postpartum-only Engagement cannot be offered to a contractor at all**, because there is no due
  date to put in a `NOT NULL` column.
- The Practice re-enters the same date on every Offer, and the Engagement cannot answer *when is this
  baby due*.

The due date belongs on the **Engagement**, **nullable** — Camille's Engagement has no due date and
never will. The Offer copies it rather than asking for it, and `engagement_offers.due_date` loses
`NOT NULL`. This does not settle **MO-G3**, which asks the larger question of whether and how a due
date is modelled; it says only where the due date the product *already collects* should live.

### An Offer says which kind of work it is

[#230](https://github.com/markgoho/doula-cloud/issues/230) fixed the Offer copy's fields before kind
existed. It gains one. For a contractor these are not variants of one job, and the difference is
bigger than on-call versus scheduled: **a full birth-cycle Engagement includes a few postpartum
visits, while a postpartum-only Engagement may be five to ten visits, or once a week until the baby
is six months old.** She is being asked for a materially different commitment of her calendar, and
#230's own standard is that the Offer must be decidable by an outsider. Leaving it to the Practice's
free text makes the one fact she needs most the one the software does not guarantee she is told.

Kind is **printed** on the Offer, not branched on. The due-date line rides along: with the Engagement
owning a nullable due date, a `postpartum` Offer has no due-date line rather than an empty one.

### What suppresses the Birth Plan

Two facts, not one:

> **Offer to create a Birth Plan when `kind = birth` AND the birth outcome is null. Show an
> already-created one always.**

| Client | kind | birth outcome | Birth Plan |
| --- | --- | --- | --- |
| Maya, 28 weeks pregnant | `birth` | `null` | Offered |
| Camille, returning postpartum-only | `postpartum` | `live_birth` | Never offered |
| Pregnant, booked postpartum-only | `postpartum` | `null` | Not offered |
| …who then upgrades | `birth` | `null` | **Now offered** |
| A birth client, two days post-birth | `birth` | `live_birth` | Existing plan readable; no new one |
| Nadia, after a loss | `birth` | `loss` | Existing plan retired |

This names the two suppressors [#311](https://github.com/markgoho/doula-cloud/issues/311) (CB-G5) had
conflated. **Hiding** on Camille is kind's job — there is nothing to author. **Retiring** on Nadia is
the living-baby rule's job — something was authored and must stop being offered. A rule keyed on the
birth outcome alone is **wrong**, and is recorded as wrong so it is not rediscovered: it offers a
Birth Plan to the pregnant postpartum client.

A Birth Plan on an Engagement that changes to `postpartum` is **retired, not deleted**. The
Engagement is a permanent record.

### A kind change moves nothing automatically

An upgrade from `postpartum` to `birth` is a Client buying more. **The Contract, the Invoice, and the
Credit ledger are untouched by the change itself.** Contract and Invoice are the Practice's own
instruments: an upgrade means a new Contract and a new Invoice, made by hand the way any other change
of terms is, because the alternative is Doula Cloud inferring commercial intent from a radio button.
**No second Credit** — a Credit covers the complete birth cycle whatever the kind, and this is the
same Engagement.

## Identity: the person lives in the login

**A Portal Account reaches many Clients, at most one per Practice; a Client has many Engagements, all
at its own Practice; an Engagement belongs to exactly one Client.**

### Why a Client is a Practice's record, not the person

`clients` was created global on purpose — `00005_client_engagement.sql` has no `practice_id`, and
`engagements.client_id` carries no uniqueness constraint. But the globality was **already fiction**:
the `clients_select` RLS policy makes a Client row visible only where an Engagement at the *current*
Practice points at it, so Practice B cannot find the row Practice A created. Reuse-an-existing-Client
has always been within-Practice only.

Making the global row real would need a new cross-tenant lookup door, and a door that answers "yes,
that email exists" tells an agency that a named woman is already another agency's client. This
product already treats that class of fact as serious — it is why an Offer carries the Client's first
initial and general area rather than her name.

The requirement that argued for a global Client — *she logs in and sees her care across Practices* —
is delivered here in full. It does not need a shared row; it needs the person to live in the
**login** instead of in `clients`. The only thing a global row additionally buys is **staff-side
cross-Practice reuse**, and that is exactly the leak. The two are separable, and only the second is
refused.

**This is the more reversible of the two models.** Going the other way later — merging into one
global row — has its merge key built in: the set of Clients sharing a Portal Account **is** the
evidence, asserted by the woman herself. The reverse split is worse, because it would need each
demographic edit attributed to a Practice, and that history was never recorded.

### The cross-Practice boundary

**No Client fact crosses a Practice.** A Practice reads its own Client records and its own
Engagements. That another agency serves the same woman is her business. Concretely: **no
`staffauth`-scoped read may return a Portal Account's sibling links**, and the join is legible only
from a portal session. The portal is the only surface where the person is one person.

**Shared reference data is a different category, not an exception.** A hospital in Rochester NY is a
public fact about the world, not a fact about a Practice or a Client, so a shared catalogue leaks
nothing — **provided** it carries no usage signal (no "added by", no counts, no "3 practices use
this"). The moment it carries one, it becomes a client-adjacent fact and the rule above bites.

### Session resolution

`clientauth.Middleware` resolves the verified uid to a **set** of Clients rather than one.
`app.current_client_id` **stays a single value**, set from the addressed Engagement's own `client_id`
after checking that Client is one this account reaches — so every existing client-tier RLS policy
keeps working untouched, and the ownership check widens from equality to set membership.

The portal root list has no `:engagementId`, so it gets **one new read-only identity-tier policy on
`engagements`**. Per ADR-0006 the real refusal lives on the API read endpoint; this is the backstop,
and without it the one cross-Client read in the product would have no database-level guard at all.

### The two 409s

There were always two doors, and they are ruled differently.

- **Invite time** — `portalinvite/invite.go`, "this client already has portal access". **Stays.** It
  is keyed on `client_id`, so it says *this Practice's record already has a login attached*, which
  remains a genuine conflict.
- **Accept time** — `portalinvite/accept.go`, "a portal account already exists for this identity",
  raised on the `UNIQUE` violation when accept writes an `identity_uid` that already exists.
  **Deleted.** This is the bug [#309](https://github.com/markgoho/doula-cloud/issues/309) reported.
  Accept becomes: find-or-create the `portal_accounts` row for the verified uid, then set
  `portal_account_id` on the invited row. The composite unique still refuses a genuine double-link of
  the same account to the same Client, and a consumed token still falls to the existing 404.

### What the portal shows, at model level

- The root resolves to **her Engagements across every Practice**, each labelled by the Practice. With
  exactly one, the portal may open it directly — that is a UI shortcut over a list, not a second
  model.
- **"Current" is derived, never stored.** The most recently created non-`completed` Engagement. A
  stored pointer is a fact that goes stale and needs a rule for who updates it; the model can already
  answer by ordering.
- A **`completed` Engagement stays reachable forever.** The Engagement is a permanent record, and her
  Messages and Birth Plan live inside it.

### One consequence, named rather than deferred

**Two logins, two people.** If she uses two different Identity Platform logins at two Practices, she
gets two Portal Accounts and the model does not know they are the same woman. Accepted; there is no
merge in v1.

**Nothing is asked of Identity Platform.** She keeps one login under either model, so this is purely
a table-shape change and [ADR-0004](0004-bff-owned-sessions.md) is not implicated.

## What a terminal state ends, and what it does not

Completion ends **reach**, not the record.

**Open Offers** — ADR-0008 already rules it, and this restates rather than extends: completion closes
every open Offer as `withdrawn`, with `decided_by` null, because that cascade is the one terminal
state in the system with no human actor by construction.

**Granted Attachments** — ADR-0008 already rules it: they end. Ending is `ended_at`, never a delete,
because "she was on this from February to May" is more of the record than "she was on this". A
contractor's reach stops the moment it does, and the Visits she worked are untouched.

ADR-0008 named an **open axis** here — whether a contractor's read *narrows* on completion (keeping
her own Visits and Messages) rather than stopping dead — and declined to take it. **It stays open.**
Nothing on this map needs it, and it is a sixth read state with no column in either table.

**Everything else stays open.**

| Surface | On a `completed` Engagement |
| --- | --- |
| Messages, staff → Client | Open |
| Messages, Client → staff | Open |
| Visits | Open — backdated logging is real work |
| Contract and Invoices | Open — a final invoice after care ends is ordinary business |
| Portal reachability | Forever |

A woman who lost a baby in March and writes to her doula in June must not hit a closed door.
Refusing Visits would make the correction move the only way to log Friday's visit on a Monday after a
Saturday completion, which turns an ordinary act into a status edit. Refusing invoicing would make
completion something a Practice avoids for cash-flow reasons, which is the opposite of what a
truthful status column needs. A Practice that wants that door closed is asking for a per-Practice
setting, not a model rule.

The abuse this openness might have created is closed by the freeze rule below rather than by locking
surfaces.

## The freeze rule

Credits are per Engagement, so the abuse to design against is **serving a second baby on a Credit
already spent**. Complete-and-reopen is one route to it. The other, which a completion-time freeze
would miss entirely, is simply **never completing**: leave one Engagement `active` forever, edit the
Client and the due date each time, pay once.

The freeze point that closes both doors is **the moment the birth outcome is first recorded** — which
the completion `CHECK` guarantees happens no later than completion.

- **`client_id` is immutable, always.** An Engagement belongs to exactly one Client; this makes it a
  constraint rather than a sentence.
- **Recording the birth outcome freezes the outcome itself and the date the pregnancy ended.**
- **Reopening unfreezes nothing.** It resumes *care* — log a Visit, send a Message, issue a final
  Invoice, complete again. It cannot re-point the record at a different person or a different baby.
- **Therefore a different person or a different baby requires a new Engagement, which locks a new
  Credit.** That is the entire rule.

`kind` is deliberately **not** in that list; see the ruling above. It is not a route to a second
baby.

**The cost, stated rather than hidden**: a genuinely wrong outcome — `loss` typed where `live_birth`
belonged — would become uncorrectable, and correctability was a deliberate choice because a wrong
`loss` silences a woman's Birth Plan and marks her record. One narrow escape exists: **an Owner, and
only an Owner, may correct a frozen birth outcome**, and every correction writes an
`engagement_events` row. Not an Admin, not a Doula. It is a hatch, not an editing surface.

**One door this document does not own.** Renaming the **Client record itself** still substitutes a
person if nobody stops it. Intake-side Client reuse belongs to
[#332](https://github.com/markgoho/doula-cloud/issues/332), and the rule it must obey is stated here:
**a Client record is one person, and editing its name corrects a spelling — it does not substitute a
human being.**

## Standing rules this document carries

Three rules that bind surfaces beyond anything named here. They are written as rules rather than as
lists of today's offending screens, because the product is safe today only in that reminders, due
dates and automated copy do not exist yet, and a list written now is a list of the two things that
happen to exist.

**1. Consult the living baby.** *Any surface that presumes a living or expected baby must consult the
derived question **does this Engagement have a living baby?** before it renders.* The rule keys on
the derived question rather than on the birth-outcome column, so a later neonatal-death event becomes
a second input without the rule being rewritten. Today its only input is the birth outcome:
`live_birth` → yes; `loss` or `unknown` → presume not. **`unknown` is deliberately a presume-nothing
value**: a withdrawn Client's Engagement stops presenting baby-shaped surfaces. That is the safe
direction, and it is a choice. This binds [#294](https://github.com/markgoho/doula-cloud/issues/294)
and [#296](https://github.com/markgoho/doula-cloud/issues/296) without either being closed here.

**2. An Engagement is editable in place.** *A Practice pays per Engagement, so no change to what the
Practice sold may require cancelling and recreating one, because that would charge a second Credit
for fixing a fact.* It is why the rare `birth` → `postpartum` downgrade is allowed, and why kind is
not frozen in the database. It also retroactively names the reason post-loss care stays on the same
Engagement.

**3. Manual is a decision for now, never a closed door.** *This platform sells more time with clients
and less time on paperwork, so any status the product can infer from work a person already did is a
candidate for automation.* Auto-activation on the first scheduled Visit is the first instance;
nothing ever auto-completes, for the reason given above.

## Concrete schema

**This is a specification, not a migration.** No file exists in `api/db/migrations/` for any of it,
deliberately: a file there *is* the migration, and goose would run it on the next test spin-up
against handlers that do not know these columns. The build effort writes the migration; this section
is the target it writes to.

There is no production data and no backwards compatibility to preserve, so this is written as the
schema the model wants rather than as the smallest diff. Column names below are the recommendation,
not the decision — the decision is the fact each column holds.

### Enums

```sql
-- Three values. 'postpartum' is gone; it is a Visit type now.
CREATE TYPE engagement_status AS ENUM ('intake', 'active', 'completed');

CREATE TYPE engagement_kind AS ENUM ('birth', 'postpartum');

CREATE TYPE birth_outcome AS ENUM ('live_birth', 'loss', 'unknown');

CREATE TYPE engagement_ending_reason AS ENUM (
    'care_complete', 'client_withdrew', 'practice_ended',
    'transferred', 'no_response', 'entered_in_error'
);

CREATE TYPE engagement_event_type AS ENUM (
    'status_changed', 'kind_changed', 'birth_outcome_recorded', 'ending_changed'
);
```

### `engagements` — four new facts, a due date, and one `CHECK`

```sql
kind                engagement_kind NOT NULL,   -- no database DEFAULT; the intake control supplies it
due_date            date,                       -- nullable: a postpartum-only Engagement has none
birth_outcome       birth_outcome,              -- nullable until known
pregnancy_ended_on  date,                       -- the event date, not the recording date
ending_reason       engagement_ending_reason,   -- required at 'completed', cleared on reopen
ending_note         text,                       -- staff-only, always optional

CONSTRAINT engagements_completed_is_explained CHECK (
    status <> 'completed'
    OR (birth_outcome IS NOT NULL AND ending_reason IS NOT NULL)
),

CONSTRAINT engagements_outcome_is_dated CHECK (
    (birth_outcome IS NULL) = (pregnancy_ended_on IS NULL)
)
```

`kind` takes **no database default**. A default in the schema would be a second opinion about what
the Practice sold, silently applied by whatever forgets to pass it; the intake control is the only
thing that knows.

The second `CHECK` keeps the outcome and its date as one fact, so the Visit-type derivation is never
handed an outcome it cannot place in time.

### Immutability is a trigger, because a policy cannot see both rows

`client_id`, `birth_outcome` and `pregnancy_ended_on` are immutable — in the database, not only at
the handler. RLS cannot express this: an `UPDATE` policy's `USING` clause sees the old row and its
`WITH CHECK` sees the new one, and neither can compare the two. A `BEFORE UPDATE` trigger on
`engagements` is the only place the comparison exists.

```
BEFORE UPDATE ON engagements, raise unless:
  NEW.client_id = OLD.client_id
  AND (OLD.birth_outcome IS NULL
       OR (NEW.birth_outcome, NEW.pregnancy_ended_on)
           IS NOT DISTINCT FROM (OLD.birth_outcome, OLD.pregnancy_ended_on)
       OR current_setting('app.allow_outcome_correction', true) = 'on')
```

The **who** stays at the handler, per ADR-0006: the database enforces that a frozen outcome cannot be
overwritten by an ordinary update, and the Owner-only correction path is the one handler that sets
`app.allow_outcome_correction`. This is the same narrow-session-variable-door idiom
`app.invite_token` (`00026`) and `app.invite_token_digest` (`00039`) already use — one handler sets
it, one policy or trigger reads it, and nothing else in the codebase can.

`kind` is **not** in the trigger. See the ruling above.

### `engagement_events` — the audit record

Shaped on `practice_membership_events` (`00039`): one row per change, **both sides** recorded, so the
answer is readable without replaying anything.

```sql
CREATE TABLE engagement_events (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id             uuid NOT NULL REFERENCES practices (id),
    engagement_id           uuid NOT NULL REFERENCES engagements (id),
    event_type              engagement_event_type NOT NULL,
    previous_status         engagement_status,
    status                  engagement_status,
    previous_kind           engagement_kind,
    kind                    engagement_kind,
    previous_birth_outcome  birth_outcome,
    birth_outcome           birth_outcome,
    previous_ending_reason  engagement_ending_reason,
    ending_reason           engagement_ending_reason,
    previous_ending_note    text,
    ending_note             text,
    actor_staff_id          uuid REFERENCES staff (id),   -- nullable, deliberately
    created_at              timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX engagement_events_engagement
    ON engagement_events (practice_id, engagement_id, created_at);

GRANT SELECT, INSERT ON engagement_events TO app_runtime;   -- no UPDATE, no DELETE
```

RLS: one plain practice-tier column comparison, the same shape as
`practice_membership_events_practice_visibility`. It carries `practice_id` directly so no subquery is
needed, and it is **never** readable from a portal session — the audit trail is the Practice's record
of its own acts.

**One table, all four mutable facts**, rather than one table per fact. `kind` is mutable and the
outcome is correctable, so three separate audit tables would be three times the work for one
question, and the cross-cutting expectation is that *how did this thing come to be?* has one place to
look.

`actor_staff_id` is **nullable** even though **no transition in this document actually writes a null
actor today**: auto-activation records the person who scheduled the Visit, and completion's Offer
cascade writes to `engagement_offers` rather than here. The column is nullable for the automations
the standing rule anticipates, and stating that now is cheaper than widening it later. The precedent
is `engagement_offers.decided_by`, null on exactly that cascade.

### `portal_accounts` — new, and the `UNIQUE` moves onto it

```sql
CREATE TABLE portal_accounts (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid text NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now()
);
```

The `UNIQUE` on `identity_uid` is **not dropped — it moves** to the table where one-per-human is
actually true. It was never wrong; it was on the wrong row.

RLS: one self-visibility policy, the same shape as `client_portal_users_self_visibility` (`00006`) —
visible only when **no** practice-tier and **no** client-tier session variable is set, and
`identity_uid` matches `app.current_identity_uid`. That guard is what makes *no `staffauth`-scoped
read may return a Portal Account's sibling links* structural rather than a convention.

### `client_portal_users` — becomes the join

- **Loses** `identity_uid`.
- **Gains** `portal_account_id uuid REFERENCES portal_accounts (id)`, nullable until accept — exactly
  as `identity_uid` is nullable today, so `client_portal_users_one_pending_per_client` is unaffected
  beyond keying its partial predicate on the new column.
- **Gains** `UNIQUE (portal_account_id, client_id)`: an account links each Practice's record at most
  once.
- Keeps `invite_token` and the pending state unchanged.

Every policy on the table that names `identity_uid` is rewritten against `portal_account_id`:
`client_portal_users_self_visibility` joins through `portal_accounts` to match
`app.current_identity_uid`; `client_portal_users_invite_insert` and `_invite_update` swap
`identity_uid IS NULL` for `portal_account_id IS NULL`; `client_portal_users_accept_update`'s
`WITH CHECK` confirms the update sets `portal_account_id` to the row the caller's own verified uid
resolves to.

### `clients` — gains `practice_id`, loses a policy

```sql
practice_id uuid NOT NULL REFERENCES practices (id)
```

This makes *a Client is a Practice's record* structural rather than conventional, and it changes two
policies:

- `clients_select` **collapses** from an `EXISTS` subquery against `engagements` to a plain column
  comparison, and becomes a single unqualified `clients_practice_visibility` policy — the same shape
  as `engagements_practice_visibility` (`00005`).
- `clients_insert` is **deleted outright.** It exists only to work around the chicken-and-egg problem
  of inserting a Client before the Engagement that makes it visible, and a `practice_id` column
  removes the problem: the value is known at insert time and equals the session variable, exactly as
  on `engagements`.

`client_portal_users_practice_visibility` and `_invite_insert` (`00026`) currently reach a Client
through an `EXISTS` against `engagements` for the same reason. They collapse the same way, onto
`clients.practice_id`.

### `engagements` — one new read policy, for the portal root

The portal root list has no `:engagementId`, so `app.current_client_id` is not yet set and neither
existing policy matches. One new read-only identity-tier policy, guarded like every other
self-visibility policy against the other tiers' session variables:

```sql
CREATE POLICY engagements_identity_visibility ON engagements
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND NULLIF(current_setting('app.current_client_id', true), '') IS NULL
        AND client_id IN (
            SELECT cpu.client_id
            FROM client_portal_users cpu
            JOIN portal_accounts pa ON pa.id = cpu.portal_account_id
            WHERE pa.identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
        )
    );
```

Per ADR-0006 the real refusal lives on the API read endpoint. This is the backstop, and without it
the one cross-Client read in the product would have no database-level guard at all.

### `engagement_offers` — two changes

- `due_date` **drops `NOT NULL`**, and is copied from the Engagement rather than typed.
- The Offer copy **gains the Engagement's `kind`**, printed on the Offer.

### What this schema deliberately does not add

- **No `visits.scheduled_at`.** The Visit type derivation needs a Visit date, `visits` has none
  (`00007` defers scheduling explicitly), and that column is **PR-G6**'s territory. This document
  asserts the derivation and names the missing column rather than letting whoever builds it find out.
- **No database constraint on the legal transitions.** The six directed moves and the role table are
  handler rules; the database enforces the `CHECK`, the immutability and the tenancy, and ADR-0006
  puts role refusals on the endpoint.
- **No `postpartum` anywhere.** The word survives only as a `kind` value and, once PR-G6 lands, as a
  derived Visit type.

## Rejected alternatives

**A status value for loss.** ADR-0005 argued the status set was short a terminal state for a
bereaved Client, and this document reaches the opposite conclusion from the same rule. The deciding
scenario is Nadia's: her doula keeps logging Visits *after* the loss. A single enum forces the
Practice to choose between recording the truth and keeping the record usable — the moment loss is
recorded as a status, the Engagement claims the work is over while bereavement Visits are still being
logged. Two facts hold both at once: *the work is ongoing* and *the pregnancy ended in loss*.

**A fourth status — `ended` or `closed` beside `completed`** — separating "finished as planned" from
"stopped early". Rejected: it re-creates the exact mistake removed by dropping `postpartum`, one
column carrying two vocabularies, and it adds a second value ADR-0005 would force a kind Client label
onto for a fact the Client must not read.

**Deriving kind from where the Client is in her pregnancy.** Rejected on the pregnant postpartum
client; see above. It was the best-looking option in the whole map and it is recorded here so it is
not rediscovered.

**Inferring kind from the Plan Template chosen (ADR-0001).** There is nothing to infer from.
ADR-0001 gives a Practice **one** Care Plan template and **one** Birth Plan template, shared across
every Engagement; there is no per-Engagement template choice. Taking this option means first
inventing template selection at creation time, so it is not the cheap structural option it looks
like.

**A Practice-configured service list instead of a kind enum.** Kind is not the price list; the
**Contract** is where what-was-sold-and-for-how-much belongs. Kind answers one narrow question —
*does this relationship include a birth we will attend?* — which has a small, stable answer set. A
Practice-configured menu would fit every Practice exactly and let Doula Cloud reason about none of
it, because the software cannot know what "Bereavement package" means. A real feature; a different
one.

**A `both` kind value.** Rejected; see above.

**A global `clients` row made genuinely global.** Rejected: it buys staff-side cross-Practice reuse,
and the lookup that delivers it tells one agency that a named woman is another agency's client.

**`completed` as absolutely terminal, with a replacement Engagement to fix a wrong completion.**
Rejected: it charges a Practice a Credit for its own typo, against the rule that an Engagement is
editable in place.

**Auto-completion on a recorded birth outcome.** Rejected: it closes Nadia's Engagement in the middle
of her bereavement care.

**The Contract being signed, or an Offer being accepted, as the `intake` → `active` trigger.** Signing
happens *inside* intake. Offer acceptance is the Practice staffing the work, which can happen before
the Client has agreed to anything, and would move her status on a decision she was not part of. The
first Visit being **recorded** was also rejected, in favour of **scheduled**: a signed Client with
three booked prenatals is not "getting started", and waiting for the visit to happen leaves her at
`intake` for weeks.

**Freezing `kind` in the database.** Rejected; see the ruling above.

## Open axes — named, not decided here

- **How an Engagement records events after the birth.** A neonatal death is a later event, not a
  birth-outcome value, and this document rules the model is columns on a row rather than a timeline.
  What that event's shape is cannot be said from here. If a timeline is where this goes, it arrives
  as its own decision.
- **Whether a contractor Doula's read *narrows* rather than stops when an Engagement completes** —
  keeping the Visits and Messages she produced while losing the live Client record. ADR-0008 named it
  and declined it; this document leaves it open on the same grounds. It is a sixth read state with no
  column in either table.
- **The ending-reason vocabulary**, pending the pilot group; `entered_in_error` in particular.
- **The interval of the *is this care finished?* prompt.**
- **Whether a Practice is billed for a mid-Engagement change of terms.** Platform billing, not
  Engagement lifecycle.
- **Transferring an Engagement between Practices.** Nothing supports it today under any model. This
  document neither blocks it nor needs it; it only makes it honest, since the receiving Practice
  necessarily gets its own Client record rather than inheriting the sending Practice's notes.
- **Shared reference data** — Birth Location and whatever else every Practice re-enters by hand
  ([#365](https://github.com/markgoho/doula-cloud/issues/365)). Not an exception to *no Client fact
  crosses a Practice*; a different category, on the usage-signal condition above.
- **What a postpartum Engagement's service actually looks like** — five to ten visits, weekly until
  six months, something else. Needs the pilot group. Kind is settled without it.

## What the build effort must charter

This document ends at a decision and a specified migration. The build is a separate effort, and this
is what it inherits, written down so the handoff is not implicit.

**The migration itself.** Everything in the schema section above, as one goose migration or a small
ordered set. Nothing in `api/db/migrations/` implements any of it today.

**The Engagement write surface.** [#253](https://github.com/markgoho/doula-cloud/issues/253) — the
status endpoint and the screen. Every one of its acceptance criteria now has an answer, and one needs
an edit when someone builds it: *every value in `engagement_status` is reachable* was written against
a four-value enum, and `postpartum` is not a status any more. The endpoint also owns the six-move
guard, the role table, the `engagement_events` write, and the ending-reason capture at completion.

**Auto-activation**, on the first Visit being scheduled, recording the person who scheduled it.

**The completion cascade** — closing open Offers as `withdrawn` with a null `decided_by`, and ending
granted Attachments with `ended_at`, per ADR-0008.

**Visit type**, and the column it needs. **PR-G6**: `visits` has no date, so the prenatal / birth /
postpartum derivation is uncomputable until scheduling lands.

**The Birth Plan suppression rule**, [#311](https://github.com/markgoho/doula-cloud/issues/311)
(CB-G5) — two suppressors, not one — and the create-time kind control,
[#308](https://github.com/markgoho/doula-cloud/issues/308) (CB-G2).

**The identity rework**: `portal_accounts`, the join, the accept-time 409 deleted
([#309](https://github.com/markgoho/doula-cloud/issues/309)), `clientauth.Middleware` resolving a uid
to a set, and the portal root list across Practices
([#310](https://github.com/markgoho/doula-cloud/issues/310),
[#312](https://github.com/markgoho/doula-cloud/issues/312)).

**Intake against `clients.practice_id`** — [#332](https://github.com/markgoho/doula-cloud/issues/332),
whose lookup-before-insert is **within-Practice only**, and which must obey the rule that editing a
Client's name corrects a spelling rather than substituting a human being.

**The Offer copy's new field and its now-optional due date** —
[#230](https://github.com/markgoho/doula-cloud/issues/230).

**The living-baby rule applied to the surfaces that exist** —
[#294](https://github.com/markgoho/doula-cloud/issues/294),
[#296](https://github.com/markgoho/doula-cloud/issues/296) — and the portal copy the register already
fixes ([#212](https://github.com/markgoho/doula-cloud/issues/212)).

**The *is this care finished?* prompt**, whose interval this document does not specify.

**One acceptance criterion elsewhere becomes wrong.**
[#293](https://github.com/markgoho/doula-cloud/issues/293) requires `engagement_status` to carry a
value that is true after a loss. Under this document it does not: the truthful state is `completed`
plus a `loss` outcome, and the status set *shrinks*. Whoever closes #293 rewrites that criterion.

## Cost

**The status column stops being the whole answer.** Anything that reads an Engagement's state now
reads up to four columns and a derived question, and the one place that used to be a single enum
comparison — the portal's `Detail.Status` — is joined by the living-baby rule on every surface that
mentions a baby. That is more to get right, and the rule is a convention no test can enforce
generically.

**A `CHECK` makes `unknown` a thing Practices must type.** A Client who vanished during intake cannot
be closed until someone records an outcome for a birth nobody witnessed. That is the constraint
working as designed, and it will still read as bureaucracy the first time a Practice meets it.

**Immutability arrives as the codebase's first trigger.** There are no triggers in
`api/db/migrations/` today. It brings a new failure mode — an update that raises rather than a policy
that returns zero rows — and a new session-variable door for the Owner-only hatch, which is one more
thing that must never be set by the wrong handler.

**A wrong `loss` is expensive to fix.** Owner-only, audited, and deliberately not an editing surface.
That is the price of closing the recycle-an-Engagement door, and it falls hardest on exactly the
record nobody wants to be handling twice.

**Three tables change shape at once.** `clients` gains a tenancy column, `client_portal_users` becomes
a join, and `portal_accounts` is new — which touches every policy on all three, plus the portal
middleware. Cheap now, because there is no production data; not cheap in January.

**The contractor cell will be wrong for someone.** It is ruled on an argument that survives either
classification outcome, and it is a permissions change to move. It is still a Practice deciding when
a contractor's Client relationship is over.

**Two logins, two people.** No merge in v1.
