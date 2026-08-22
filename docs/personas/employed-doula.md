# Priya Raman — employed doula

- **Archetype**: Staff who works Engagements and nothing else
- **Pronouns**: she/her
- **Surface**: staff app, narrowed to her own Engagements
- **Roles**: `doula` only — no `owner`, no `office_manager`
- **Entry point**: an emailed invitation from Renata, accepted at `/accept-invite`

## Who she is

Priya is two years in, employed by Rooted Birth Collective rather than running her own
book. She works three or four Engagements at a time around a part-time day job. She
does the care work and none of the business: she never sets a price, never sends an
Invoice, and never sees another doula's Clients.

She works from a phone far more than a laptop, often in a hospital corridor.

## Why she comes to Doula Cloud

To know where she has to be, what this Client wants, and what she was told last time —
without texting Renata.

## Primary journey

From invitation to caring for an assigned Client: accept the invite, sign in, open the
Engagement assigned to her, read the Birth Plan, log Visits, and message the Client.

## Done looks like

She has an active membership with the `doula` role, has opened an Engagement assigned to
her, logged a Visit, and exchanged messages with the Client. She never saw a screen she
had no right to see.

## Watch for

- She is the negative-permission persona. Every owner-only and admin-only surface —
  Staff management, billing, Plan Template settings, invitations — must be absent for
  her, not merely unlinked.
- **Expect this to fail.** Only the `'owner'` role is enforced anywhere in the app or
  the API; the `doula` role is never checked. Whatever an owner can reach that is not
  explicitly owner-gated, Priya can probably reach too, by URL if not by link.
- Her scope is her Engagements. Confirm whether she can see Practice Engagements
  assigned to other doulas; if she can, that is a finding.
- Invitation acceptance is her first impression, and it happens on whatever device the
  email opened on.
- She reads Care Plans and Birth Plans far more often than she writes them.
