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
| 3.4-a | Follow that instruction and try to buy credits | Stripe Checkout opens for the chosen quantity; paying credits the ledger | `manual` |

**The doula's card statement says `DOULA.CLOU`.** Found on Dee's walk
([#236](https://github.com/markgoho/doula-cloud/issues/236)) while tracing the
Client-facing `DOULA.CLOU` that `7261a59` fixed. This is the *other* half of that
bug and it is **not** fixed: the credits Checkout session sets no descriptor
(`billing/purchase.go`), so the charge falls back to the platform account's, and
the platform's `statement_descriptor` is `DOULA.CLOUD` — 11 characters. Stripe caps
a card prefix at 10 and truncates, giving `DOULA.CLOU`. The Client never sees it,
because a connected account now carries its own `display_name`; the **doula** sees
it every time she buys credits.

**No code change fixes this** — it is one Dashboard field, so it is a launch
checklist item rather than a `journey-gap`. Set the *shortened descriptor* (the
prefix) on the Doula Cloud account to `DoulaCloud`, which is exactly 10 characters
and drops the period that makes the truncation read as a cut-off URL. It must be
set on **both** the sandbox and the live account; Stripe only exposes it in the
Dashboard (`settings/business-details`), not the API. Verified against the Sandbox
on 2026-08-22.


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
| 5.2 | Build the Contract from the Practice template | `POST .../contract` creates it at status `draft` with the template prose snapshotted — and **no merge field resolved**: every value comes back empty and must be typed by hand, `practice_name` and `client_name` included (MO-G10) | `manual` |
| 5.3 | Press **Send** | Status moves to `sent` | `manual` |
| 5.4 | As the Client, accept the invite and sign in | The accept → login path lands on `/portal/engagements/{engagementId}` | `automated (portal-invite-accept.e2e.ts)` |
| 5.4-a | Sign the Contract from the portal | `POST /api/portal/engagements/{id}/contract/sign` renders the signed PDF, puts it in the object store, **then** sets status `signed` | `manual` |
| 5.5 | Back as Maya, reload the Engagement | The Contract reads `signed`, without leaving the app, and a **Void Contract** action appears | `manual` |
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
| 7.1 | Open `/practices/[practiceId]/settings/payments` and start Connect onboarding | `POST .../payments/connect` is owner-gated and passes for Maya, returns a real v2 Account Link, and the hosted flow completes to an active account (#247, walked 2026-08-22) | `manual` |
| 7.2 | Raise an Invoice against the signed Contract | An Invoice is created on the connected account and is payable (#247, walked 2026-08-22) | `manual` |
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
| `manual` | 19 |
| `blocked` | 0 |
| `missing-feature` | 4 (MO-G1, MO-G2, MO-G3, MO-G4) |

MO-G5 to MO-G9 are experience-layer or infrastructure findings; they are observed
inside the steps above (3.4, 7.1, 7.2-a) rather than given steps of their own.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 1.3-a | `billing.e2e.ts` | pass |
| 2.1 | `plan-templates.e2e.ts` | pass |
| 2.2 | `plan-templates.e2e.ts` | pass |
| 2.3 | `plan-templates.e2e.ts` | pass |
| 2.4 | `contract-template.e2e.ts` | pass |
| 3.3 | `birth-plan.e2e.ts` | pass |
| 4.2 | `birth-plan.e2e.ts` | pass |
| 5.4 | `portal-invite-accept.e2e.ts` | pass |
| 8.2-a | `push-notification.e2e.ts` | pass |

**9 automated steps: all pass.**

The `manual`, `blocked` and `missing-feature` steps are **not walked yet**.
That is [#234](https://github.com/markgoho/doula-cloud/issues/234).

### 2026-08-22 — manual walk ([#234](https://github.com/markgoho/doula-cloud/issues/234))

`bun run dev:full` in `app/`, walked in a desktop browser at 1280x900 as Maya
Okonkwo, with a second 390x844 context for the Client. Preconditions: none, as
the plan says — signed up from `/signup` with no fixture, twice (one pass for
stages 1–8, a second to observe 4.2-a's template edit, 7.2's Invoice with an
amount, and stage 8). The 9 `automated` steps were **not** re-run.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | `Sign up your Practice`, four fields — `Practice name`, `Your name`, `Email`, `Password` — and **Create Practice**. Nothing else on the screen |
| 1.2 | `manual` | as expected | `POST /api/staff/signup` -> `201 {"staffId":…,"practiceId":…}`. One request, no confirmation step |
| 1.3 | `manual` | as expected | `Welcome to {practice name}` and seven links: Clients, Billing, Invite a Staff member, Staff, Plan Templates, Contract Template, Payments, plus `Sign out` |
| 3.1 | `manual` | as expected | `Add a Client`: exactly two fields, `Their name` and `Their email` |
| 3.1-a | `missing-feature (MO-G3)` | as expected | Confirmed unwalkable. No due date, phone, address, hospital, partner, or notes field exists on the form or anywhere behind it. The paper folder cannot be transcribed |
| 3.2 | `manual` | as expected | `201 {"clientId":…,"engagementId":…,"status":"intake"}`, and the browser lands **straight on the Engagement page** — the Clients list is never passed through, as Tasha's walk also found |
| 3.2-a | `manual` | as expected | `Credit balance: 2`, ledger `consumption / -1` above `signup_bonus / +3`. Nothing on the Add Client screen had said a credit would be spent |
| 3.4 | `manual` | as expected | Clients 2 and 3 -> `201`. The fourth -> `402 no credits remaining, ask a Practice Owner to buy more`, printed verbatim under the form. Maya is the Owner it tells her to ask |
| 3.4-a | `manual` (was `blocked`) | **better than the plan claimed** | The Sandbox exists now ([#242](https://github.com/markgoho/doula-cloud/issues/242)), and the leg runs end to end. **Buy credits** with `Quantity 2` -> Stripe Checkout, `$10.00`, `Engagement credit`, `Qty 2, $5.00 each`. Paid with `4242 4242 4242 4242` -> back to `?checkout=success`, `Credit balance: 5`, ledger row `purchase / +2`. **The webhook is what credited it**: four events were delivered and all answered `200`, but only `checkout.session.completed` was recorded in `stripe_webhook_events` — `payment_intent.created`, `payment_intent.succeeded` and `charge.updated` were acknowledged and ignored, so the single purchase credited once. The 500 this step used to answer (`You did not provide an API key`) is gone |
| 4.1 | `manual` | as expected | **Create Care Plan** -> `201` with the seeded field set (Support People, pain management, requests, backup-doula checkbox). Filled, **Save Care Plan** -> `PUT … 200`, and the value survived a reload |
| 4.2-a | `manual` | as expected | Added a field to the **Birth Plan** template afterwards (`PUT .../plan-templates/birth_plan` -> `200`, `Saved.`), reopened the filled plan: the new field is **absent**. The Plan Instance snapshot holds, exactly as claimed |
| 5.1 | `manual` | as expected | `201 {"clientPortalUserId":…,"inviteToken":…}` and the screen says it plainly: `Invited. There is no email sending yet, so share this link with them directly:` followed by the raw URL in a `<code>` block. Maya must copy it out by hand (**MO-G6** territory) |
| 5.2 | `manual` | **falsified** | `201`, status `draft`, prose snapshotted — but **no merge field is resolved**. The form shows six empty text inputs, `Practice name` and `Client name` among them, and the create handler sets `Values: MergeFieldValues{}` unconditionally (`api/internal/contracts/contract.go:124`). The product knows the Practice name and the Client name and asks her to retype both. New gap **MO-G10** on the journey map |
| 5.3 | `manual` | as expected, with a second finding | **Send Contract** -> `200`, status `sent`. It sent with **every merge field blank** — no validation, no warning — and the Client's portal then rendered `This agreement is between  and  for doula services.` Folded into **MO-G10** |
| 5.4-a | `blocked` (was `manual`) | **re-marked** | `POST /api/portal/engagements/{id}/contract/sign` -> `500 internal error`. Not a missing feature: signing renders the PDF and calls `store.Put` **before** the status write (`api/internal/contracts/sign.go:85-89`), and the walking stack points the object store at an unreachable host on purpose (`app/e2e/stack.ts:220`, `STORAGE_EMULATOR_HOST: 'storage-emulator-disabled.invalid:1'`). The bucket itself is real in the deployed service (`.github/workflows/ci.yml:516`). Infrastructure absent from the stack, not from the product |
| 5.5 | `blocked` (was `manual`) | **re-marked** | Unreachable while 5.4-a is. Maya's Engagement reads `Status: sent` |
| 5.5-a | `missing-feature (MO-G4)` | as expected | Confirmed unwalkable. Every control on the Engagement page was enumerated: nothing anywhere reads or writes the Engagement's status. It shows `intake` in a read-only description list and stays there |
| 6.1 | `manual` | as expected | A `Visits` heading, an **Add a Visit** button, and a table of `Staff` / `Date` / `Reassign`, empty message `No Visits yet.` |
| 6.2 | `manual` | as expected | `201 {"visitId":…,"staffId":…}`, one row: `Maya Okonkwo / 8/22/2026`. No prompt, no form — the button *is* the Visit |
| 6.2-a | `missing-feature (MO-G1)` | as expected | Confirmed unwalkable. The only input the Visits section carries is `Reassign to Staff id`, a free-text box for a UUID. No date, no time. The `Date` column renders `visit.createdAt` |
| 6.2-b | `missing-feature (MO-G2)` | as expected | Confirmed unwalkable. No notes field anywhere in the section |
| 7.1 | `manual` (was `blocked`) | **better than the plan claimed** | Walked end to end on 2026-08-22 (#247). `Payments` reads `Not connected` with `Connect Stripe so Clients can pay their invoices.`; the button opens a real v2 Account Link, and Stripe's hosted flow (business type, personal details, business details, bank, payouts, public details, Radar, review) completes and returns to `?connect=return`. The screen then read `Awaiting Stripe review` — `card_payments: restricted`, `payouts: pending`, **no** requirements outstanding — and **hid** the onboarding button, since there was nothing to supply. Once the capability went active, `capability_status_updated` landed and the screen read `Active` / `Clients can pay their invoices and payouts reach your bank.` **The webhook is what wrote it**: `practices.stripe_connect_status_event_id` holds the delivering event id, not a poll. The old 500 here was the Stripe-401, and is gone |
| 7.2 | `manual` (was `blocked`) | **better than the plan claimed** | Walked 2026-08-22 (#247), on a Practice whose Connect account is active. `Amount (USD)` `1800` -> **Create Invoice** -> the Invoices section shows `$1,800.00 — Open`, and Stripe holds a finalized Invoice on the connected account. Paid on the hosted page with `4242 4242 4242 4242` -> `Invoice paid`, and `invoice.paid` flipped `invoices.status` to `paid` and created exactly one `payments` row for `180000`. **Two bugs fell out of this step**: the Client's invoice read `From DOULA.CLOU` (no `display_name` on the v2 Account, so Stripe fell back to the statement descriptor), and every `payments` row was written with an empty `stripe_payment_reference` (`payment_intent` no longer exists on an Invoice under `2026-07-29.dahlia`). Both fixed. The `connectRequired` gate this row used to record is still correct for a Practice that has *not* connected — it is just no longer what Maya hits |
| 7.2-a | `manual` | as expected | Side by side: **Billing** is `Credit balance: 0`, a ledger, `Quantity`, **Buy credits**. **Payments** is `Stripe Connect status: Not connected`, **Connect Stripe**. Neither screen carries one word about what it is for, or that the other exists. **MO-G7** stands as written |
| 8.1 | `manual` | as expected | `201`, and the message renders as `Maya Okonkwo (staff) — 8/22/2026, 2:04:28 PM`. No edit and no delete control on the message, as claimed |
| 8.2 | `manual` | as expected | The Client replied from the portal (`201`, `senderType: "client"`), and Maya's reloaded thread showed both in order, staff then client, each labelled with its sender type |

**17 `manual` steps walked; 4 `missing-feature` steps confirmed unwalkable; 5
`blocked` steps attempted and their real responses recorded (2 re-marked from
`manual`).** One expected result was falsified (5.2), minting **MO-G10** on the
journey map. No `journey-gap` issue was filed — that is
[#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict against "a pass means"**: **it does not pass, and one of the two
reasons is not the product's fault.** One Client carries a filled Birth Plan the
Client can read in the portal, a Visit, an open two-way message thread, and a
Contract — all reached by one person with no help. The Contract is **not
signed**, because the walking stack has no object store; that is the harness, and
the deployed service has the bucket. The **Invoice does not exist**, and that is
the product's own missing leg, blocked on a Stripe account nobody has opened.

Her moment of truth landed where the map put it. The Add Client form takes a name
and an email, so at minute five the paper folder survives the move — and the walk
found the same shape one stage later, in a Contract that asks her to retype the
Practice name and the Client name the product already holds, and then lets her
send it blank.

**For the walks behind this one** ([#235](https://github.com/markgoho/doula-cloud/issues/235)–[#241](https://github.com/markgoho/doula-cloud/issues/241)):
every plan with a Contract-signing step meets the same 500. To walk it, run a
fake GCS (`fsouza/fake-gcs-server`) and point `STORAGE_EMULATOR_HOST` at it in
`app/e2e/stack.ts` instead of `storage-emulator-disabled.invalid:1`. That is test
infrastructure, not a product change, and it belongs to
[#209](https://github.com/markgoho/doula-cloud/issues/209) rather than to any one
walk.

#### Addendum, same day — the two `blocked` steps walked

The walk above left 5.4-a and 5.5 `blocked` on an object store the stack did not
have, and recommended standing one up. That was done the same day:
`app/compose.e2e.yaml` gained a pinned `fsouza/fake-gcs-server`, `stack.ts`
creates the bucket and points the BFF at it. Both steps went back to `manual` and
were walked.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 5.4-a | `manual` (was `blocked`) | as the plan originally claimed | `POST /api/portal/engagements/{id}/contract/sign` -> `200`, `"status":"signed"`, and the portal screen changed from `Status: sent` to `Status: signed` and dropped the signing form. The signed PDF is really in the store: `contracts/{engagementId}/signed.pdf`, 1245 bytes, `application/pdf` |
| 5.5 | `manual` (was `blocked`) | as expected, plus one control the plan does not name | Maya's Engagement reads `Status: signed` without leaving the app, and the Contract section now offers **Void Contract** — the button Dee's plan walks at 7.1-a |

**Final marks for this plan: 9 `automated`, 19 `manual`, 3 `blocked` (all
Stripe), 4 `missing-feature`.** The Contract leg of "a pass means" now passes
end to end — invite, build, send, sign, and Maya sees it — and the only leg still
short is the Invoice, which is blocked on a Stripe account nobody has opened.

One thing the fix made visible that the first walk could not: with the merge
fields **filled**, the Client reads `This agreement is between Okonkwo Birth
Support and Hannah Sorensen for doula services.` The prose resolves correctly the
moment values exist — which is what makes **MO-G10** a gap about the product not
filling in what it already knows, rather than a broken template.
