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

---

## 4. What Stripe's own terms say, and what they demand in return

All quotations below are from first-party Stripe pages, fetched 25 August 2026.

### 4.1 Stripe defines the merchant of record by charge type, and direct charges put it on the Practice

From [Understand the merchant of record in a Connect integration](https://docs.stripe.com/connect/merchant-of-record):

> "The MoR is the entity with legal responsibility for a transaction. It can be your platform or your connected accounts, depending on your configuration."
>
> "**Direct charges:** The merchant of record is the connected account."
>
> "For a connected account created using the Accounts v2 API to be the MoR, it must have the `merchant` configuration. Otherwise, payments will fail."

Doula Cloud creates its accounts with the `merchant` configuration (section 2.2), so this is the branch that applies. Stripe frames the same split by business model in [SaaS platforms and marketplaces](https://docs.stripe.com/connect/saas-platforms-and-marketplaces): "**SaaS platforms** … Connected accounts act as the merchant of record (MoR)"; "**Marketplaces** … Your platform is usually the merchant of record". Doula Cloud is on the SaaS side of Stripe's own dichotomy.

### 4.2 Stripe's description of direct charges matches what section 3 observed

From [Connect charges](https://docs.stripe.com/connect/charges):

> "You create a charge on your connected account, so the payment appears in the connected account's balance, not in your platform's balance."
>
> "For disputes on payments created using direct charges, Stripe debits the disputed amount from the connected account's balance, not your platform's balance."

And the contrast Stripe draws with the two indirect types:

> "**Indirect charges**: Payments made to your platform. Funds then transfer from your platform to Stripe as fees and to the connected account as its portion of the payment."
>
> Destination charges: "You create a charge on your platform, so the payment appears in your platform's balance."
>
> Separate charges and transfers: "You create a charge on your platform's account first. Create a separate transfer to move funds to your connected account."

Stripe names the exact mechanism the code uses, in [Direct charges](https://docs.stripe.com/connect/direct-charges):

> "**Stripe-Account**: This header indicates a direct charge for your connected account."
>
> "We recommend using direct charges for connected accounts that have access to the full Stripe Dashboard."

Doula Cloud sets `dashboard: full` (section 2.2), which is the configuration Stripe recommends direct charges for.

### 4.3 The responsibility settings are Stripe's recommended pairing, and they push liability to Stripe

From [Connected account configuration](https://docs.stripe.com/connect/accounts-v2/connected-account-configuration), on the two values Doula Cloud sets:

> `fees_collector` = `stripe`: "Stripe collects payment fees directly from the connected account."
>
> `losses_collector` = `stripe`: "**Stripe is liable for the connected account's negative balances. Your platform is still liable for negative balances on your platform account.**"

Same page, on KYC:

> "Responsibility for collecting KYC requirements is based on the `defaults.responsibilities.losses_collector` and `dashboard` values. **In most configurations, Stripe is responsible for collecting KYC requirements from your connected accounts.** Your platform is responsible only when you set `defaults.responsibilities.losses_collector` to `application` and `dashboard` to `none`."

Doula Cloud sets neither of those, so Stripe collects KYC. [Integration recommendations](https://docs.stripe.com/connect/integration-recommendations) lists exactly this triple — direct charges, full dashboard, negative-balance liability with Stripe, payment-fee collector Stripe — as the recommended shape.

### 4.4 Stripe contracts with the Practice directly

Connect Infrastructure Terms §2.1, [stripe.com/legal/connect](https://stripe.com/legal/connect) (last modified 18 November 2025):

> "**Stripe has a direct contractual relationship with each Connected Account under the Connected Account Agreement and will provide the Services directly to each Connected Account.**"

And the [Connected Account Agreement](https://stripe.com/legal/connect-account) itself:

> "**Stripe is not a Stripe Connect Platform**, and only provides the Services described in this Connected Account Agreement and the Stripe Services Agreement."
>
> "User is solely responsible for, and Stripe disclaims all liability for, the provision of goods and services sold to User's Customers…"

So the Practice has its own agreement with Stripe. Doula Cloud is not a party interposed between them.

### 4.5 Stripe names itself, not the platform, as the licensed money transmitter

Stripe Payments Terms §5.1, [stripe.com/legal/connect](https://stripe.com/legal/connect):

> "**5.1 Regulated Money Transmission.** Certain Services involve regulated money transmission under U.S. Law. To the extent that User's use of the Services involves money transmission or other regulated services under U.S. Law, Stripe's Affiliate, Stripe Payments Company ('SPC') provides those regulated Services… **Stripe is not a bank, and does not accept deposits.**"

[Stripe Payments Company Terms](https://stripe.com/legal/spc):

> "**SPC is a U.S. state-licensed money transmitter and federally registered money services business.**"

[Stripe Payments Company Licenses](https://stripe.com/legal/spc/licenses), New York entry:

> "**New York.** New York Department of Financial Services. **Stripe Payments Company is licensed and regulated as a money transmitter by the New York State Department of Financial Services.**"

That is the licensed party in this flow, by name, in New York specifically. It is not Doula Cloud.

### 4.6 What Stripe demands of the platform in return

This is the half a favourable read tends to skip. The obligations are real and ongoing.

**Joint-and-several liability for connected accounts.** Stripe Connect Terms §3.1:

> "As between User and Stripe, User is responsible for all Activity on its Connected Accounts, whether initiated by User or not. User is liable to Stripe for all: (a) Transactions, Disputes, Refunds, Reversals and resulting Merchant Losses (**except with respect to SMR Connected Accounts to the extent stated in Section 3.2**) and (b) any other losses, damages, and costs that result from use of the Services, including any fines assessed by Financial Providers or Governmental Authorities… User remains jointly and severally liable with the applicable Connected Accounts to Stripe for these amounts…"

§3.2 is the carve-out that `losses_collector: stripe` buys:

> "**Stripe is liable for Merchant Losses on SMR Connected Accounts up to the amount of the SMR Risk Cap (if any)**, except: (a) as stated in Section 2.5…; and (b) to the extent these Merchant Losses arise from User's fraud, violation of Law, breach of the Agreement…, negligence, willful misconduct, or misuse of the Stripe Connect Services."

Note the shape. Stripe absorbs a Practice's negative balance, but the base liability in §3.1 is uncapped (§7: "User's liability for all Connected Accounts … is not limited or excluded in any way"), and §3.2's protection falls away on Doula Cloud's own breach or violation of law. **The protection is conditional on Doula Cloud behaving.**

**Policing the connected accounts.** Connect Infrastructure Terms §3.2:

> "User must ensure that Connected Accounts do not use the Services in breach of the Connected Account Agreement or for any activity that Law or this Agreement prohibits. User must immediately inform Stripe if User becomes aware that a Connected Account is engaging in any activity that is illegal, fraudulent, deceptive or harmful…"

**Getting the Practice onto the Connected Account Agreement.** Doula Cloud uses Stripe-hosted onboarding — an Account Link into Stripe's own form (`CreateAccountLink`) — which is §4.1(b), not §4.1(a). That distinction has money attached: §7 removes the liability cap entirely for a platform that uses **User-Hosted** onboarding and fails to bind a connected account to the Connected Account Agreement. Staying on Stripe-hosted onboarding is therefore not only a KYC convenience; it keeps Doula Cloud out of the uncapped-liability clause.

**The one clause that reads directly on this question.** Stripe Payments Terms §3.4(b):

> "User must ensure that Customers understand that User is responsible for the Transactions. **User must not act as or hold itself out as a payment facilitator, intermediary or aggregator, or otherwise resell the Stripe Payments Services.**"

Contractual, not regulatory — but it points the same way as the statute would: the platform must not present itself as standing between payer and payee. Doula Cloud's product copy should be checked against it (section 6).

**Money services businesses are a Restricted business.** The [Prohibited and Restricted Businesses](https://stripe.com/legal/restricted-businesses) list puts "Money transmitters, remittances, currency exchange services, and other money service businesses" in the **Restricted** tier ("require additional due diligence… proof of relevant licenses"), and "Peer-to-peer money transmission" and "Payable-through accounts" in the **Prohibited** tier. SSA General Terms (ix) forbids using the Services to "operate or benefit from any Prohibited or Restricted Business… unless Stripe has pre-approved the respective Prohibited or Restricted Business in writing". So if Doula Cloud ever *were* a money transmitter, its own Stripe account would be out of compliance until Stripe pre-approved it and it produced licences. The regulatory question and the vendor question do not come apart.

### 4.7 What Stripe does **not** say — a checked absence, not an unchecked one

Across `/connect/charges`, `/connect/direct-charges`, `/connect/destination-charges`, `/connect/separate-charges-and-transfers`, the `/connect` overview, `/connect/service-agreement-types`, `/connect/account-balances`, `/connect/accounts-v2/connected-account-configuration`, `/connect/integration-recommendations`, `/connect/top-ups` and `/connect/required-verification-information`, the phrase "money transmi…" appears **once**: in [risk management](https://docs.stripe.com/connect/risk-management), listing "Money transmitter licenses (MTL) in the US" among the screens *Stripe* performs.

- **Stripe nowhere states that a Connect platform does not need its own money transmitter licence.** There is no such assurance to rely on, in docs or in contract.
- The only Stripe pages linking a *platform* to MTL need are the payouts comparison tables — [cross-border payouts](https://docs.stripe.com/connect/cross-border-payouts) and [global payouts vs Connect](https://docs.stripe.com/global-payouts/compare-with-connect) — which say Connect payouts "**Offload legal and compliance requirements to Stripe using the Stripe Money Transmitter License**", while Global Payouts means "**If you're managing your customers' funds, you might need a Money Transmitter License**". Directionally supportive, but scoped to payouts products Doula Cloud does not use.
- **There is no clause in the SSA where a platform represents that it is not engaged in money transmission.** The nearest constraints are §3.4(b) above and the Restricted-business list.
- **No Stripe document forbids a platform holding a balance**, or says a platform balance changes its status. [Account balances](https://docs.stripe.com/connect/account-balances) and [top-ups](https://docs.stripe.com/connect/top-ups) describe the mechanics with no regulatory commentary.
- The one place Stripe *requires* a platform to state it is "neither a bank nor a money transmitter" is [Treasury/Financial Accounts compliance](https://docs.stripe.com/financial-accounts/connect/compliance) — a product Doula Cloud does not use. There is no Connect equivalent.

**The practical consequence.** Stripe's terms and docs establish that in this configuration the Practice is the merchant of record and Stripe Payments Company is the licensed transmitter, NYDFS-licensed by name. They do **not** contain a statement that a platform in this posture is exempt from state licensing. Nobody at Stripe has signed anything to that effect, and a contract with Stripe would not bind NYDFS in any case. That is why section 7 exists.

**Two dead URLs, recorded so nobody re-hunts them.** `stripe.com/legal/connect-platform-agreement` and `stripe.com/legal/licenses` both 404. The live documents are [stripe.com/legal/connect](https://stripe.com/legal/connect) ("Stripe Connect – Platform" and "Stripe Connect – Infrastructure" sections) and [stripe.com/legal/spc/licenses](https://stripe.com/legal/spc/licenses).

---

## 5. New York, specifically

Sources: NY Banking Law article 13-B via [nysenate.gov](https://www.nysenate.gov/legislation/laws/BNK/A13-B), NYDFS's own licensing page and opinion letters at dfs.ny.gov, and 3 NYCRR part 406. Fetched 25 August 2026.

**Two corrections to the working vocabulary before anything else**, because both send a reader to the wrong place:

- The money-transmitter regulation is **3 NYCRR part 406**, in Title 3 (Banking). It is *not* 23 NYCRR. 23 NYCRR part 200 is the separate virtual-currency "BitLicense" regime and has nothing to do with this.
- Within article 13-B, the penalty section is **§ 650**. § 651 is "Investments" (a permissible-investments reserve rule) and § 652 is severability. The application requirements are § 641(2), not § 642 — § 642 is the superintendent's action on an application.

### 5.1 The operative prohibition, quoted

**NY Banking Law § 641(1)** ([nysenate.gov/legislation/laws/BNK/641](https://www.nysenate.gov/legislation/laws/BNK/641)):

> "No person shall engage in the business of selling or issuing checks, or engage in the business of **receiving money for transmission or transmitting the same**, without a license therefor obtained from the superintendent as provided in this article, **nor shall any person engage in such business as an agent, except as an agent of a licensee or as agent of a payee**; provided, however, that nothing in this article shall apply to a bank, trust company, private banker … [and other depository institutions]."

Three things follow from the text itself:

1. **There is no de minimis threshold.** Nothing in § 641 turns on volume or dollar amount. The dollar figures that do appear in the statute — $10,000 in one transaction, $25,000 in thirty days, $250,000 in a year — are in **§ 650(2)(b)** and are *felony-escalation* thresholds, not licensing thresholds. Unlicensed transmission with no threshold at all is already a **Class A misdemeanor** under § 650(2)(a).
2. **The exemption list is depository institutions only.** No software exemption, no SaaS exemption, no marketplace exemption.
3. **The nexus is "in this state"** — § 641(2)(c), § 642(4) and § 648 all frame the business as conducted in New York. A New York LLC serving New York Practices is squarely inside it.

The cost of a licence, if one were needed, is set by **§ 643**: a surety bond of "no less than five hundred thousand dollars", plus § 651's permissible-investments reserve. This is not a filing fee. It is a capital requirement that would end the business as currently conceived — which is why the question is on the entity map.

### 5.2 The statute does not define its own trigger

**§ 640 defines: "Person", "Licensee", "Check", "Payment instrument", "Traveler's check", "Permissible investments", "Agent". It does not define "money transmission", "money transmitter", or "receiving money for transmission."** That absence is a finding, not an oversight in the research: the operative phrase in § 641(1) is given content only by regulation and by DFS's own letters.

The regulatory definition, **3 NYCRR 406.2(a)**:

> "The term **money transmission** shall include all instruments sold or issued including travelers checks, money orders, checks, drafts, orders, wire or electronic transfers, facsimile transfers and shipments by courier for the transmission or payment of money."

Note that § 640(10)'s definition of "Agent" is an agent **of a licensee** only. The statute nowhere defines "agent of a payee"; that lives at **3 NYCRR 406.2(l)**:

> "The term **agent of a payee** means any person authorized by a payee to receive funds on behalf of the payee and to deliver such funds received from the payor to the payee."

And **3 NYCRR 406.1**:

> "As provided in Banking Law, section 641, the provisions contained in this Part shall not apply to an agent of a payee … **Only agents of a payee which do not engage in any money transmission activities other than as an agent of a payee are exempt** from the provisions of this Part."

### 5.3 New York has no payment-processor exemption, and has not adopted the model act

- **No payment-processor exemption exists anywhere in article 13-B or 3 NYCRR part 406.** This is the sharpest contrast with the federal position (section 6.4). The only adjacent carve-outs are 3 NYCRR 406.2(k), which excludes certain *instruments* — bank obligations, interbank clearing — from being "issued or sold".
- **New York has not adopted the Money Transmission Modernization Act.** CSBS's own tracker, ["2022-2026 MTMA Proposed and Enacted Legislation" (April 2026)](https://www.csbs.org/sites/default/files/external-link-files/MTMA%20Legislative%20Update_4.23.2026.pdf), does not mention New York anywhere across four years of introductions and enactments. So the MTMA's statutory exemption list — the one many states now use to carve out payroll processors and agents of a payee — **does not apply in New York**. Article 13-B as quoted was last modified 22 September 2014.
- New York's agent-of-a-payee carve-out is therefore **narrower in form than most states'**: it is not a general exemption for receiving money as agent of a payee, it is an exception to the *agent-licensing prong* of § 641(1), conditioned by DFS's letters.

### 5.4 The NYDFS letter that is nearly on point

This is the most important source in the memo. NYDFS **Letter of 9 July 2007, "Money Transmitter Licensing Requirements for Payment Processing Services"** ([dfs.ny.gov/legal/interpret/lo070709b.htm](https://www.dfs.ny.gov/legal/interpret/lo070709b.htm)). The facts were a company running websites and IVR that collected card and ACH payments on behalf of government entities, universities and schools.

The Department's conclusion:

> "the Department essentially disagrees with your conclusion and believes … that the Client would have to be licensed as a money transmitter based upon the information you supplied, **except in the situation where credit card payments go directly from credit card accounts to accounts owned by the intended payees**."

And, decisively for this memo:

> "**Since these credit card payments go directly from the credit card account to an account owned by the intended payee, there appears to be no risk to the payor and it appears that there is no receipt of money for transmission by the Client within Banking Law, Section 641.**"

That describes the shape section 3 measured: the Client's card payment settles directly into the Practice's own Stripe balance and thence to the Practice's own bank, and Doula Cloud never receives it.

The same letter sets out what DFS demands where a processor *does* receive the funds and wants to rely on being an agent of the payee — a standard it called the "agency exception":

> "agents of payees must give customers a receipt which indicates that payment to the agent is deemed payment to the payee. There can be no risk of loss to the payor if any of the transmitter fails to remit the funds. Whether or not the payee receives the funds, the payees must treat the payors as if, in effect, the payees received payment."

The earlier **Letter of 10 January 2000** ([lo000110.htm](https://www.dfs.ny.gov/legal/interpret/lo000110.htm)) states the rule plainly:

> "Section 641(1) states that an entity which acts as an agent of a payee is not engaged in money transmission and need not obtain a money transmission license."
>
> "Agents of a payee must give customers a receipt which indicates that payment to the agent is deemed payment to the payee."

And the **Letter of 20 July 2007** ([lo070720b.htm](https://www.dfs.ny.gov/legal/interpret/lo070720b.htm)) reaches the same place: "the Client would have to be licensed as a money transmitter, unless there is an agency agreement with each of the merchants and no risk to the payor."

**Why this is encouraging and still not an answer.** The 2007 letter's favourable branch turns on funds going "directly … to an account owned by the intended payee". Doula Cloud's funds go directly to a Stripe balance *owned by the Practice under the Practice's own Connected Account Agreement with Stripe* (section 4.4), and from there to the Practice's own linked bank account. Whether a Stripe connected-account balance is "an account owned by the intended payee" for this purpose is a construction question. It is a good-faith reading, and it is a reading, not a holding.

### 5.5 What NYDFS has *not* said — recorded honestly

- **No DFS FAQ, interpretive bulletin, or post-2012 guidance addresses software platforms, SaaS, marketplaces, payment facilitators, or Stripe Connect.** Searches restricted to dfs.ny.gov surfaced only the pre-DFS Banking Department letters above (2000–2011) and the licensing pages.
- **No DFS letter addresses Stripe Connect direct charges by name**, or the merchant-of-record construct at all.
- The DFS legal-interpretations index at `dfs.ny.gov/legal/interpret` **returns 404**; letters resolve only by individual filename. So an exhaustive sweep of DFS letters was not possible, and the memo does not claim one.
- NYDFS's own [money transmitters licensing page](https://www.dfs.ny.gov/apps_and_licensing/money_transmitters) quotes § 641(1) in full including the agent-of-a-payee clause, and names the authorities as "Article 13-B of the Banking Law (Sections 640 to 652-b), and Superintendent's Regulations Parts 406, 416, 417 and 300."

The controlling authority on the branch that matters is a **2007 opinion letter about a fact pattern that predates the whole platform-payments industry**, and letters bind nobody but their addressee. That is the gap counsel is being paid to close.

---

## 6. Whether the answer constrains what the LLC may do

Yes, and this is the part that outlives the memo. The favourable facts in sections 2 and 3 are **not properties of Doula Cloud**; they are properties of one specific integration shape. Each of the following would put the question back on the table, and several are things a 14-doula pilot agency will plausibly ask for.

### 6.1 Guardrails — what must stay true

1. **Direct charges only.** Every Client-facing Stripe object stays on the Practice's connected account via the `Stripe-Account` header. Moving to **destination charges** or **separate charges and transfers** puts the charge on Doula Cloud's balance first — Stripe's own words: "You create a charge on your platform, so the payment appears in your platform's balance." That is receipt of the Client's money by Doula Cloud, and section 5's analysis restarts from a much worse position.
2. **No `application_fee_amount`, ever.** An application fee is not by itself receipt of the payer's funds, but it changes the story a regulator hears and it is the thin end of every indirect flow. Keep the platform's revenue where it is: a separate, direct sale of credits.
3. **No platform balance holding Client money.** No top-ups, no `connect_reserved` strategy, no "we hold it until the Engagement completes". Escrow is on Stripe's **Restricted** business list and is close to the centre of what article 13-B regulates.
4. **`losses_collector` and `fees_collector` stay `stripe`.** Setting `losses_collector: application` shifts negative-balance liability to Doula Cloud, hands it the KYC obligation (section 4.3), and moves the platform toward the marketplace posture Stripe itself associates with platform-as-merchant-of-record.
5. **Stripe-hosted onboarding stays.** Section 4.6: user-hosted onboarding removes the liability cap if a Practice is ever not properly bound to the Connected Account Agreement.
6. **Product copy must not present Doula Cloud as standing between Client and Practice.** Stripe Payments Terms §3.4(b) forbids holding oneself out "as a payment facilitator, intermediary or aggregator". The same words a regulator would read are the words a marketing page is tempted to write. "Get paid through Doula Cloud" is a worse sentence than "invoice your clients from Doula Cloud."

### 6.2 The single most likely feature that would break this

**Paying the contractor Doula.** `engagement_attachments.fee_amount_cents` already records what a Practice owes a contractor for an Engagement (section 2.5), and nothing moves it. A 14-doula agency will ask, sooner or later, for Doula Cloud to pay its contractors out of what the Client paid.

Every obvious way to build that puts Doula Cloud in the flow of funds — separate charges and transfers, a platform balance, Connect payouts to a second connected account. It is precisely the "receiving money for transmission" that § 641(1) names, and New York has no payroll-processor or agent-of-payee statutory exemption to fall back on (section 5.3). **Building that feature is not a product decision; it is an entity decision, and it should not be taken without counsel.**

### 6.3 A superseded research memo that points the other way

`docs/research/stripe-connect-platform-fee-norms.md` (for [#38](https://github.com/markgoho/doula-cloud/issues/38)) recommends a Stripe Connect **Express** tier with a platform `application_fee_amount` of 0.25%–0.35%. That is a real, considered piece of research, and it sits in this same directory.

**It is superseded by [ADR-0007](../adr/0007-connect-account-state-is-two-capabilities-and-a-requirements-list.md) and by this memo.** Its market-comparables data is still good; its recommended integration shape is not the shape that was built, and adopting it would reopen the question this memo exists to close. Anyone reading the two together should treat ADR-0007 as controlling.

### 6.4 The federal picture, briefly, because it differs from New York's

Included only to prevent the common mistake of reading a federal exemption as a New York one.

**31 CFR 1010.100(ff)(5)(ii)** ([eCFR](https://www.ecfr.gov/current/title-31/subtitle-B/chapter-X/part-1010/subpart-A/section-1010.100)) excludes from "money transmitter" a person that only:

> "(B) **Acts as a payment processor to facilitate the purchase of, or payment of a bill for, a good or service through a clearance and settlement system by agreement with the creditor or seller;**"

and

> "(F) Accepts and transmits funds only integral to the sale of goods or the provision of services, other than money transmission services, by the person who is accepting and transmitting the funds."

FinCEN's four conditions on (B) are not in the CFR text — they come from **FIN-2014-R009** ([PDF](https://www.fincen.gov/sites/default/files/administrative_ruling/FIN-2014-R009.pdf)):

> "(a) the entity providing the service must facilitate the purchase of goods or services, or the payment of bills for goods or services (other than money transmission itself); (b) the entity must operate through clearance and settlement systems that admit only BSA-regulated financial institutions; (c) the entity must provide the service pursuant to a formal agreement; and (d) the entity's agreement must be at a minimum with the seller or creditor that provided the goods or services and receives the funds."

**New York has neither exclusion.** No payment-processor exemption, no integral-to-the-sale exemption (section 5.3). A federal analysis that comes out clean says nothing about article 13-B. Do not let anyone conflate them.

---

## 7. The question to put to a New York attorney

Hand over sections 7.1 and 7.2 as they stand. They are written so the hour is spent on the conclusion, not on explaining the business.

### 7.1 Stipulated facts

*Every fact below was verified against the source code and against a live Stripe Sandbox on 25 August 2026, not assumed. Section references point back into this memo, where the evidence sits.*

1. **The company.** A single-member New York LLC (in formation), operating software called Doula Cloud under a DBA. Its customers are doula Practices — small businesses, typically 1 to 15 people. The pilot target is a 14-doula agency in New York.
2. **What the software does.** It is practice-management software: scheduling, care records, messaging, and invoicing. Payments are one feature among many, not the product.
3. **The money flow.** A Practice's own Client pays that Practice's invoice by card. Doula Cloud uses Stripe Connect with **direct charges**: every payment object — Customer, Invoice, InvoiceItem, PaymentIntent, Charge — is created on the **Practice's own Stripe connected account** using Stripe's `Stripe-Account` header. (section 2.1)
4. **The Practice is the merchant of record.** Stripe's own documentation, for direct charges: "The merchant of record is the connected account." (section 4.1)
5. **The Practice contracts with Stripe directly.** Stripe's Connect terms: "Stripe has a direct contractual relationship with each Connected Account under the Connected Account Agreement and will provide the Services directly to each Connected Account." Doula Cloud onboards a Practice by redirecting it into **Stripe's own hosted KYC form**; Stripe, not Doula Cloud, collects the Practice's identity information. (sections 4.3, 4.4)
6. **Doula Cloud takes no cut of the Client's payment.** There is no `application_fee_amount` anywhere in the codebase, no `transfer_data`, no `destination`, and no transfers. (section 2.3)
7. **Stripe's processing fee and any chargeback are borne by the Practice**, by configuration (`fees_collector: stripe`, `losses_collector: stripe`). (sections 2.2, 4.3)
8. **Funds never touch Doula Cloud.** In a live end-to-end walk, a Client paid an $1,800.00 invoice. The full amount, less Stripe's $52.50 fee, settled into **the Practice's** Stripe balance. The platform's balance recorded **nothing**. The platform account's entire balance-transaction history — five entries — is fully accounted for as one $10 sale of Doula Cloud's own software credits plus four entries from Stripe's own built-in sandbox demo fixtures. There are no platform payouts. (sections 3.1 to 3.3)
9. **Doula Cloud has no ability to take the proceeds.** From the Practice's balance the money goes to the Practice's own linked bank account. Doula Cloud has no API call in its codebase that moves funds, and on this configuration no route by which settlement could reach it.
10. **But Doula Cloud does have administrative API authority over the Practice's Stripe account.** Its platform key can create and finalize an Invoice on the Practice's account — that is how invoicing works at all — and Stripe records the platform as `controller.type: application`, `is_controller: true`. It has *no possession* of funds; it does have *reach* over the account. We want this distinction addressed, not glossed. (section 3.4)
11. **Doula Cloud's own revenue is separate and ordinary.** Practices buy prepaid **credits** from Doula Cloud through a Stripe Checkout Session on Doula Cloud's *own* Stripe account. Doula Cloud is the merchant, the Practice is the customer, and the money is earned revenue for Doula Cloud's own software. Credits are spent only against Doula Cloud's own service; they are not redeemable for cash and not usable with any third party. (section 2.4)
12. **Doula Cloud does not pay anyone on a Practice's behalf.** The system records what a Practice agreed to pay a contractor doula — a fee amount on the engagement record — but moves no money for it. Practices pay their contractors entirely outside the software. (section 2.5)
13. **Stripe Payments Company, Stripe's regulated affiliate, states that it "is a U.S. state-licensed money transmitter and federally registered money services business", and is by name "licensed and regulated as a money transmitter by the New York State Department of Financial Services."** (section 4.5)
14. **Stripe has never stated that a Connect platform is exempt from state licensing.** A search of every relevant Connect documentation page found no such statement, and there is no clause in the Stripe Services Agreement in which a platform represents that it is not engaged in money transmission. We are not relying on any Stripe assurance, because none exists. (section 4.7)

### 7.2 The questions

**Q1 — The core question.** On the facts in 7.1, does Doula Cloud "engage in the business of receiving money for transmission or transmitting the same" within **NY Banking Law section 641(1)**, such that it requires a money transmitter licence from the Superintendent?

**Q2 — The controlling authority, as we read it.** NYDFS's [letter of 9 July 2007](https://www.dfs.ny.gov/legal/interpret/lo070709b.htm) concluded that a payment processor required a licence *"except in the situation where credit card payments go directly from credit card accounts to accounts owned by the intended payees,"* reasoning that in that case *"there appears to be no risk to the payor and it appears that there is no receipt of money for transmission by the Client within Banking Law, Section 641."*

We read our facts as falling inside that exception. **Is a Stripe connected-account balance — held by Stripe Payments Company, credited to the Practice, governed by the Practice's own agreement with Stripe, and payable only to the Practice's own bank account — "an account owned by the intended payee" for the purposes of that letter?** If not, what would have to be different for it to be? And how much weight should a 2007 opinion letter, addressed to another party and predating the platform-payments industry, carry in 2026?

**Q3 — Agent of a payee.** Section 641(1) excepts a person acting "as agent of a payee", and 3 NYCRR 406.2(l) defines that as one "authorized by a payee to receive funds on behalf of the payee and to deliver such funds received from the payor to the payee." **On our facts we do not receive funds at all, so we read this exception as unnecessary rather than as our defence.** Is that right — or should we structure the Practice agreement to satisfy DFS's agency conditions anyway (a written agency agreement, a customer receipt stating that payment to the agent is payment to the payee, no risk of loss to the payor), as a belt-and-braces matter? If so, what language should the Practice agreement and the Client-facing invoice carry?

**Q4 — Administrative reach without possession.** Does the platform's ability to create and finalize invoices on a Practice's Stripe account (fact 10), without any ability to take the proceeds, bear on the section 641(1) analysis? We have found no New York authority addressing control without custody, and would rather ask than assume.

**Q5 — The prepaid credits.** Practices pre-pay Doula Cloud for Doula Cloud's own software, in a closed loop with no redemption to third parties and no cash-out (fact 11). Does that raise any issue under article 13-B — "payment instrument" under section 640(5), section 651's permissible-investments requirement, or New York's abandoned-property and gift-certificate rules? We assume not, since the credits are consideration for our own services and section 640(5) excludes an "instrument which is redeemable by the issuer in merchandise or services", but we would like that confirmed rather than assumed.

**Q6 — What would change the answer.** Which of the following would require a licence, so we can put them behind a legal gate in our roadmap rather than discovering it later?

- (a) switching to **destination charges** or **separate charges and transfers**, so the charge lands on our platform balance before reaching the Practice;
- (b) taking an **application fee** — a percentage of each Client payment — as our revenue instead of a separate subscription;
- (c) holding funds on our platform balance at all, even briefly: an escrow or hold-until-completed feature;
- (d) **paying a Practice's contractor doulas out of what the Client paid** — the feature our pilot agency is most likely to request;
- (e) accepting a Client payment for a Practice that has not finished Stripe onboarding.

We expect (a), (c) and (d) to be the dangerous ones. Please tell us if (b) or (e) is worse than we think.

**Q7 — Consequences, and other states.** If a licence *were* required: we understand section 643 sets a surety bond of "no less than five hundred thousand dollars", and section 650(2)(a) makes unlicensed transmission a Class A misdemeanour with no minimum threshold. Is that the correct read of the exposure? And since Practices will eventually be outside New York — does the answer here generalise, or should we expect a state-by-state analysis before selling across state lines? We note New York has **not** adopted the Money Transmission Modernization Act, so its exemption architecture differs from the 31 states that have.

**Q8 — What we should write down.** If the conclusion is that no licence is required, is there anything we should put in our Practice agreement, our Client-facing invoice, or our marketing copy to *keep* it true? Stripe's own contract forbids us to "act as or hold itself out as a payment facilitator, intermediary or aggregator", and we would like our copy checked against the same standard a regulator would apply.

### 7.3 What we are not asking

We are not asking whether the integration is well built, and we are not asking for a federal BSA or FinCEN analysis except where it bears on New York. We know 31 CFR 1010.100(ff)(5)(ii)(B) carries a payment-processor exclusion and that New York has no equivalent. We are not treating the federal answer as the state answer.

---

## 8. Fact versus pending

**Established, verifiable, and re-checkable from this memo:**

- The integration uses direct charges — proven from the code, from the live Sandbox, and against Stripe's own definitions.
- The Practice is the merchant of record; Stripe's fee and any dispute are borne by the Practice.
- Client funds never settle anywhere Doula Cloud possesses them. The platform's whole balance history is reconciled, and none of it is a Client's money.
- Doula Cloud takes no per-transaction cut. Its revenue is a separate direct sale of credits.
- Stripe Payments Company is the NYDFS-licensed money transmitter in this flow, by name.
- Stripe nowhere asserts that a platform is exempt, and no contractual clause says so.
- New York has no payment-processor exemption, has not adopted the MTMA, and imposes no de minimis threshold on section 641(1).
- The nearest NYDFS authority — the 2007 payment-processor letter — describes this fact pattern in its favourable branch.

**Awaiting a New York attorney:**

- Whether a Stripe connected-account balance counts as "an account owned by the intended payee" under that 2007 letter.
- Whether administrative API reach without custody bears on section 641(1).
- Whether the prepaid credit ledger raises any stored-value question.
- Which of the six roadmap changes in Q6 would require a licence.
- **And the conclusion itself.** This memo does not state that no licence is required in New York. It states that Doula Cloud is not in the flow of funds, and that whether anything short of being in the flow of funds still triggers section 641(1) is the question counsel is being paid to answer.

---

## 9. How to re-verify this memo

Everything in sections 2 and 3 is re-checkable in under ten minutes, and should be re-checked before anyone relies on it:

- `grep -rniE "ApplicationFee|TransferData|Destination|V1Transfers|V1Payouts|Topup" api/ --include="*.go"` — expect no money-movement call sites.
- `stripe accounts list` — expect connected accounts whose `controller` reads `fees.payer: account`, `losses.payments: stripe`, `stripe_dashboard.type: full`.
- `stripe charges list` on the platform, and on each connected account — expect Client payments only on connected accounts, with `application_fee_amount`, `transfer_data`, `destination` and `source_transfer` all null.
- `stripe balance_transactions list` on the platform — expect every entry to be Doula Cloud's own credit revenue or a Stripe sandbox fixture, and nothing else.

If any of those stops being true, this memo is out of date and section 6's guardrails have been crossed.
