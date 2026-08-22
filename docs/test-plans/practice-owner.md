# Renata Alvarez — test plan

- **Journey**: [practice-owner.md](../journeys/practice-owner.md)
- **Persona**: [practice-owner.md](../personas/practice-owner.md)
- **A pass means**: a new Doula has accepted an invitation, holds the Doula role,
  and appears as the assigned Doula on a live Engagement; and one screen shows
  every Engagement in the Practice with its Contract and Invoice state.

## Preconditions

- A Practice with Renata as Owner, and at least two Clients with Engagements.
  Build it through Maya's stages 1 and 3, or provision it with
  `POST /api/staff/signup` plus `POST /api/practices/{id}/clients` as the specs do.
- A second Identity Platform account for the invitee, and a way to read the
  invitation token (it is printed on the invite screen; no email is sent).

## Steps

### Stage 1 — Sign in and choose the Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Sign in at `/login` | `POST /api/session` sets `__session`; the browser lands on `/practices/[practiceId]` | `automated (staff-login.e2e.ts)` |
| 1.2 | Choose Rooted Birth Collective from her memberships | The picker lists every Practice she is a member of | `manual` |
| 1.3 | Read the tiles | All seven render: the five owner-only ones (Invite, Staff, Plan Templates, Contract Template, Payments) plus Clients and Billing | `manual` |

The membership picker (1.2) is exercised by no spec — every spec's Staff member
belongs to exactly one Practice, so the multi-membership path is untested here and
is Lena's normal case.

### Stage 2 — Invite a new Doula

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Open `/practices/[practiceId]/invite` | The form renders for an Owner | `manual` |
| 2.2 | Enter a name and email, press **Send invite** | `POST /api/practices/{id}/invitations` succeeds and the screen **prints a link**; no email leaves the system | `manual` |
| 2.2-a | Check the invitee's inbox | Nothing arrives. No invitation email is sent by anything | `missing-feature (RA-G1)` |
| 2.3 | Deliver the link out of band | The invitee receives a raw URL by text or Renata's own mail client, which they cannot verify is genuine | `manual` |
| 2.4 | Have the invitee accept at `/accept-invite` | `POST /api/staff/accept-invite` creates the membership with `roles = '{}'` — a zero-role member is the only possible outcome of inviting anyone | `manual` |
| 2.4-a | Look for a roles control on the invite form | The invitation carries no roles at all | `missing-feature (RA-G8)` |

### Stage 3 — Set the new Doula's roles

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/practices/[practiceId]/staff` | The roster loads; the `GET` is owner-gated and passes for Renata | `manual` |
| 3.2 | Read the Roles column | Raw enum strings render — an Admin shows as `office_manager`, the word `CONTEXT.md` rules out | `manual` |
| 3.3 | Change a member's roles from the screen | No control exists. The only row action is **End sessions everywhere** | `missing-feature (RA-G2)` |
| 3.3-a | Call `PATCH /api/practices/{id}/staff/{staffId}/roles` directly with `["doula"]` | Succeeds for an Owner. **This is the only way to build a roster**, and every later plan depends on it as fixture setup | `manual` |

### Stage 4 — Assign the Doula to Engagements

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open an Engagement from the Clients list | The single-page Engagement view renders | `automated (birth-plan.e2e.ts)` |
| 4.2 | Look for an assignment control | No field, no endpoint, no screen. An Engagement carries no Doula | `missing-feature (RA-G4)` |
| 4.3 | Add a Visit naming the new Doula as `staffId` | `POST .../visits` succeeds — assignment exists at Visit level only, on a record with no date | `manual` |

### Stage 5 — Reassign when someone is sick

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | `PATCH .../visits/{visitId}` with a new `staffId` | The Visit's Staff member changes; nothing dated moves, because nothing is dated | `manual` |

### Stage 6 — See the whole Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open `/practices/[practiceId]/clients` | The list renders and each Client links to their Engagement | `automated (birth-plan.e2e.ts)` |
| 6.1-a | Create a Client as a *second* Staff member, then reload as Renata | It appears: the handler returns every Client with an Engagement at the Practice "regardless of which Staff member created it". **This half of her requirement passes** | `manual` |
| 6.1-b | Read the columns | Name and Status only — and Status is `intake` on every row forever (MO-G4) | `manual` |
| 6.2 | Learn each Engagement's Contract and Invoice state | Reachable only by opening every Engagement in turn | `manual` |
| 6.2-a | Look for a roll-up of Contract state, Invoice state, or covering Doula | There is none at any level above one Engagement | `missing-feature (RA-G6)` |

### Stage 7 — See the money across all Staff

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open `/practices/[practiceId]/billing` | Credit balance and purchase ledger render — Doula Cloud's own billing, not Client money | `automated (billing.e2e.ts)` |
| 7.2 | Look for unpaid Client Invoices | No Practice-wide Invoice list and no unpaid view exist | `missing-feature (RA-G7)` |

### Stage 8 — Coverage, at 2 a.m. (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Sign in on a phone | The practice screen renders on a small viewport | `manual` |
| 8.2 | Find who is free tonight | No availability, on-call, or coverage surface exists — and dateless Visits (MO-G1) mean the data one would read does not exist either | `missing-feature (RA-G5)` |

### Stage 9 — Edit Plan Templates for the whole Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 9.1 | Add a field to the Birth Plan template and save | `PUT .../plan-templates/{planType}` succeeds for an Owner and persists | `automated (plan-templates.e2e.ts)` |
| 9.2 | Reopen an already-filled Birth Plan | Unchanged — the Plan Instance snapshots the field definitions at creation. **Passes** | `manual` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 5 |
| `manual` | 16 |
| `missing-feature` | 7 (RA-G1, RA-G2, RA-G4, RA-G5, RA-G6, RA-G7, RA-G8) |

RA-G3 is observed at 3.2 rather than given a step: the screen renders, so the step
is walkable — what fails is the word it prints.

## Run log

Not yet run. First execution is
[#209](https://github.com/markgoho/doula-cloud/issues/209).
