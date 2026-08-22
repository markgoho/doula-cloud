# Hannah Sorensen — first-time pregnant client

- **Archetype**: First pregnancy, full birth Engagement start to finish
- **Pronouns**: she/her
- **Surface**: client portal (`/portal`)
- **Roles**: none — she is a **Client**, not Staff
- **Entry point**: a portal invitation from her Practice, accepted at
  `/portal/accept-invite`

## Who she is

Hannah is 29, 18 weeks along with her first, and reading everything. She hired Maya
after two interviews. She has a list of questions she is slightly embarrassed to ask her
OB and asks Maya instead, usually at night, usually from her phone.

She has never used a client portal for anything except her dentist, and she disliked it.

## Why she comes to Doula Cloud

To keep one thread with her doula, and to have her birth preferences written down
somewhere she can hand to the hospital.

## Primary journey

The full arc of an Engagement from her side: accept the portal invitation, sign the
Contract, read and react to her Birth Plan, message her doula through pregnancy, get
notified of new messages, and reach postpartum.

## Done looks like

She has a portal account, a signed Contract, a Birth Plan she has read and can print,
a message thread with real traffic in both directions, and an Engagement that has moved
through `intake` → `active` → `postpartum`.

## Watch for

- She is the push-notification persona. ADR-0002 says the push carries no content and
  wakes a fetch, so the assertion is at the in-app fetch level, not on a real device.
- Printing the Birth Plan for the hospital is a real, physical step. The print
  stylesheet is part of her journey, not a nicety.
- She sees the Birth Plan read-only and never sees the Care Plan. Confirm the Care Plan
  is genuinely absent from the portal.
- Her partner will want access and cannot have it. CONTEXT.md names this as a future
  extension of Client; capture it as a gap line, not a persona.
- There is no first-class due date on `engagements`. A Practice could record one as a
  Plan Template field (the field types are text, select, checkbox, and section header —
  there is no date type), but nothing in the app can then treat it as a date. Note what
  the portal shows her instead before calling this a gap.
