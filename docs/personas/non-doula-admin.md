# Dee Whitlock — non-doula Admin

- **Archetype**: Office manager who never works a Visit
- **Pronouns**: they/them
- **Surface**: staff app, business screens
- **Roles**: **Admin** only — stored today as the enum value `office_manager`
- **Entry point**: an invitation from Renata, accepted at `/accept-invite`

## Who they are

Dee runs the back office at Rooted Birth Collective four days a week. They have never
attended a birth and do not want to. They handle intake calls, get Contracts out and
signed, chase Invoices, and keep the calendar honest. They are the first person a
prospective Client speaks to and the last person an unpaid Invoice belongs to.

They are the fastest typist in the Practice and live in the app all day.

## Why they come to Doula Cloud

To do the paperwork end of an Engagement without waiting on a doula who is at a birth.

## Primary journey

Take a new Client from first call to signed and billed: create the Client and
Engagement, assign a Doula, send the Contract, track signature, raise the Invoice, and
record the Payment.

## Done looks like

An Engagement exists with a Doula assigned, a signed Contract, and an Invoice with a
recorded Payment — all without Dee ever opening a Care Plan or logging a Visit.

## Watch for

- **The permission model is binary owner / non-owner, so the Admin role grants nothing.**
  `owner` is checked in four places (`payments/invoice.go`, `plans/template.go`,
  `contracts/template.go`, `staffauth/roles.go`) and the app gates on `'owner'`
  client-side. Neither `office_manager` nor `doula` is read anywhere. Dee is therefore
  indistinguishable from any other non-owner Staff member today. That is a first-order
  finding for the practice-side test plan, not a naming quibble.
- **Admin is the settled word** (CONTEXT.md), and the code has not caught up: the enum
  value is still `office_manager`, and the Staff list renders roles as raw strings
  (`member.roles.join(', ')`), so Dee shows up on screen as `office_manager`. Journey
  maps, test plans, and gap-issue titles all say **Admin**.
- Dee is reachable as a distinct persona because the invite path grants zero roles
  (`invite.go` inserts `'{}'`), and an Owner then assigns a subset. Signup is the
  opposite: it grants all three at once. Only the invite route produces an Admin.
- Their permissions are the mirror image of Priya's: business screens yes, care records
  arguably no. Two separate questions, and only one is open:
  - **Editing Plan Templates is already settled** — `plans/template.go:220` gates on
    `owner`, so Dee cannot. Assert it; do not re-litigate it.
  - **Reading a Client's filled Care Plan or Birth Plan is genuinely undecided.** No
    role check guards the Plan Instance read path. This journey must answer it.
- They act on Engagements they are not assigned to, so any mine-only scoping breaks them.
- Payment recording is where the missing Stripe account bites hardest.
