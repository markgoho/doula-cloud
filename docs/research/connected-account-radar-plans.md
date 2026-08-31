# What Radar plan is a connected account offered in live mode, and can Doula Cloud set it?

Research question, from [#445](https://github.com/markgoho/doula-cloud/issues/445): Doula Cloud is a Stripe Connect platform using direct charges, `controller.fees.payer` (v1) / `defaults.responsibilities.fees_collector` (v2) set to `account`/`stripe`, and Stripe-hosted onboarding. Stripe's hosted onboarding shows every connected account a Fraud protection step offering Radar plans. [#421](https://github.com/markgoho/doula-cloud/issues/421) walked that step in the Sandbox and found test mode forces Radar Pro and disables every other tier, with UI copy reading "Your account includes Radar Pro in test mode, so you can try all available features. You can update your plan before going live." So what a real Practice is offered by default in live mode could not be observed there. [#383](https://github.com/markgoho/doula-cloud/issues/383) made "Radar left on" one of five rules that keep Doula Cloud's money-transmitter finding true, so the platform cannot be indifferent to a Practice turning it off. Live mode is not available — the Stripe platform account is not yet in production (target end of October 2026, per the entity map, [#375](https://github.com/markgoho/doula-cloud/issues/375)).

**This is not a substitute for walking the real live-mode flow once it exists.** It is a record of what Stripe's own documentation, its public pricing page, its API reference, and Doula Cloud's own code and Sandbox each say today, with every claim traced to the source that owns it and labelled by how it was verified. Researched **29 August 2026**.

This document is downstream of [`docs/research/money-transmission-posture.md`](https://github.com/markgoho/doula-cloud/blob/research/money-transmission-posture/docs/research/money-transmission-posture.md) (#383), which established the "Radar left on" guardrail this ticket is checking, and of [#421](https://github.com/markgoho/doula-cloud/issues/421)'s Sandbox walk, which raised this ticket and supplied the test-mode observations quoted above.

## The findings that matter

1. **The per-transaction figures #421 read off the test-mode screen are correct and current.** Stripe's own public pricing page, walked live today, shows Radar Standard at $0.05, Plus at $0.07, and Pro at $0.09 per screened transaction, with Lite free — an exact match to what the Sandbox onboarding step displayed. See §2.
2. **A second, always-on-the-platform fee likely exists that nothing in this codebase had accounted for.** Stripe bills a "Radar account fee" to the platform "always," independent of who pays per-transaction fees; a distinct fee, quantified today at $1.00 per active connected account/month on Standard, $3.00 on Plus, $5.00 on Pro, appears on Stripe's own "platforms and marketplaces" pricing tab. Reading those two facts together, Doula Cloud would owe this second fee for every connected account on a paid tier, scaling with the number of active Practices rather than transaction volume — but whether that pricing-tab fee is the same "Radar account fee" the Connect docs describe, and whether it applies when a full-Dashboard Practice chooses her own plan rather than the platform choosing it for her, was not confirmed against a real billing event. See §2 and the open item in §6.
3. **Radar cannot be turned off — only down to Lite, which is unconditional and free on every Stripe account.** "Radar fraud protection is active by default for all Stripe users," and Radar Lite is the floor of the plan comparison table, not an opt-out. So #383's "Radar left on" guardrail is already true by construction; the real exposure is a Practice being *able* to go no lower than Lite on her own, not Radar being absent. See §3.
4. **Platform Radar rules do not reach direct charges by default, and Doula Cloud's connected accounts can write their own.** Because Doula Cloud creates every account with full Stripe Dashboard access, a Practice can configure her own Radar rules and plan independently of the platform, unless Doula Cloud deliberately changes a Dashboard-only setting to claim control. There is no Accounts API field for any of this. See §4 and §5.
5. **What live mode actually defaults a Practice to remains genuinely unknown**, and this document does not guess at it. It also could not confirm the platform-side "Platform payments controls" setting's current state in the Sandbox, because no logged-in Stripe Dashboard session was available to this research pass. Both are recorded as open items, not resolved. See §1 and §6.

## 1. What live mode offers at the Fraud protection step

**Cannot be observed. Recorded as unresolvable until #387 (production Stripe account) exists**, consistent with the ticket's own framing — this is not guessed at here.

What *is* first-party and available now: the Sandbox onboarding screen's own copy, read by #421, states plainly that Radar Pro is a test-mode-only forced default ("Your account includes Radar Pro in test mode... You can update your plan before going live") — which is Stripe telling the operator, in its own UI text, that the live-mode default differs from test mode's forced Pro. It does not say what that live default actually is, and it does not say whether Lite becomes genuinely clickable (its Sandbox link is `aria-disabled`) once live. Both remain open.

**A checked absence worth recording:** Stripe's own step-name reference for the Account onboarding embedded component (`docs.stripe.com/connect/supported-embedded-components/account-onboarding`) enumerates every step a connected account can land on — `business_type`, `business_details`, `representative_details`, `owners`, `external_account`, `support_details`, `climate`, `tax`, `summary`, and a dozen more — and none of them is named for Radar or fraud protection. The Fraud protection screen #421 walked either falls under an undocumented step name, or the step-name table is incomplete for it. Either way, there is no Stripe documentation page that describes this specific step's behavior, defaults, or live-mode differences in prose — only the UI copy observed live in the Sandbox.

**Recommendation:** re-walk the identical hosted-onboarding flow the moment production Stripe credentials exist (#387), before any pilot Practice is sent through it, and update this document with what live mode actually shows.

## 2. Published per-transaction pricing, and who pays it under `fees.payer = account`

**Verified live** against `stripe.com/radar/pricing` on 29 August 2026 (public marketing page, no authentication required, read via a real browser since the pay-as-you-go figures sit behind a "Show pricing" toggle that a plain fetch does not render).

**"For your business" pricing** (what a connected account, as a standalone Stripe user, is shown):

| Plan | Pay-as-you-go, per screened transaction | Subscription equivalent |
| --- | --- | --- |
| Lite | Free, included at no additional charge | — |
| Standard | $0.05 | $10/month, includes 200 transactions, then $0.05/additional |
| Plus | $0.07 | $14/month, includes 200 transactions, then $0.07/additional |
| Pro | $0.09 (+ $0.005 per screened customer) | $20/month, includes 200 transactions and 400 screened customers, then $0.09 / $0.005 per additional unit |

This is an exact match to the dollar figures #421 read off the Sandbox's forced-Pro screen and to the figures quoted in this ticket's own framing.

**Who pays, under Doula Cloud's actual configuration.** Doula Cloud's account-creation code sets `Defaults.Responsibilities.FeesCollector = stripe` (Accounts v2) — `api/internal/payments/stripe_api_client.go:84` — which `docs.stripe.com/connect/direct-charges-fee-payer-behavior` confirms is the v2 equivalent of v1 `controller.fees.payer = account`, described there as: *"Stripe collects fees directly from your connected account. We don't charge any Connect fees to it or to your platform."* `docs.stripe.com/connect/radar`'s "Radar fees" section states: *"Stripe charges Radar account fees to the platform always, and Radar transaction fees based on the rate for the account that collected the payment."* Under direct charges, the connected account is the one that collects the payment, so the per-transaction Radar fee above is billed at the Practice's own Radar plan rate, and — per the fee-payer behavior confirmed against Doula Cloud's own code — collected directly from the Practice, not from Doula Cloud.

**The fee that is not the Practice's.** The same "Radar account fee" sentence names a *second* charge that Stripe bills "to the platform always," with no exception stated for fee-payer configuration. **"For platforms and marketplaces" pricing**, read from the same live page today, quantifies it:

| Plan | Per-active-connected-account fee (billed to the platform) |
| --- | --- |
| Lite | None (free) |
| Standard | $1.00/month |
| Plus | $3.00/month |
| Pro | $5.00/month |

So for every Practice whose connected account sits on a paid Radar tier, Doula Cloud owes Stripe this fee directly, regardless of `fees.payer` and regardless of how many transactions that Practice screens. It scales with the count of active connected accounts, not with volume — a real, previously unquantified line item once the pilot's roughly 50 Practices are onboarded.

## 3. Can a Practice decline Radar entirely?

**No — confirmed, not inferred.** `docs.stripe.com/radar/supported-payment-methods`: *"Radar fraud protection is active by default for all Stripe users."* The same page adds, specifically for the free tier: *"Radar Lite users: Stripe might still block payments for a variety of reasons, including but not limited to fraud risk, sanctions, and other compliance and regulatory considerations."* The plan comparison table at `docs.stripe.com/radar/how-radar-works#compare-plans` shows Radar Lite already supporting "essential fraud prevention for card payments," "card testing prevention," and "fraud alerts" — there is no tier below it and no toggle to disable Radar outright.

So "declining Radar" in practice means dropping from a paid tier to Lite, not switching fraud protection off. Lite is the unconditional floor on every Stripe account, connected or otherwise. This reframes what #383's "Radar left on" rule is actually protecting against: not the possibility of Radar being absent (Stripe does not allow that), but the possibility of a Practice sitting at nothing more than the free baseline rather than a paid tier with fuller coverage.

## 4. Can the platform set, default, or constrain a connected account's Radar plan?

**Yes, via a Dashboard-only setting — no Accounts API field exists for it (checked absence).** `docs.stripe.com/radar/risk-settings#apply-radar-rules-to-direct-charges` documents **Settings → Radar → Platform controls for direct charges → Platform payments controls**, with three modes:

- **"Only connected accounts"** — connected accounts manage their own Radar rules, settings, *and plan*. One override exists here: *"If your platform pays the fees for your connected accounts, you can override connected account plans with the free Radar Lite plan."* That override is explicitly conditioned on the platform being the fee payer. Doula Cloud's accounts are `fees.payer = account`, the opposite configuration — so **this specific override path is checked and confirmed not available to Doula Cloud as configured today.**
- **"Only my platform"** — *"Your platform manages Radar rules, settings, and plans... This setting overrides a connected account's Radar rules, if they exist."* No fee-payer condition is stated for this mode anywhere in the documentation. Read literally, this is the lever that would let Doula Cloud pick the Radar plan for every Practice outright, regardless of who pays for it. This reading is **inferred from the doc's plain text, not verified against Doula Cloud's own Sandbox** — see the gap noted in §6.
- **"Both my platform and connected accounts"** — shared control; Radar evaluates platform rules first, then the connected account's.

**Checked absence, verified against live objects, not just documentation:** two real Sandbox accounts were retrieved in full via the Stripe API — the platform's own account and the connected account #421 walked (`acct_1U9c2i1rKoEV0BlC`). Neither object's `settings` hash, which carries every other per-product configuration block (`card_payments`, `payouts`, `branding`, `invoices`, and so on), contains a `radar` key or anything naming a plan or tier. There is no field for a Radar plan on the v1 `Account` object as actually returned by Stripe today, and no way found to set a connected account's Radar plan at account-creation time or any other time through the API — the "Platform payments controls" Dashboard setting above is the only lever found. See Sources.

## 5. Do the platform's Radar rules apply to direct charges?

**No, not by default — confirmed from two independent pages.** `docs.stripe.com/connect/radar`: *"Direct charges: Paid directly to a connected account; Stripe applies only the collecting account's Radar configuration and rules."* `docs.stripe.com/connect/integration-recommendations` states it more sharply, in the context of exactly Doula Cloud's shape: *"Connected accounts can't define their own Radar rules without the full Stripe Dashboard. Also, your platform Radar rules don't apply to direct charges made on your connected accounts. To configure Radar rules for a connected account, your platform must set them up using the View Dashboard as this account feature."*

Doula Cloud creates every connected account with `Dashboard: stripe.V2CoreAccountDashboardFull` (`api/internal/payments/stripe_api_client.go:88`, confirmed independently by #383's Sandbox walk) — full Stripe Dashboard access. That is exactly the condition under which a Practice *can* write and hold her own Radar rules and plan, independent of the platform. So today, absent the Dashboard change described in §4, Doula Cloud's platform-level Radar posture has no effect on a Practice's charges at all — #383's "Radar left on" guardrail depends entirely on what each individual Practice does with her own account, not on anything Doula Cloud enforces centrally.

## 6. What this does not answer

- **What live mode actually offers or defaults to at the Fraud protection step**, and whether Lite is genuinely clickable there. Unresolvable until #387 exists. Do not treat the Sandbox's forced-Pro behavior as informative about the live default.
- **Whether "Only my platform" mode actually works under `fees.payer = account`, and what it costs.** The reading in §4 is inferred from the plain text of Stripe's documentation. This session could not log into the Doula Cloud Sandbox Dashboard (`dashboard.stripe.com/test/settings/radar`) to read the current "Platform payments controls" setting or exercise a mode change, because no authenticated Stripe Dashboard session was available in the browser profile this research pass had access to. **Recommended next step:** an operator with Dashboard access should open Settings → Radar → Platform controls for direct charges in the Sandbox, record the current setting (its default was not stated anywhere in the documentation reviewed), and if switching to "Only my platform" is being considered, confirm in the Sandbox whether Stripe still bills the per-transaction Radar fee to the connected account or shifts it to the platform under that mode — the documentation is silent on this specific interaction.
- **Whether the "Radar account fee" ($1.00 / $3.00 / $5.00 per active connected account) is charged even when the connected account itself — not the platform — selected its own paid Radar plan through hosted onboarding**, as opposed to a plan the platform assigned. `docs.stripe.com/connect/radar` says the account fee goes to the platform "always," without qualifying that by who chose the plan, but this was not confirmed against a live billing event.
- **What #383's rule requires Doula Cloud to do, if anything, now that "Radar left on" is understood to mean "at least Lite" rather than "a paid tier."** That determination, and any resulting work, is left to the parent session per this ticket's scope — this document supplies the facts, not the decision.

## Sources

All retrieved 29 August 2026.

**Stripe documentation (docs.stripe.com)**
- [How Radar works — Compare Radar plans](https://docs.stripe.com/radar/how-radar-works#compare-plans) — plan feature comparison; Lite as the unconditional floor; "Radar fees" section quoting the account-fee-to-platform-always / transaction-fee-by-collecting-account split
- [Use Radar with Connect](https://docs.stripe.com/connect/radar) — direct vs. transferred charge Radar behavior; "If your platform pays the fees for your connected accounts, you can choose whether your connected accounts use Radar Lite" conditional override
- [Risk setting and risk controls — Apply Radar rules to direct charges](https://docs.stripe.com/radar/risk-settings#apply-radar-rules-to-direct-charges) — the three "Platform payments controls" modes and their stated conditions
- [Supported payment methods with Radar](https://docs.stripe.com/radar/supported-payment-methods) — "Radar fraud protection is active by default for all Stripe users"; Radar Lite users still subject to blocking
- [Recommended Connect integrations and charge types](https://docs.stripe.com/connect/integration-recommendations) — "your platform Radar rules don't apply to direct charges made on your connected accounts"; full-Dashboard requirement for a connected account to write its own rules
- [Fee behavior on connected accounts](https://docs.stripe.com/connect/direct-charges-fee-payer-behavior) — `account` (v1) / `stripe` (v2 `fees_collector`) behavior: "Stripe collects fees directly from your connected account. We don't charge any Connect fees to it or to your platform."
- [Migrate your Connect integration to use controller properties](https://docs.stripe.com/connect/migrate-to-controller-properties) — `controller.fees.payer` default value `account`, and its possible values
- [Account onboarding (Connect embedded component) — Step names](https://docs.stripe.com/connect/supported-embedded-components/account-onboarding?platform=web) — the full documented step-name enumeration, checked for and found without a Radar/fraud-protection entry

**Stripe API reference and live Sandbox objects**, checked via the Stripe MCP `stripe_api_read` tool against the Doula Cloud Sandbox: the platform's own account (`acct_1U7N3e1rKoVEA79v`) and the connected account #421 walked (`acct_1U9c2i1rKoEV0BlC`) were both retrieved in full via `GET /v1/accounts/{account}`. Neither object's `settings` hash — which carries every other per-product configuration block (`card_payments`, `payouts`, `branding`, `invoices`, `bacs_debit_payments`, `sepa_debit_payments`, `card_issuing`) — contains a `radar` key or anything naming a plan or tier. This is a directly observed absence on real Sandbox objects, not a schema browse.

**Stripe public pricing page**
- [stripe.com/radar/pricing](https://stripe.com/radar/pricing) — walked live via a real browser session (the pay-as-you-go per-transaction and per-active-account figures render only after clicking each plan's "Show pricing" control and are not present in the page's static markup). "For your business" tab: $0.05 / $0.07 / $0.09 per screened transaction for Standard / Plus / Pro, Lite free. "For platforms and marketplaces" tab: the same per-transaction rates plus $1.00 / $3.00 / $5.00 per active connected account for Standard / Plus / Pro.

**Doula Cloud's own code and Sandbox**
- `api/internal/payments/stripe_api_client.go:84` — `Defaults.Responsibilities.FeesCollector = stripe.V2CoreAccountDefaultsResponsibilitiesFeesCollectorStripe`, confirming Doula Cloud's connected accounts use the fee-payer-account model
- `api/internal/payments/stripe_api_client.go:88` — `Dashboard: stripe.V2CoreAccountDashboardFull`, confirming full Stripe Dashboard access on every connected account
- [#421](https://github.com/markgoho/doula-cloud/issues/421) — the Sandbox walk that first read the forced-Pro Fraud protection screen and its "before going live" UI copy
- [#383](https://github.com/markgoho/doula-cloud/issues/383) / [`docs/research/money-transmission-posture.md`](https://github.com/markgoho/doula-cloud/blob/research/money-transmission-posture/docs/research/money-transmission-posture.md) — direct-charge and `dashboard: full` findings, corroborated independently here from the code
- Stripe Dashboard (`dashboard.stripe.com/test/settings/radar`) — attempted, not completed. No authenticated session was available in this research pass's browser profile; the platform's current "Platform payments controls" setting remains unread. See §6.
