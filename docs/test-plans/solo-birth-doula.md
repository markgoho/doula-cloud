# Maya Okonkwo — test plan

- **Journey**: [solo-birth-doula.md](../journeys/solo-birth-doula.md)
- **Persona**: [solo-birth-doula.md](../personas/solo-birth-doula.md)
- **A pass means**: one Client carries a signed Contract, a filled Birth Plan the
  Client can read in the portal, at least one Visit, an open message thread, and an
  Invoice — reached by one person with no help.

## Preconditions

None. Maya's journey starts at a cold `/signup`, so this plan builds its own
fixture and is the only practice-side plan that needs no seeded state.

## Steps

### Stage 1 — Sign up and create the Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open `/signup` | The form shows Practice name, Your name, Email, Password | `manual` |
| 1.2 | Fill all four and press **Create Practice** | `POST /api/staff/signup` succeeds; a Practice, a Staff row, and a membership holding `owner`, `office_manager` and `doula` are created together | `manual` |
| 1.3 | Land on `/practices/[practiceId]` | `Welcome to {practice name}`, with all seven tiles including the five owner-only ones | `manual` |
| 1.3-a | Read the credit balance on **Billing** | `Credit balance: 3`, one `signup_bonus` ledger row of `+3` | `automated (billing.e2e.ts)` |

No spec signs up through the form: every spec provisions its Practice with
`POST /api/staff/signup` directly, so 1.1–1.3 are the untested seam under the
whole suite.

### Stage 2 — Judge the seeded Plan Templates

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Open `/practices/[practiceId]/settings/plan-templates` | The seeded Care Plan fields render, none of them blank | `automated (plan-templates.e2e.ts)` |
| 2.2 | Switch to the Birth Plan tab | A separate seeded field set; edits to one plan type do not appear on the other | `automated (plan-templates.e2e.ts)` |
| 2.3 | Add a field and **Save** | `PUT /api/practices/{id}/plan-templates/{planType}` succeeds for an Owner; `Saved.` appears and survives a reload | `automated (plan-templates.e2e.ts)` |
| 2.4 | Open `/practices/[practiceId]/settings/contract-template`, edit the prose, **Save**, reload | The seeded prose renders with `{{client_name}}` visible; the edit persists | `automated (contract-template.e2e.ts)` |

### Stage 3 — Add the first Client (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/practices/[practiceId]/clients/new` | A form of exactly two fields, Name and Email | `manual` |
| 3.1-a | Look for due date, phone, address, hospital, or intake notes | Nothing to enter; the paper folder cannot be transcribed | `missing-feature (MO-G3)` |
| 3.2 | Enter name and email, press **Add Client** | `POST /api/practices/{id}/clients` creates a Client **and** an Engagement at `intake`; one credit is consumed | `manual` |
| 3.2-a | Re-read the Billing balance | `Credit balance: 2` and a consumption ledger row | `manual` |
| 3.3 | Open the Engagement from the Clients list | The Engagement page shows Visits, Care Plan, Birth Plan, Contract, Invoices and Messages on one page | `automated (birth-plan.e2e.ts)` |
| 3.4 | Add a second and third Client, then attempt a fourth | The fourth returns `402` with "no credits remaining, ask a Practice Owner to buy more" — to Maya, who *is* the Owner | `manual` |
| 3.4-a | Follow that instruction and try to buy credits | Purchase cannot complete; no Stripe account exists | `manual` |

### Stage 4 — Fill the Care Plan and the Birth Plan

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Create and fill the Care Plan section, save | `PUT .../plans/care` persists; the values survive a reload | `manual` |
| 4.2 | Press **Create Birth Plan**, fill a field, **Save Birth Plan** | The value persists, and the Client's portal Birth Plan view shows the same text | `automated (birth-plan.e2e.ts)` |
| 4.2-a | Edit the Birth Plan **template** afterwards, then reopen the filled plan | The filled Plan Instance is unchanged — it snapshots the field definitions at creation | `manual` |

### Stage 5 — Contract and signature

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Send the portal invite from the Engagement page | `POST .../portal-invite` returns a token; **no email is sent** and the link must be delivered by hand | `manual` |
| 5.2 | Build the Contract from the Practice template | `POST .../contract` creates it at status `draft`, merge fields resolved | `manual` |
| 5.3 | Press **Send** | Status moves to `sent` | `manual` |
| 5.4 | As the Client, accept the invite and sign in | The accept → login path lands on `/portal/engagements/{engagementId}` | `automated (portal-invite-accept.e2e.ts)` |
| 5.4-a | Sign the Contract from the portal | `POST /api/portal/engagements/{id}/contract/sign` sets status `signed` | `manual` |
| 5.5 | Back as Maya, reload the Engagement | The Contract reads `signed`, without leaving the app | `manual` |
| 5.5-a | Look for a way to move the Engagement past `intake` | No update path exists anywhere; the status is fixed for the Engagement's whole life | `missing-feature (MO-G4)` |

### Stage 6 — Schedule Visits

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open the Visits section | A list and an add control | `manual` |
| 6.2 | Add a Visit | `POST .../visits` records a row of Staff name plus creation timestamp | `manual` |
| 6.2-a | Set a date and time for a future prenatal | No date field, no endpoint field, no column. A Visit can be recorded, never scheduled | `missing-feature (MO-G1)` |
| 6.2-b | Write what was covered on the Visit | Nowhere to put it | `missing-feature (MO-G2)` |

### Stage 7 — Get paid

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open `/practices/[practiceId]/settings/payments` and start Connect onboarding | `POST .../payments/connect` is owner-gated and passes for Maya; onboarding cannot complete — no Stripe account exists (MO-G8) | `manual` |
| 7.2 | Raise an Invoice against the signed Contract | `connectRequired` with `isOwner: true`; no Invoice is created | `manual` |
| 7.2-a | Compare the **Billing** and **Settings → Payments** screens | Two money screens, neither explaining that one buys credits from Doula Cloud and the other takes money from Clients | `manual` |

### Stage 8 — Message the Client

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Send a message from the Engagement page | `POST .../messages` appends to the one Engagement thread; it cannot be edited or deleted | `manual` |
| 8.2 | Reply from the portal and reload Maya's view | One continuous thread, both sides in order | `manual` |
| 8.2-a | With the portal thread open, deliver a push event | The open tab refetches and renders the new message | `automated (push-notification.e2e.ts)` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 9 |
| `manual` | 22 |
| `missing-feature` | 4 (MO-G1, MO-G2, MO-G3, MO-G4) |

MO-G5 to MO-G9 are experience-layer or infrastructure findings; they are observed
inside the steps above (3.4, 7.1, 7.2-a) rather than given steps of their own.

## Run log

Not yet run. First execution is
[#209](https://github.com/markgoho/doula-cloud/issues/209).
