# Stripe Connect platform application fee — market norms

> [!WARNING]
> **Superseded. Do not build from this document.**
>
> This research assumed a Stripe Connect **Express** account with the platform taking an `application_fee_amount`. Neither survived. [ADR-0007](../adr/0007-connect-account-state-is-two-capabilities-and-a-requirements-list.md) settled the integration on **Accounts v2** with the `merchant` configuration, and [#383](https://github.com/markgoho/doula-cloud/issues/383) established from the code and a live Sandbox walk that the integration uses **direct charges** with **no application fee anywhere** — the Practice is merchant of record, Stripe's fee is charged to the Practice, and no money ever reaches a platform balance.
>
> That is not an implementation detail. [`money-transmission-posture.md`](money-transmission-posture.md) records that staying out of the flow of funds is what keeps the money-transmitter question answerable, and **New York has no payment-processor exemption and has not adopted the MTMA**. Reviving the shape recommended below would reopen that question in the least favourable state.
>
> Kept for the market-rate figures, which are still accurate, and so the reasoning is not silently lost. The recommendation is not.

Research for GitHub issue #38 ("Stripe integration shape"). Question: if Doula
Cloud offers a Stripe Connect Express "we handle it" tier, what should the
platform's own `application_fee_amount` be, on top of Stripe's own
processing cut (~2.9% + $0.30 US card baseline)?

## Recommendation

**0.25%–0.35% of transaction value, no added flat fee**, applied as the
`application_fee_amount` on Express-tier charges. Do not use a percentage-of-
revenue marketplace commission (e.g. Mindbody's 20% marketing fee) — that is
a lead-gen commission model, not a payment-processing convenience fee, and is
not the right comparable for this decision.

Rationale:

- The closest vertical comparable (SimplePractice, health/wellness practice
  management) visibly marks up standard card processing by **~0.25
  percentage points** (3.15% vs. Stripe's 2.9% baseline), keeping the flat
  $0.30 unchanged. That is direct evidence of what a comparable buyer will
  tolerate as a "we handle it for you" convenience markup.
- 0.25%–0.35% is also enough for Doula Cloud to recover what Stripe itself
  charges the platform for running Express accounts: **$2/active
  account/month + 0.25% + $0.25 per payout** (Stripe's own Connect pricing
  page). At typical practice transaction volumes this cost is more than
  covered by a 0.25%+ application fee.
- The "bring your own Stripe account" (Standard Connect, no platform fee)
  tier is directly validated by Setmore, which charges **zero** platform
  markup on top of Stripe/Square when a practice connects their own account
  — confirming that a genuine $0-fee self-managed tier is a normal, existing
  market pattern, not unusual.
- Stripe's own Platform Pricing Tool documentation uses illustrative
  percentage fees in the sub-1% range (e.g. a 0.45% variable fee example),
  reinforcing that small percentage-point application fees, not large
  commissions, are the expected shape of a Connect platform fee.

## Comparable data points

| Platform | Vertical | Payment processing rate charged to merchant | Platform markup vs. ~2.9%+30¢ baseline | Source |
|---|---|---|---|---|
| Jane App | Health/wellness practice management (closest comparable to a doula practice) | 2.85% + $0.25 online; 2.6% + $0.10 in-person | At or below baseline — effectively near-cost pass-through | [jane.app/pricing](https://jane.app/pricing) (primary, fetched Aug 2026) |
| SimplePractice | Health/wellness practice management | 3.15% + $0.30, all transactions | **+0.25 points** over 2.9% baseline | [SimplePractice support: Processing online payments](https://support.simplepractice.com/hc/en-us/articles/360022512232-Processing-online-payments) (primary) |
| Setmore | Appointment scheduling, bring-your-own-processor model | Pass-through of Stripe (2.9%+30¢) or Square rates; "Setmore does not charge any additional fees" | **0% markup** — validates a genuine no-fee Standard/BYO tier | [setmore.com/integrations/stripe](https://www.setmore.com/integrations/stripe) (primary) |
| Vagaro | Salon/spa booking | 2.6%+10¢ to 3.5%+15¢ depending on volume tier and entry method | Not cleanly separable (Vagaro is a full merchant-services reseller, not a thin Connect pass-through) — directional only | [Vagaro support: US Credit Card Processing Rates and Fees](https://support.vagaro.com/hc/en-us/articles/115000595607-United-States-Credit-Card-Processing-Rates-and-Fees) (primary) |
| Housecall Pro | Field/home services | 2.49%–3.49% depending on entry method; 1% ACH | Not cleanly separable from raw interchange — directional only | [Housecall Pro Help Center: Payment Processing Options](https://help.housecallpro.com/en/articles/2046930-housecall-pro-payment-processing-options) (primary) |
| Mindbody | Fitness/wellness studio booking | 2.75%–3.5% base processing; **separately**, a 20% marketplace/marketing fee applies only to bookings driven through Mindbody's own marketing tools | Base processing markup unclear (contact-sales pricing); the 20% figure is a distinct lead-gen commission, not a processing fee — do not conflate | [Mindbody: Invoice and Marketing Platform fees FAQ](https://support.mindbodyonline.com/s/article/217038307-What-are-Marketing-Platform-fees) (primary) |
| Square Appointments | Salon/services booking | 2.6%+15¢ to 2.9%+30¢ (in-person), tiered by subscription plan | Square is both platform and processor (no separate application fee); rate varies by subscription tier, not a Connect-style markup | [squareup.com/us/en/pricing](https://squareup.com/us/en/pricing) (primary) |

## Stripe's own guidance (primary)

- **Standard US card rate**: 2.9% + $0.30 per successful charge.
  [stripe.com/connect/pricing](https://stripe.com/connect/pricing)
- **Express account cost to the platform**: $2 per monthly active connected
  account, plus 0.25% + $0.25 per payout sent.
  [stripe.com/connect/pricing](https://stripe.com/connect/pricing)
- **Platform Pricing Tool** (Dashboard: Settings > Connect > Platform
  pricing) lets a platform define application fees as fixed, variable
  (percentage), or blended (percentage + flat), with optional min/max caps
  and markup/discount modifiers. Stripe's own worked example in the docs
  uses a 0.45% variable fee and 4%/3% markup/discount modifiers as
  illustrations of the mechanism — not an explicit "typical" recommendation,
  but indicative of the expected order of magnitude (sub-1% variable fees).
  [docs.stripe.com/connect/platform-pricing-tools/pricing-schemes](https://docs.stripe.com/connect/platform-pricing-tools/pricing-schemes)
- Stripe does not publish a stated "typical platform fee %" for vertical
  SaaS; it explicitly leaves this to platform discretion (per the pricing
  scheme docs above).

## Note on sourcing

All figures above were pulled directly from vendor pricing/support pages or
Stripe's own docs (fetched August 2026), not from secondary "best practices"
roundups. Third-party aggregator pages (e.g. Capterra, GlossGenius,
Koalendar pricing-guide sites) surfaced in search results were not used as
sources — only as pointers to the primary page, which was then fetched
directly.
