# Connect account state is two capabilities and a requirements list

Stripe refuses to create Accounts v1 for a new integration. Setting up the
Sandbox for [#242](https://github.com/markgoho/doula-cloud/issues/242), the very
first Connect call came back:

> Stripe no longer recommends Accounts v1 for new Connect integrations. Create
> connected accounts with `POST /v2/core/accounts` instead.

A compatibility toggle would have unblocked v1 with no code change. We did not
take it: pre-launch, no users, no data to migrate, and by Stripe's own definition
we *are* a new integration. Building the walk, the docs and the test-plan marks on
a surface we would have to leave anyway means doing all of it twice.

The move itself is mechanical. What needed deciding is what a Practice's Connect
state **is**, because v2 does not report the thing v1 reported.

## v1's three booleans do not survive

`00023_stripe_connect_account.sql` stored what a v1 Account said about itself:

| v1 | v2 |
| --- | --- |
| `charges_enabled` | `configuration.merchant.capabilities.card_payments.status` |
| `payouts_enabled` | `configuration.merchant.capabilities.stripe_balance.payouts.status` |
| `details_submitted` | nothing — `requirements.entries` carries what is outstanding |

Both statuses are four-valued: `active`, `pending`, `restricted`, `unsupported`.
A boolean cannot hold four values, and it particularly cannot hold `pending` —
Stripe reviewing what the Owner already supplied, so the account works for nobody
and there is nothing left for the Owner to do. Under v1 that read as "onboarding
incomplete", which invited the Owner to redo onboarding they had already
finished.

So the columns were redesigned rather than renamed
(`00029_stripe_connect_accounts_v2.sql`):

```
stripe_connect_card_payments_status  text  -- active|pending|restricted|unsupported
stripe_connect_payouts_status        text  -- same four
stripe_connect_requirements_due      text[]
stripe_connect_status_event_id       text
stripe_connect_status_updated_at     timestamptz
```

`requirements_due` holds the `description` of every requirements entry
`awaiting_action_from: user` — dotted Stripe field paths like
`configuration.merchant.mcc`. Entries awaiting Stripe or awaiting the platform
are dropped: neither is something the Owner can clear by reopening onboarding.
These are Stripe field names, never Client data, so the no-PHI rule (#30, #78)
is untouched.

The last two columns are the audit trail: which Stripe delivery moved this
Practice's state, and when.

## The Payments screen gained two states

`not_connected` / `onboarding_incomplete` / `active` could not describe a real v2
account. Two more:

- **`pending`** — Stripe is reviewing. The onboarding button is **hidden**, since
  there is nothing the Owner can supply.
- **`payouts_restricted`** — `card_payments` is active while payouts is not.
  Clients can pay their invoices; the money cannot reach the Practice's bank yet.
  Reporting this as "onboarding incomplete" read as if invoicing were broken,
  which it is not.

`card_payments` leads the derivation, because being payable at all is what the
Practice came for. Payouts only refines an otherwise active account. Outstanding
requirements outrank a `pending` capability: the two capabilities move
independently, so an account really can report `card_payments` restricted while
payouts is pending, and calling that `pending` would hide the button while the
Owner still owed Stripe information.

Two smaller decisions fell out of the same question:

- **The button follows the ask, not the status.** It appears when Stripe has
  something outstanding the Owner could supply — which is why
  `payouts_restricted` shows it only when `requirementsDue` is non-empty. Stripe
  can restrict payouts because it is reviewing bank details, and reopening the
  form then is a dead end.
- **The screen shows the count, not the list.** `requirements.entries[].description`
  is documented by stripe-go as a *machine-readable* string; `configuration.merchant.mcc`
  names nothing an Owner recognizes. The paths are persisted for the audit trail
  and summarized on screen as "Stripe needs N more details from you." The place
  those get asked in words is Stripe's own hosted form.

## Two webhook routes, because Stripe allows one payload type per destination

Verified against the Sandbox rather than assumed:

- **The v1 payload cannot carry the v2 model.** A v2 account *does* still emit v1
  snapshot `account.updated` on the connected account — an earlier draft of this
  ADR said it emitted none, which was wrong: that check listed the platform's
  `/v1/events`, where connected-account events never appear, and the walk later
  showed `account.updated` arriving. What holds is the reason that matters: the v1
  payload is the three booleans, which cannot express four-valued capability
  statuses or a requirements list. `v2.core.account[configuration.merchant].capability_status_updated`
  is the event that can.
- **One destination cannot carry both.** Subscribing one destination to a thin
  and a snapshot event type is rejected: *"Enabled events list contains 'thin'
  event types when event_payload is 'snapshot'."*

Hence `/api/stripe/account-webhook` (thin) and `/api/stripe/connect-webhook`
(snapshot), with separate signing secrets. `stripe.ConstructEvent` refuses a thin
event by design, so verification splits too — `Client.ParseAccountEvent` alongside
`VerifyWebhookSignature`.

A thin notification carries **no object**, only a reference to what changed. So
the account handler fetches the account from Stripe and persists what Stripe
currently reports. That is not a workaround: it means an out-of-order delivery
cannot write a stale status, because whatever is fetched is current at the moment
of the write.

## The Practice bears fees and losses

`defaults.responsibilities` is mandatory for a merchant configuration and has no
default. Both collectors are `stripe`: the Practice's own account is billed
Stripe's processing fee and absorbs a disputed charge.

This is not a new position — it is what v1's `type=standard` did silently, and
what direct charges already meant here. There is no `ApplicationFeeAmount`
anywhere in `payments`; the Client's money lands in the Practice's balance and
never passes through ours. The alternative (`application`) would put Client funds
on our balance sheet and make us a money transmitter.

`identity.country` is likewise required at create time, where v1 inferred it
during onboarding. Hardcoded `us`, matching the USD credit Price and the USD
currency on every InvoiceItem. A non-US Practice needs a country on the Practice
itself, which nothing in the pilot asks for.

## Two things the walk found that the API reference did not

Both were invisible until a real Client paid a real invoice.

**The Client saw the statement descriptor, not the Practice.** With no
`display_name` on the v2 Account, Stripe falls back to the statement
descriptor, so a walked invoice read **"From DOULA.CLOU"** — the platform's
truncated descriptor — rather than the Practice the Client hired.
`CreateAccount` now takes the Practice's name and sets `display_name`.

**Every `payments` row was written with an empty Stripe reference.** Under
API version `2026-07-29.dahlia` an Invoice carries neither `payment_intent`
nor `charge`, and the `invoice.paid` event body carries no `payments` list
either. The handler's `payment_intent` JSON tag therefore unmarshaled to
`""` silently, and nothing failed. The mapping now lives on the
InvoicePayment object, which webhook payloads never include, so the handler
fetches it through the port. A failure to fetch logs and stores an empty
reference rather than 500-ing: the money has already moved, and Stripe
redelivering cannot improve on it.

That second one is the argument for walking rather than reading. The field
was gone from the API, the struct still had a tag for it, and the tests
passed because the fixture supplied a field production never sends.

## What did not change

The Invoice leg stays on v1 APIs. Stripe accepts a v2 account id on a v1 endpoint
and applies updates to the corresponding v2 properties, so every `V1Invoices` call
with `Params.StripeAccount` works unchanged.

## The direct-charge shape is load-bearing, not incidental

Recorded after [#383](https://github.com/markgoho/doula-cloud/issues/383) walked the Sandbox and read the code: every Client-facing charge is created **on the connected account**, with no `application_fee_amount`, no `transfer_data`, no `destination`, and no platform balance entry. A walked $1,800 payment settled wholly into the Practice's balance, with Stripe's fee charged to the Practice.

That is what makes the Practice merchant of record and keeps Doula Cloud out of the flow of funds — which in turn is the whole basis for the ruling on #383 that Doula Cloud is **not a money transmitter**. New York offers no payment-processor exemption and has not adopted the MTMA, so there is no fallback if the shape changes.

**Five constraints follow, and breaking any of them reopens the question:** no application fee on a Client's payment ever (a constraint on [#285](https://github.com/markgoho/doula-cloud/issues/285)'s pricing model, not only on code); no funds resting with the platform; no contractor payouts out of Doula Cloud ([#391](https://github.com/markgoho/doula-cloud/issues/391)); no describing the product as a payment intermediary (Stripe Connect Terms §3.4(b)); hosted onboarding and Radar left on, which Stripe's managed-risk relief is conditioned on.

Full reasoning: [`docs/research/money-transmission-posture.md`](../research/money-transmission-posture.md), and the ruling in [#383](https://github.com/markgoho/doula-cloud/issues/383).
