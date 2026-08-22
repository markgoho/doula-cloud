# Priya Raman — test plan

- **Journey**: [employed-doula.md](../journeys/employed-doula.md)
- **Persona**: [employed-doula.md](../personas/employed-doula.md)
- **A pass means**: an active membership holding the Doula role, an Engagement
  assigned to her opened, a Visit logged, messages exchanged — **and she never saw
  a screen she had no right to see.**

She is the negative-permission Persona, so half this plan is the
[Permission boundary](#permission-boundary) matrix, which is tester work rather
than anything Priya would do.

## Preconditions

- A Practice with an Owner, **two or more** Clients, at least one of them created
  by a different Staff member — otherwise stage 4 cannot fail the way it should.
- One Engagement carrying a filled Birth Plan and a Contract with an amount on it.
- Priya invited, accepted, and given `doula` with
  `PATCH /api/practices/{id}/staff/{staffId}/roles`; no screen does it (RA-G2).
- A phone, or a phone-sized viewport, for stage 6. It is the moment of truth and it
  is device-specific.

## Steps

### Stage 1 — Accept the invitation on the phone

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open the invite link on the device the text arrived on | The accept form renders on a small viewport | `manual` |
| 1.2 | Set email and password, press **Accept invite** | Membership created with `roles = '{}'` | `manual` |

### Stage 2 — Receive the Doula role

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Have the Owner set `doula` from a screen | No screen does this | `missing-feature (RA-G2)` |
| 2.1-a | Set it with `PATCH .../staff/{staffId}/roles` | Succeeds for an Owner | `manual` |
| 2.1-b | Compare every later step with and without the role | **No observable difference.** `doula` is read nowhere in the codebase; the role is decorative | `manual` |

### Stage 3 — Sign in and choose the Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Sign in at `/login` | `POST /api/session` succeeds and lands on `/practices/[practiceId]` | `automated (staff-login.e2e.ts)` |
| 3.2 | Choose Rooted Birth Collective | The picker lists her memberships | `manual` |
| 3.3 | Read the tiles | Invite, Staff, Plan Templates, Contract Template and Payments are hidden by `{#if roles.includes('owner')}`. **Clients and Billing remain** | `manual` |

### Stage 4 — Find her Clients

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/practices/[practiceId]/clients` | The list renders for a non-owner | `manual` |
| 4.2 | Count the rows | **Every Client in the Practice**, including other doulas'. The handler is Practice-scoped by design: "v1 has no restricted-visibility model" (PR-G1) | `manual` |
| 4.2-a | Look for a column saying which rows are hers | None — Engagements carry no Doula | `missing-feature (RA-G4)` |

### Stage 5 — Open the Engagement

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open an Engagement that is **not** hers | It opens. No read path is role-checked | `manual` |
| 5.2 | Read the Contract and Invoices sections | Contract prose, merge values **including the amount**, and the Invoice history all render. Per [ADR-0006](../adr/0006-read-follows-the-role.md) an employed Doula reads a Contract's scope but **not** its money (PR-G2) | `manual` |

### Stage 6 — Read the Birth Plan (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | On the phone, reach this Client's Birth Plan from a cold start, timed | It is a section partway down one long Engagement page holding Visits, Care Plan, Contract, Invoices and Messages | `manual` |
| 6.2 | Read the filled values | `GET .../plans/birth` renders the Plan Instance's snapshot | `manual` |
| 6.2-a | Deep-link straight to it, collapse what she does not need, or print it for hospital staff from her side | None of the three exist; the print stylesheet lives on the Client's portal view | `missing-feature (PR-G5)` |

### Stage 7 — Log a Visit

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Add a Visit | A row of her name and a creation timestamp | `manual` |
| 7.1-a | Record when the Visit was | No date or time anywhere | `missing-feature (MO-G1)` |
| 7.1-b | Record which of the three kinds it was — prenatal, birth, postpartum | A Visit carries no type | `missing-feature (PR-G6)` |
| 7.1-c | Record what was covered, for "what was I told last time" | A Visit carries no notes | `missing-feature (MO-G2)` |

### Stage 8 — Message the Client

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Send a message | `POST .../messages` appends to the one Engagement thread | `manual` |
| 8.2 | Have the Client reply | One continuous thread, in order, immutable | `manual` |
| 8.2-a | With the Client's thread open, deliver a push event | The tab refetches and shows the message; the push itself carries no content (ADR-0002) | `automated (push-notification.e2e.ts)` |

**Nadia crossing, settled.** Stages 7 and 8 are where an Engagement ending in loss
lands on the Doula's side — a bereavement Visit, and a thread that must not carry
on unchanged. [Her plan](loss-client.md) now exists and **neither stage changes**:
her 7.1 walks the bereavement Visit against **PR-G6**, **MO-G1** and **MO-G2**,
already given steps here, and the thread with no way to mark that its subject has
changed is hers to own (**NH-G7**). No step id here moves.

## Permission boundary

**Not stages.** Priya would never type these URLs. Each is walked as Priya, signed
in, holding `doula` and not `owner`. No owner-only route has a `+page.ts` load
guard, so the page always renders; what differs is whether the API hands over data
(PR-G4).

| Step | Route | Expected result | Mark |
| --- | --- | --- | --- |
| PR-B1 | `/practices/[practiceId]/staff` | **Holds.** The heading renders, then an error instead of the roster — the `GET` requires Owner | `manual` |
| PR-B2 | `/practices/[practiceId]/invite` | **Holds on submit.** The form renders; `POST .../invitations` refuses | `manual` |
| PR-B3 | `/practices/[practiceId]/settings/plan-templates` | **Reads through.** `GET` is ungated, `PUT` is Owner. Under ADR-0006 Templates are open to every Staff role, so this is a correct read and a refusal only at **Save** | `manual` |
| PR-B4 | `/practices/[practiceId]/settings/contract-template` | Same shape, same verdict as PR-B3 | `manual` |
| PR-B5 | `/practices/[practiceId]/settings/payments` | **Holds.** `POST .../connect` requires Owner | `manual` |
| PR-B6 | `/practices/[practiceId]/billing` | **Fails.** The balance and ledger take any Staff member, so she reads the Practice's spending; buying is correctly refused (DW-G4) | `manual` |

The pattern under test: protection sits on the write endpoint, never on the read.
No spec in the suite asserts a refusal by role at all, so every row here is new
ground.

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 2 |
| `manual` | 21 (six of them the permission boundary) |
| `missing-feature` | 6 (RA-G2, RA-G4, PR-G5, PR-G6, MO-G1, MO-G2) |

PR-G1, PR-G2, PR-G3 and PR-G4 are observed inside walkable steps (4.2, 5.2, 2.1-b,
PR-B3 to PR-B6) rather than given steps of their own: the step can be performed,
and what it hands back is the finding.

## Run log

Not yet run. First execution is
[#209](https://github.com/markgoho/doula-cloud/issues/209).
