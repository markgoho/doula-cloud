# Lena Vasquez — contractor doula

- **Archetype**: Independent doula who takes contracted work from Practices she does
  not belong to
- **Pronouns**: she/her
- **Surface**: staff app, at more than one Practice
- **Roles**: **Doula** only — no Owner, no Admin
- **Employment type**: **contractor** (see `CONTEXT.md`, "Employment type")
- **Entry point**: an emailed invitation to Rooted Birth Collective, accepted on an
  account she already holds at a different Practice — after which individual jobs are
  offered to her, one at a time

## Who she is

Lena is nine years in and runs her own book. She is not employed by anybody. Two or
three times a year she takes overflow work from Rooted Birth Collective: she is
offered a Client, the dates, and a fee, and she says yes or no. She also takes work
from a second agency across town, and carries her own private Clients besides.

She is not "one of Renata's doulas". She is a business talking to another business,
and the difference shows up in what she needs to know: the money on the job, and only
the job.

## Why she comes to Doula Cloud

To carry a Practice's Client the way that Practice expects, without being handed the
Practice's whole book — and to be able to check, months later, exactly what she agreed
to and what she is owed.

## Primary journey

From an offered job to a finished, paid one: join the agency, weigh a job she is
offered and take it, find it again afterwards, confirm the terms and the fee are what
she agreed, do the care work, and close the job out.

## Done looks like

The Engagement she took is finished, she can point to what she agreed and what she was
paid, and she never saw a Client who was not hers.

## Watch for

- She is the second negative-permission Persona, and the tighter one. Priya Raman
  ([employed-doula.md](employed-doula.md)) may read every Engagement at the Practice by
  design; Lena may not. Where Priya is refused only money, Lena is refused the
  Practice.
- **Nothing distinguishes her from Priya.** `practice_memberships` carries
  `practice_id`, `staff_id`, `roles`, `created_at` and nothing else
  (`api/db/migrations/00002_practice_staff_tenancy.sql:29`). There is no employment
  type anywhere in the schema, so neither half of her read rule — the money she should
  see, and the Engagements she should not — is expressible today.
- **She already has an account, and the invite route assumes she does not.**
  `InviteHandler` always inserts a fresh `staff` row
  (`api/internal/staffauth/invite.go:58`), and acceptance claims it by writing her
  identity onto it (`api/internal/staffauth/accept.go:101`) — but `staff.identity_uid`
  is `UNIQUE` (`00002_practice_staff_tenancy.sql:21`). The same migration's own comment
  (lines 16–18) says a person may work at more than one Practice via separate
  memberships.
- **The work she takes is not recorded as hers.** `engagements` has no staff column
  (`00005_client_engagement.sql:20`), so "the Engagement she is attached to" — the
  phrase her whole read rule rests on ([ADR-0006](../adr/0006-read-follows-the-role.md))
  — names nothing the app can store.
- The Contract read returns prose, merge fields and values in one object with no role
  check (`api/internal/contracts/contract.go:136`). Her rule and Priya's are opposite
  halves of the same missing split.
- **What the Practice owes *her* is absent.** `invoices` rows point at a `practice_id`
  and a `contract_id` (`00024_invoices.sql:16`) — a Practice billing a Client. No table
  records a payment from a Practice to a doula.
- **She may refuse.** Nothing in the schema expresses a job being offered, taken, or
  turned down — the only assignment the product has is a Visit's `staff_id`
  (`00007_visit.sql:8`), which is a bare fact with no one agreeing to it.
- A contracted job ends. Nothing in the schema expresses an attachment ending, so
  whatever grants her a read has no observed way to stop granting it.
- She works across Practices in one session and will land on the Practice picker more
  often than any other Persona.
