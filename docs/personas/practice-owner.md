# Renata Alvarez — multi-doula practice owner

- **Archetype**: Owner of a Practice with several Staff
- **Pronouns**: she/her
- **Surface**: staff app
- **Roles**: **Owner** and **Doula** — she still takes two births a year, but runs the
  business
- **Entry point**: already has a Practice; signs in at `/login` and works from the
  Practice's Staff and Engagement screens

## Who she is

Renata built Rooted Birth Collective over nine years. Three employed doulas, one office
manager, roughly forty Engagements a year. She spends her week on assignment,
coverage, and money, and she is the person clients escalate to when something goes wrong.

Her real anxiety is coverage. If two Clients go into labor the same night, she needs to
know within a minute who is free and who is already at a birth.

## Why she comes to Doula Cloud

To see the whole Practice at once — who is assigned to whom, which Contracts are
unsigned, which Invoices are unpaid — without asking four people.

## Primary journey

Grow and run the roster: invite a new Doula, set that Doula's roles, put her on the
Engagements she will carry, then review the Practice's Engagements and money across
all Staff.

## Done looks like

A new Doula has accepted an invitation, holds the Doula role, and Renata can tell at
a glance which live Engagements are hers. Renata can see, in one place, every
Engagement in the Practice and its Contract and Invoice state.

## Watch for

- She needs a Practice-wide view, not a mine-only view. Any screen scoped to "my
  Engagements" fails her.
- Invite, accept, and role assignment span two people and two sessions. This journey
  cannot be walked by one browser context alone.
- **An Engagement has no assigned Doula.** `engagements` holds no staff column
  (`00005_client_engagement.sql`); only `visits.staff_id` exists, and a Visit carries
  no date. The need above is real; the capability is absent. See
  [her journey map](../journeys/practice-owner.md), RA-G4.
- Reassigning an Engagement from one Doula to another is a normal event for her (leave,
  illness, a Client who asks). Today `PATCH .../visits/{visitId}` reassigns a Visit,
  which is the only form of it that exists.
- She edits Plan Templates for the whole Practice, which changes what every new Plan
  Instance looks like — but must not alter plans already filled in.
