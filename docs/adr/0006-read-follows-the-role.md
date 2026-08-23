# Read follows the role, and the read endpoint is the boundary

> **Superseded by [ADR-0008](0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md).**
> The table below is superseded in full — ADR-0008 carries a five-column version
> with employment type and attachment as real columns, plus a write-side table this
> ADR never had. The argument in this document stands: why read-follows-write and
> one-practice-one-view both lost, and why the Staff roster and Templates cells fall
> where they do. Read this ADR for the reasoning, ADR-0008 for the current table.

Doula Cloud's permission model checks who you are when you try to **change**
something and almost never when you try to **look** at something. `RequireOwner`
(`api/internal/staffauth/roles.go:63`) is the only role gate in the codebase, and
it sits on write handlers — invite, role assignment, session revocation, credit
purchase, Stripe Connect. `staffauth.Middleware` confirms only that a membership
exists; it never reads what roles that membership holds. Every `GET` behind it is
therefore open to any Staff member of the Practice.

Drafting the practice-side journey maps turned that up three times in one pass: an
Admin reads every filled Care Plan (`plans/template.go`), a Doula reads any
Engagement's Contract amount and Invoice history for Clients who are not hers, and
any Staff member reads the Practice's credit balance and purchase ledger
(`billing/balance.go:86`) while buying credits is correctly Owner-gated
(`billing/purchase.go:33`). Those are not three defects. They are one unwritten
rule, and this ADR writes it.

## Read follows the role

**The table in this section is superseded by [ADR-0008](0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md).**
The argument below it is not.

The Practice boundary is not part of this rule. Postgres row-level security
already fences Practice from Practice on both tiers, proved by a dedicated
cross-cutting test (`api/internal/rlsguardrail`). Everything below is **inside**
one Practice.

| | Owner | Admin | Doula (employee) | Doula (contractor) |
| --- | --- | --- | --- | --- |
| Engagements, Visits, Messages | all | all | all at the Practice | only those she is attached to |
| Plan Instances — Care Plan and Birth Plan | ✓ | ✓ | ✓ | on her Engagements |
| Contract — scope (Visit counts, dates, on-call terms) | ✓ | ✓ | ✓ | on her Engagements |
| Contract — money, and Invoice history | ✓ | ✓ | ✗ | ✓ on her Engagements |
| Plan Template and Contract Template | ✓ | ✓ | ✓ | ✓ |
| Credit balance and ledger | ✓ | ✓ | ✗ | ✗ |
| Staff roster | ✓ | ✓ | ✗ | ✗ |

Two alternatives were live and are recorded because both are defensible. **Read
follows write** — you may read only what some role of yours lets you change — is
the tidiest rule to state and to test, and it is wrong at the edge: a Doula must
read a Contract's scope, which she may never edit. **One Practice, one view** —
any Staff member reads everything, only writes are gated — is what the code does
today, and choosing it would have made today's behaviour deliberate. It was
rejected because it leaves the phrase "staff-only internal notes" with no work to
do and no rule for #207 to assert.

The Staff roster is readable by an **Admin** and not by a Doula. This one cell was
first written the other way and corrected: `CONTEXT.md` gives an Admin *scheduling*,
and assignment exists only per Visit, so booking a Visit means picking a Doula — an
Admin who cannot read the roster cannot do the job the glossary gives her. The
alternative was a second, narrower Doula list for the scheduling screen, which builds
a list to avoid showing a list, to protect a colleague's name from a colleague.
Changing the roster stays Owner-only. A Doula keeps `✗` because no journey has yet
given her a reason to need it, not because reading it would harm anyone; if one does,
this is a cheap cell to move.

Templates are readable by everyone because a Template holds no person's
information: it is the Practice's own blank stationery, and only the Owner may
edit it. Gating the read protects nothing and costs a rule to test.

"Staff-only" on the Care Plan means **not the Client**. It does not exclude a
non-doula Admin. That reading was genuinely open — `CONTEXT.md` could have been
drawing a line between the office and the birth room — and it is settled the other
way: one Practice is one team, and splitting the two plan types would add a rule
the pilot agency never asked for.

Every **employed** Doula reads every Engagement at the Practice. The pilot is one
agency of fourteen doulas who cover for each other, not fourteen solo practices
sharing a login; a doula picking up a birth at 3am must read the plan of a Client
who is not "hers". "Her own Engagements only" is deliberately not the rule for an
employee.

## Employment type is a second axis, and it is not a role

A **contractor** Doula is outside the business. She should not read the whole
Practice, and she *should* see the money on the work she takes, because she is
negotiating it. So one attribute governs two things at once: what a Doula reads
about money, and which Engagements she reads at all.

That attribute is **not** a fourth value in `practice_role`
(`('owner', 'office_manager', 'doula')`). A role says what you **do**; employment
says what you **are to the business**, and the two are orthogonal — an Admin can be
a contractor too. Folding them into one array would allow a membership holding
`{contractor}` and nothing else, which means nothing. Employment type is a separate
attribute of a membership, with values `employee` and `contractor`.

Credits stay owned by the **Practice**, and only an Owner buys them, whatever a
Doula's employment type. The contractor case does not change that: the Owner takes
the Engagement and offers it on. What a credit is actually spent on is still open
in code — `billing/billing.go:7` records that the consume path is unwired — and
that question belongs to the contractor Doula's journey map, not here.

## A membership with no roles stops existing

`InviteHandler` accepts a name and an email and inserts
`practice_memberships (…, roles) VALUES (…, '{}')`
(`api/internal/staffauth/invite.go:67`). Roles arrive later through an Owner-only
`PATCH`, which no screen calls (**RA-G2**). A zero-role membership is therefore the
only possible outcome of an invitation, and today it reads everything ungated.

Rather than write a read rule for that state, the state is abolished: **the
invitation carries roles**. A role-assignment UI is still owed, for changing roles
afterwards. The invite's missing roles are recorded as **RA-G8** on Renata's map.

## The refusal lives on the read endpoint

`csrf.Wrap` (`api/internal/csrf/wrap.go:22`) checks the `Origin` header on
**state-changing** requests only, and lets a request carrying no `Origin` header
through by design, because webhooks need that. `curl` sends no `Origin`. A `GET`
from a terminal with a valid `__session` cookie therefore reaches the API with
nothing in its way.

The person who does that is not a stranger. She is a Staff member with a real
login. An `Origin` check stops another *website* from using her session; it does
not stop *her*. So the API read endpoint is the boundary, and it is the only place
the rule is enforced. A SvelteKit `+page.ts` load guard changes what a browser
draws and can never be a boundary on its own; guards are added only where one
spares a person a dead-end screen, and the test plans assert the endpoint.

## What cannot be tested yet

The Owner, Admin, and employed-Doula columns are all expressible against the model
as it stands. The contractor column is not, and depends on two gaps: a membership
carries no employment type at all, and an Engagement carries no assigned Doula
(**RA-G4**), so "the Engagements she is attached to" is not a set the product can
compute. The practice-side test plan asserts the employed case, which is the only
case the model can currently produce; the contractor case lands on her journey map.

**A fifth state, added after this ADR was written.** This ADR left open how a
contractor Doula becomes attached to an Engagement, and
[Lena Vasquez's journey map](../journeys/contractor-doula.md) decided it: the
Practice **offers** her the job and her acceptance is what attaches her. That puts a
state in the model the table above does not describe — a Doula who has been offered
an Engagement and has not yet accepted — and she must read enough to decide (Client,
dates, on-call terms, fee) without the Practice being open to someone who has agreed
to nothing. The column is **LV-G7**, and it amends this table.

Stating a rule whose second half is not yet buildable is deliberate, and it follows
[ADR-0005](0005-one-context-client-register-at-the-ui-edge.md): state the true rule
and name the thing the model is missing, rather than bending the rule to fit what
the schema happens to hold.

## Cost

Read gating is new surface. Every read handler that serves gated data now needs a
role check, and `Roles()` (`api/internal/staffauth/roles.go:40`) is already exported
for it, but the checks are many and easy to forget — a new read endpoint is open
until someone remembers. That is the price of a rule with real content; the
alternative that needed no checks at all is the one rejected above.

The Contract read is the sharpest instance: it must be able to return scope without
money, which means the handler serves two shapes of the same record rather than one.
