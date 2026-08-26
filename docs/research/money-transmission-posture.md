# Money-transmission posture: does running this Connect platform license us?

Research for [#383](https://github.com/markgoho/doula-cloud/issues/383), on the entity map [#375](https://github.com/markgoho/doula-cloud/issues/375). Verified 25 August 2026 against the code at `trunk` and against the live Stripe Sandbox `acct_1U7N3e1rKoVEA79v` ("Doula Cloud sandbox").

**This memo establishes facts. It does not conclude the legal question.** The ticket says the conclusion — *"and therefore no licence is required in New York"* — needs an attorney's signature, and that judgement stands. Section 7 is the question to hand over.

## 1. What is established, in one paragraph

Doula Cloud's Connect integration uses **direct charges**. Every Client-facing Stripe object is created on the Practice's own connected account via the `Stripe-Account` header; the Practice is the merchant of record; Stripe's processing fee and any dispute loss are collected from the Practice, not the platform; and there is no `application_fee_amount`, no `transfer_data`/`destination`, and no separate charges-and-transfers anywhere in the codebase. A real walked payment of $1,800.00 settled **entirely into the Practice's Stripe balance** and produced **zero** entries on the platform's balance. Doula Cloud's own revenue is an unrelated direct sale of credits on its own Stripe account. At no point, not even momentarily, do Client funds rest anywhere Doula Cloud controls.

Whether that posture is *sufficient* to stay outside NY Banking Law article 13-B is the attorney's call, not this memo's.

---

## 2. Charge type, established from the code

### 2.1 Every Client-facing call is made on the connected account

`api/internal/payments/stripe_api_client.go` builds one `stripe.Params{StripeAccount: accountID}` and passes it to every object in the invoice flow — Customer, Invoice, InvoiceItem, finalize, and the InvoicePayment lookup:

```go
onBehalfOf := stripe.Params{StripeAccount: stripe.String(accountID)}
```

That header is what makes a charge a *direct* charge. Stripe's own definition of the three Connect charge types turns on where the object is created and whose balance it settles into (section 4).

**One naming hazard, so nobody misreads it.** The local variable is called `onBehalfOf`, but it sets the **`Stripe-Account` header**, not Stripe's `on_behalf_of` *parameter*. Those are different things: `on_behalf_of` names a settlement merchant on a charge created on the *platform's* account, and it is a destination-charge concept. It is `null` on the walked charge and the walked invoice (section 3.1), and it appears nowhere in the codebase as a parameter. The variable name is unfortunate; the behaviour is not.

### 2.2 The connected account is created as merchant of record, bearing fees and losses

`CreateAccount` builds an Accounts v2 account with the **`merchant`** configuration, a **full** dashboard, and:

```go
Defaults: &stripe.V2CoreAccountCreateDefaultsParams{
    Responsibilities: &stripe.V2CoreAccountCreateDefaultsResponsibilitiesParams{
        FeesCollector:   "stripe",
        LossesCollector: "stripe",
    },
},
```

`fees_collector: stripe` means Stripe debits its processing fee from **the Practice's** balance. `losses_collector: stripe` means a disputed charge is debited from **the Practice's** balance. Neither cost, and therefore neither corresponding credit, ever transits Doula Cloud.

This is deliberate and already recorded: [ADR-0007](../adr/0007-connect-account-state-is-two-capabilities-and-a-requirements-list.md) says in terms that the alternative (`application`) "would put Client funds on our balance sheet and make us a money transmitter", and `docs/environment.md` repeats it.

### 2.3 The absence is exhaustive, not spot-checked

A grep across all of `api/` and `app/` for `ApplicationFee`, `TransferData`, `Destination`, `V1Transfers`, `V1Payouts`, `V1Balance`, `Topup` returns **no** money-movement call sites. The only hits are the word "destination" in comments about *webhook event destinations* — an unrelated Stripe concept — and the one comment in `stripe_api_client.go` asserting the absence itself. There is no `Transfer`, no `Payout`, no `Topup`, and no read of a platform balance anywhere in the product.

### 2.4 The platform's own revenue is a separate, ordinary sale

`api/internal/billing/` sells **credits** to a Practice through a Checkout Session on Doula Cloud's *own* Stripe account (`billing/stripe_api_client.go`, `CreateCheckoutSession`). The Practice is the customer, Doula Cloud is the merchant, and the money is Doula Cloud's own earned revenue for its own software. That flow is not Connect at all and involves no third party's funds. It is called out separately in section 7 only because prepaid balances have their own statutory vocabulary.

### 2.5 The contractor doula's fee is a record, not a rail

`engagement_attachments.fee_amount_cents` / `fee_terms` (`api/db/migrations/00030_employment_attachment_offer.sql`, written by `staffauth/attach.go`) record what a Practice agreed to pay a contractor Doula when she accepted an Offer. **Nothing in the product moves that money.** There is no payout, no transfer, no Stripe object of any kind attached to it. A Practice pays its contractors entirely outside Doula Cloud.

This matters for section 6: it is the most plausible future feature that would reopen the whole question.

---

## 3. Whether funds ever rest under Doula Cloud's control — answered from the Sandbox

Read live from the Sandbox on 25 August 2026. Every id below is re-checkable.

### 3.1 The walked Client payment

A real invoice was created, finalized, and paid end-to-end during the #247 walk:

| Fact | Value |
| --- | --- |
| Invoice | `in_1U7Q3y1rKofzifuhGBzyiV9i`, status `paid`, total `180000` ($1,800.00) |
| Lives on | connected account `acct_1U7PeY1rKofzifuh` (metadata `practice_id: ec75a834-…`) |
| `application_fee_amount` | `null` |
| `on_behalf_of` | `null` |
| `transfer_data` | `null` |
| Charge | `ch_3U7Q401rKofzifuh0dn9JD0c`, `180000 usd` |
| Charge `destination` / `source_transfer` | `null` / `null` |
| Balance transaction | `txn_3U7Q401rKofzifuh0Lc9IAkK`, **on the connected account**: gross `180000`, fee `5250`, net `174750` |
| Practice's balance afterwards | `pending: 174750 usd` |
| Platform balance entries caused by this payment | **none** |

The `5250` fee is Stripe's standard 2.9% + $0.30 ($52.20 + $0.30) charged to *the Practice*, which is `fees_collector: stripe` observed rather than assumed.

### 3.2 The platform balance, reconciled completely

The platform account's entire balance-transaction history is five rows. All five are accounted for, and none is a Client's money:

| Balance txn | Type | Amount | Source | What it is |
| --- | --- | --- | --- | --- |
| `txn_3U7Ntw1rKoVEA79v113y0cfI` | `charge` | `+1000` (net `941`) | `ch_3U7Ntw1rKoVEA79v1E7kyQQg` | A $10 **credit purchase** — Doula Cloud's own SaaS revenue (section 2.4) |
| `txn_3U7NEZ1rKoVEA79v0p0K8SPu` | `charge` | `+10000` | `ch_3U7NEZ1rKoVEA79v01NtL6pk` | Stripe's own demo fixture, description literally `(created by Testing Blueprints)` |
| `txn_3U7NEZ1rKoVEA79v0KW7ooJd` | `transfer` | `-10000` | `tr_3U7NEZ1rKoVEA79v0V6hPBWn` | The same fixture's destination transfer out |
| `txn_1U7NEe1rKoVEA79vnj8v1qhK` | `application_fee` | `+300` | `fee_1U7NEb1rKooDgWEh0Ij40e1K` | The same fixture's application fee |
| `txn_1U7NEf1rKoVEA79vs0M8tRPT` | `application_fee` | `+300` | `fee_1U7NEd1rKoKakL1QuGppxOwa` | The same fixture's application fee |

Platform balance today: `available 280`, `pending 941` — the $10 credit sale still settling, plus what is left of Stripe's own $6 of demo application fees. `connect_reserved: 0`. There are **no platform payouts** at all.

### 3.3 The destination charge in the Sandbox is Stripe's, not ours

This needs saying explicitly, because a future auditor listing platform charges will see a `transfer_data`, a `destination`, and two application fees and think the "no application fee" claim is false.

Five connected accounts exist in the Sandbox. Three were created by Doula Cloud's code and carry a `practice_id` in metadata. Two were not:

| Account | Created | `practice_id` metadata | Business URL | Origin |
| --- | --- | --- | --- | --- |
| `acct_1U7Rwv1rKod8tdZe` | 2026-08-23 03:20 | `0f5cdc9a-…` | rootedbirthcollective.com | Doula Cloud |
| `acct_1U7RdD1rKocBawcv` | 2026-08-23 02:59 | `ce8e1bd7-…` | — | Doula Cloud |
| `acct_1U7PeY1rKofzifuh` | 2026-08-23 00:52 | `ec75a834-…` | doula.cloud | Doula Cloud |
| `acct_1U7NEM1rKooDgWEh` | 2026-08-22 22:17 | *(empty)* | accessible.stripe.com | **Stripe fixture** |
| `acct_1U7NEM1rKoKakL1Q` | 2026-08-22 22:17 | *(empty)* | accessible.stripe.com | **Stripe fixture** |

Both fixtures are named "Test connected account" and were minted by Stripe's own sandbox onboarding blueprint at 22:17 on 22 August, before any Doula Cloud code ran. **Every** application fee, transfer, and destination charge in the account traces to those two. Doula Cloud's three accounts have produced exactly one charge between them — the $1,800 direct charge in 3.1 — with every Connect field null.

The three Doula Cloud accounts also read, on the v1 compatibility view, as:

```
controller: { fees: { payer: "account" },
              losses: { payments: "stripe" },
              requirement_collection: "stripe",
              stripe_dashboard: { type: "full" },
              type: "application" }
```

`fees.payer: account` and `losses.payments: stripe` are the v1 spelling of the same thing section 2.2 sets on v2 — the Practice pays the fee and eats the dispute. The Stripe fixture `acct_1U7NEM1rKoKakL1Q`, by contrast, reads `fees.payer: application`, `losses.payments: application` — the platform-liable shape Doula Cloud deliberately does not use.

### 3.4 The answer

**No. Client funds never settle anywhere Doula Cloud possesses them, not even momentarily.** Money moves Client → Stripe → the Practice's Stripe balance → the Practice's own linked bank account. Doula Cloud's balance never sees it, and nothing in the integration routes — or could route, without adding a charge type it does not use — settlement to Doula Cloud.

**One distinction the memo must not blur**, because counsel will draw it anyway. Doula Cloud has no *possession* of Client funds, but it does have *administrative API authority* over each connected account: the platform key acts on the Practice's account through the `Stripe-Account` header (that is how the invoice is created there at all), and the v1 view records `controller.is_controller: true, type: application`. Doula Cloud can create and finalize an Invoice on the Practice's account. What it cannot do, on this configuration, is take the proceeds. Whether administrative reach short of possession bears on the statutory test is exactly the sort of question section 7 puts to the attorney rather than answering here.
