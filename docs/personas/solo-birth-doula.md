# Maya Okonkwo — solo birth doula

- **Archetype**: Owner + Admin + Doula in one person
- **Pronouns**: she/her
- **Surface**: staff app
- **Roles**: **Owner**, **Admin**, and **Doula** — all three on one membership
- **Entry point**: cold signup at `/signup`, then creates her own Practice

## Who she is

Maya is 34, certified six years, and works alone out of a spare room in Providence.
She carries four to six birth Clients at a time and turns down more. Her current system
is a paper folder per Client, a shared calendar, and a phone that does everything else.
She lost an intake form last spring and has not trusted the folders since.

She is not afraid of software, but she is on call. Anything that needs a laptop and
twenty quiet minutes will not get done.

## Why she comes to Doula Cloud

One place that holds the Client record, the Contract, the Birth Plan, and the messages,
so that a 3 a.m. call does not start with hunting for a folder. She is also tired of
chasing payment by text.

## Primary journey

From an empty account to one live Engagement: create the Practice, add a Client, send
and countersign the Contract, fill the Care Plan and Birth Plan, get her Visits into
the app, and invoice.

## Done looks like

A Client exists with a signed Contract, a completed Birth Plan the Client can read in
the portal, at least one Visit, an open message thread, and an Invoice she can point at.
She did all of it without asking anyone for help.

## Watch for

- She holds three roles at once. Nothing in the UI should make her switch context or
  re-authenticate to move between Owner work, Admin work, and Doula work.
- **She is not a test of role separation.** Signup grants all three roles in one
  statement (`signup.go:152`), so every permission boundary rides on Renata's invite
  flow instead. Do not let Maya's journey stand in for Priya's or Dee's.
- Signup, Practice creation, and the seeded default Plan Templates are her first three
  minutes. If the seeded templates are wrong for her, she must be able to edit them
  before she has any Client.
- She is the only Staff member, so wherever the app asks which Doula, the answer must
  default to her. Today the only such place is a Visit's `staffId`.
- **A Visit cannot be scheduled.** `visits` is `(engagement_id, staff_id, created_at)`
  — no date, no type, no notes (`00007_visit.sql`). Her Visits can be recorded after
  the fact and nothing more.
- **Signup grants 3 free credits** (`signup.go:144`) and creating a Client consumes one
  (`billing.ConsumeCredit`). She carries four to six Clients, so she hits a paywall on
  her fourth — and no Stripe account exists to take her money.
- Billing setup (Stripe Connect) sits between her and getting paid, and no Stripe
  account exists yet — expect this leg to be blocked rather than broken.
