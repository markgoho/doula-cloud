# Dee Whitlock — non-doula Admin

- **Archetype**: Office manager who never works a Visit
- **Pronouns**: they/them
- **Surface**: staff app, business screens
- **Roles**: `office_manager` only — the enum value CONTEXT.md calls **Admin**
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

- **The `office_manager` role is not implemented as a permission.** The app branches on
  `'owner'` only; `office_manager` and `doula` are never read client-side, and API-side
  `office_manager` appears only in `staffauth`'s valid-role list and in the signup grant.
  Dee is therefore indistinguishable from any other non-owner Staff member today. That
  is a first-order finding for the practice-side test plan, not a naming quibble.
- The Staff list renders roles as raw strings (`member.roles.join(', ')`), so Dee shows
  up as `office_manager` on screen.
- The naming mismatch is unresolved: the enum is `office_manager`, CONTEXT.md says
  **Admin**. Journey maps, test plans, and gap-issue titles must all pick the same side.
- Their permissions are the mirror image of Priya's: business screens yes, care records
  arguably no. Whether an `office_manager` should read a Care Plan or a Birth Plan is
  an open question this journey must answer, not assume.
- They act on Engagements they are not assigned to, so any mine-only scoping breaks them.
- Payment recording is where the missing Stripe account bites hardest.
