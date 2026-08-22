# Camille Boyd — returning postpartum-only client

- **Archetype**: Second baby, hires the same Practice for postpartum support only
- **Pronouns**: she/her
- **Surface**: client portal (`/portal`)
- **Roles**: none — she is a **Client** with a prior, completed Engagement
- **Entry point**: signs in to the portal she already has, or is invited into a second
  Engagement at the same Practice

## Who she is

Camille used Rooted Birth Collective for her first birth two years ago and liked Priya.
This time she has the birth handled — a midwife, a partner who took leave — and what she
wants is someone in the house for the first six weeks. Nights, feeding, and a person who
will tell her the baby is fine.

She is not a beginner. She wants to skip the parts she has already done.

## Why she comes back

She already trusts this Practice. Re-explaining herself from scratch would be the fastest
way to lose her.

## Primary journey

A second Engagement with the same Practice, postpartum only: her old Engagement is
`completed`, a new one opens, and it starts at postpartum work rather than at intake and
birth planning.

## Done looks like

Camille has two Engagements at the same Practice — one `completed`, one live and doing
postpartum work — reachable from one portal account, with the older one still readable
as a record.

## Watch for

- **`clients` is a global table with no `practice_id`**, so one person holding two
  Engagements at one Practice is supported by the schema. Confirm the portal actually
  shows both, and that she does not need a second account.
- **There is no Engagement type or kind column.** `engagement_status` is
  `intake | active | postpartum | completed`, so "postpartum-only" cannot be declared —
  it can only be approximated by moving status forward. CONTEXT.md claims Engagement is
  "deliberately generic so it fits both birth-doula and postpartum-doula work"; this
  journey is the test of that claim, and a likely gap.
- A Birth Plan makes no sense for her. Check whether the app insists on one anyway.
- Whether her old messages and plans carry across or stay locked to the old Engagement
  is an open question — one continuous thread per Engagement means two threads.
