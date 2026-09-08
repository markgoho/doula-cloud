# Can a Client pay inside Doula Cloud's own chrome, on Connect Standard?

Research for [#980](https://github.com/markgoho/doula-cloud/issues/980), a ticket of map [#333](https://github.com/markgoho/doula-cloud/issues/333). Every claim below is either an observation against the live Stripe Sandbox (object and event ids given) or a citation to docs.stripe.com.

## Short answer

**Yes.** A Client can pay a connected account's Invoice from a page Doula Cloud serves, with the Payment Element, on a **Connect Standard** account, and it needs no new Stripe capability, no OAuth token, and no change to the charge architecture `#38` locked. The mechanism is a **direct charge**: expand the finalized Invoice's `confirmation_secret` to get the PaymentIntent's `client_secret`, hand that to the browser, and mount Elements with `Stripe(platformPublishableKey, { stripeAccount: 'acct_...' })`.

The **by-hand rail has no payable Stripe surface at all** — no Stripe object exists, so there is nothing to pay. That is not a gap to close with a smarter Elements integration; it can only change by creating a Stripe object, which is exactly what `#271` decided not to do on that rail.

## Sandbox setup used

| Thing | Value |
| --- | --- |
| Platform account | `acct_1U7N3e1rKoVEA79v` ("Doula Cloud sandbox"), publishable key `pk_test_51U7N3e...zzm` |
| Connected account | `acct_1UDCmj1rKosUjGsh` ("Riverside Doulas"), the account `#271` used |
| Connected account type | `type: "standard"`; `controller` = `{fees.payer: "account", losses.payments: "stripe", requirement_collection: "stripe", stripe_dashboard.type: "full", type: "application"}` — the Standard shape |
| API version | `2026-07-29.dahlia`, the version `stripe-go/v86 v86.3.0` pins (`api_version.go:10`), and the version every event below carries |
| Customer | `cus_VDt4CfOli6xPHM` |
| Invoice A (paid via a PaymentIntent confirm, the Elements path) | `in_1UDRIp1rKosUjGshqgg47uNF`, number `RL3B9CYF-0001` |
| Invoice B (walked on the hosted page) | `in_1UDRPg1rKosUjGsho9DRJsYo`, number `RL3B9CYF-0002` |

Both Invoices were raised the way `api/internal/payments/stripe_api_client.go:301` raises one: `collection_method=send_invoice`, `days_until_due=30`, one InvoiceItem, every call carrying `Stripe-Account: acct_1UDCmj1rKosUjGsh`.

## Question by question

### Can Payment Element collect payment against that Invoice's PaymentIntent from a page we serve?

Yes. Finalizing with `expand[]=confirmation_secret` returns the payable handle directly:

```
POST /v1/invoices/in_1UDRIp1rKosUjGshqgg47uNF/finalize?expand[]=confirmation_secret
Stripe-Account: acct_1UDCmj1rKosUjGsh

"status": "open",
"number": "RL3B9CYF-0001",
"hosted_invoice_url": "https://invoice.stripe.com/i/acct_1UDCmj1rKosUjGsh/test_YWNjdF8xVURDbWoxcktvc1VqR3NoLF9WRHQ0MjhiMUNMODlzMzVBN3Jwc3VPTVpTMVVZeWhyLDE3OTQyMzU1MQ0200BvQZdwoB?s=ap",
"confirmation_secret": { "client_secret": "pi_3UDRIw1rKosUjGsh0EpGvpzg_secret_dM4LPxIV9Sk8VBdgSuyCmX3x5", "type": "payment_intent" }
```

Expanding `payments` on the same Invoice shows the underlying object: `inpay_1UDRIw1rKosUjGsh6Rl7MRDv`, `payment.payment_intent = pi_3UDRIw1rKosUjGsh0EpGvpzg`, `amount_requested: 45000`.

It needs exactly three things, and no more:

1. **The client secret** — `invoice.confirmation_secret.client_secret`, available on `finalize` and on any later `retrieve`/`update` of the finalized Invoice. Stripe's own guide says so: "Get the invoice's client secret by expanding its `confirmation_secret` attribute when finalizing the invoice or when making another API call, such as retrieve or update, on the invoice after you finalize it" — [Integrate with the Invoicing API](https://docs.stripe.com/invoicing/integration).
2. **The platform's own publishable key.** Not a per-Practice key. The connected account is addressed by option, not by key.
3. **`stripe.js` loaded from `js.stripe.com`, constructed with `stripeAccount`** — `Stripe(pk, { stripeAccount: 'acct_1UDCmj1rKosUjGsh' })`. This is the documented Connect direct-charge form; the Express Checkout Element guide states it for the same reason: "Connect platforms that either create direct charges or add the token to a customer on the connected account must take additional steps. 1. On your frontend, before creating the ExpressCheckoutElement, set the `stripeAccount` option on the Stripe instance" — [Accept a payment with the Express Checkout Element](https://docs.stripe.com/elements/express-checkout-element/accept-a-payment?payment-ui=elements).

**Observed, not just read.** A 25-line page served from `http://localhost:8931/pay.html` did `Stripe(pk_test_51U7N3e..., { stripeAccount: 'acct_1UDCmj1rKosUjGsh' })`, `stripe.elements({ clientSecret: 'pi_3UDRIw...secret_dM4LPxIV9Sk8VBdgSuyCmX3x5' })`, `elements.create('payment').mount('#pe')`, and driven in a real Chrome the Payment Element fired its `ready` event — the page printed `ELEMENT_READY`. The console warnings it emitted are themselves evidence the Element resolved the *connected* account's payment-method configuration, not the platform's: it named `cashapp` and `klarna` as "not activated" and `apple_pay` as blocked pending domain registration, and the account's own `capabilities` list carries `klarna_payments: active` but no Cash App. Stripe.js also warned "You may test your Stripe.js integration over HTTP. However, live Stripe.js integrations must use HTTPS."

The browser-side `stripe.confirmPayment` was **not** exercised in this run: the automation relay could not fill the Element's cross-origin iframe, so the confirm was made from the API instead, with `pm_card_visa` and the platform secret key under `Stripe-Account`. The Element's `ready` event is what proves the publishable-key + `stripeAccount` + client-secret path authenticates against the connected account's PaymentIntent; the confirm below proves that PaymentIntent is payable as a direct charge. Read them together, and read the docs citation above for the browser-side call itself.

```
POST /v1/payment_intents/pi_3UDRIw1rKosUjGsh0EpGvpzg/confirm
Stripe-Account: acct_1UDCmj1rKosUjGsh
payment_method=pm_card_visa

"status": "succeeded", "amount": 45000,
"latest_charge": "ch_3UDRIw1rKosUjGsh0bQU53aG",
"application": "ca_V7cTvhB0N2IFBX3IQacXkKvU1UOVO3v4"
```

`application` being the platform's Connect application id is the signature of a direct charge: the charge lives on the connected account and is attributed to Doula Cloud as the platform. The Invoice moved to `paid` off the back of it.

### Does the platform holding no card data change what Elements can do, and does Standard restrict it?

No, and no.

Holding no card data is the *point* of Elements, not a limitation on it. The Element renders inside a Stripe-owned iframe served from `js.stripe.com` and posts the card straight to Stripe; the card never touches a Doula Cloud origin, a Doula Cloud server, or a Doula Cloud log. Stripe: "A Stripe Element contains an iframe that securely sends the payment information to Stripe over an HTTPS connection" — [Save a customer's payment method](https://docs.stripe.com/payments/save-and-reuse?payment-ui=elements). Doula Cloud only ever moves an opaque `client_secret`, which is scoped to one PaymentIntent and exposes only its status, amount, and currency.

Standard imposes nothing extra here. The `stripeAccount` mechanism is documented as the requirement for "Connect platforms that either create direct charges or add the token to a customer on the connected account", with no carve-out by controller type, and the run above is a Standard account (`type: "standard"`, `stripe_dashboard.type: "full"`) that took the charge. The one Connect-specific chore that *does* apply to any controller type is **domain registration for wallets**: the Sandbox refused Apple Pay on an unregistered domain, and Stripe's Express Checkout guide says to "Register all of the domains where you plan to show the Express Checkout Element". Card, Link, Klarna and the rest render without it.

### Is there an embeddable Stripe component for a hosted invoice?

**No.** The hosted invoice page is redirect-only. Nothing in [Supported Connect embedded components](https://docs.stripe.com/connect/supported-embedded-components) is payer-facing: account onboarding, payments, payment details, disputes list, disputes for a payment, payouts, issuing cards list. Connect embedded components exist to "add connected account dashboard functionality to your website" — they are for the *Practice*, not for the Client, and every one of them is gated behind an `AccountSession` created for a connected account. There is no `invoice-payment` component and no iframe embed of `hosted_invoice_url`.

The embeddable payer-side surfaces Stripe does offer are Elements (above) and Checkout in `ui_mode: embedded`. Elements against `confirmation_secret` is the one that pays *this* Invoice; an embedded Checkout Session would be a second, parallel charge with no link to the Invoice object.

### What does the redirect look and feel like?

Walked live in Chrome at Invoice B's `hosted_invoice_url`. Accessibility snapshot of the real page:

- **"Riverside Doulas"** at the top, and a **"Sandbox"** badge (test mode's own marker; in live mode this badge is absent).
- `$450.00`, "Due October 8, 2026", "Invoice PDF" download button.
- A summary table: To "Research 980" / From **"Riverside Doulas"** / Invoice "#RL3B9CYF-0002".
- The payment form: "Choose how you'd like to pay", a "Save payment details to Riverside Doulas for future purchases" checkbox, and a "Pay" button.
- Footer: **"Powered by Stripe"**, plus Stripe's own **Terms** and **Privacy** links.

So the branding is the **Practice's**, not Doula Cloud's. The name shown is the connected account's `settings.dashboard.display_name` ("Riverside Doulas"). Logo, icon and brand color come from the connected account's `settings.branding` — Stripe: "The hosted invoice page is customizable with your: Brand color, Logo, Icon" — [Hosted invoice page](https://docs.stripe.com/invoicing/hosted-invoice-page). On this account all four are `null` (`{"icon": null, "logo": null, "primary_color": null, "secondary_color": null}`), so it renders Stripe's default gray. **Doula Cloud's brand cannot appear on that page at all**, and a Practice that never fills in `settings.branding` gets a page a Client has no reason to recognize.

**There is no return URL.** The finalized Invoice object carries exactly one URL field — `hosted_invoice_url` — and no `success_url`, `return_url`, or `redirect_on_completion` anywhere on the object or in the create/finalize parameters. After payment the Client stays on `invoice.stripe.com` looking at a Stripe receipt. The only navigation back to Doula Cloud is the browser's back button. That is the sharp difference from Elements, where `stripe.confirmPayment` takes a `return_url` we choose and card payments land back on it immediately.

Two further hosted-page facts worth carrying: **the URL expires** — "Invoice URLs expire 30 days after the due date… In all cases, the expiration window is never longer than 120 days" — so a stored `hosted_invoice_url` is not a durable "pay" link for an old balance; and Stripe's *saved payment method* checkbox is on by default for new accounts (`settings.invoices.hosted_payment_method_save: "offer"` on this account), meaning a Client can be asked to save a card to the Practice on a page Doula Cloud does not control.

### Which surface fires which webhooks, and does `handleInvoicePaid` work unchanged?

Both surfaces converge on the same events, because both end in the same PaymentIntent confirming against the same Invoice. Events read from `/v1/events` on the connected account after the Elements-path payment, newest first:

| Event id | Type | `data.object.id` |
| --- | --- | --- |
| `evt_1UDRPW1rKosUjGshdUv8lYhY` | `invoice.payment_succeeded` | `in_1UDRIp1rKosUjGshqgg47uNF` |
| `evt_1UDRPV1rKosUjGsh0s1wIGWG` | **`invoice.paid`** | `in_1UDRIp1rKosUjGshqgg47uNF` |
| `evt_1UDRPW1rKosUjGshMAfCjJmd` | `invoice.updated` | `in_1UDRIp1rKosUjGshqgg47uNF` |
| `evt_3UDRIw1rKosUjGsh0xgdZpsQ` | `payment_intent.succeeded` | `pi_3UDRIw1rKosUjGsh0EpGvpzg` |
| `evt_3UDRIw1rKosUjGsh0BlX6aSi` | `charge.succeeded` | `ch_3UDRIw1rKosUjGsh0bQU53aG` |
| `evt_1UDRIx1rKosUjGshAqmNLmTA` | `invoice.finalized` | `in_1UDRIp1rKosUjGshqgg47uNF` |

`api/internal/payments/webhook.go` switches on `invoice.paid` and `invoice.payment_failed` only, and `resolveInvoiceForEvent` keys off `data.object.id` against `invoices.stripe_invoice_id` plus the account id against `practices.stripe_connect_account_id` (`webhook.go:190-209`). Neither of those inputs differs by surface — the Invoice id and the account id are identical whichever way the Client paid. **`handleInvoicePaid` works unchanged for both, with no code change and no new event subscription.** Nothing in `webhook.go` reads `payment_intent` off the Invoice, which matters because on `2026-07-29.dahlia` the Invoice no longer carries a top-level `payment_intent` field at all — the PaymentIntent is reached through `payments.data[].payment.payment_intent` or `confirmation_secret`.

Every event carried `"api_version": "2026-07-29.dahlia"`, matching what `stripe-go/v86` expects, so `ConstructEvent`'s version check passes.

### What does the platform's PCI obligation become under each option?

- **Hosted invoice page (redirect).** Doula Cloud serves no payment form and touches no card field. This is the lowest-obligation posture Stripe offers.
- **Payment Element (in-chrome).** Doula Cloud serves a page that *contains* a Stripe-owned iframe but never a card input of its own. Stripe describes exactly this as SAQ A: Elements "provides prebuilt UI components and offers simple PCI compliance with **SAQ A** reporting" — [Migrate to the Payment Intents API](https://docs.stripe.com/payments/payment-intents/migration). The controls that come with it are ordinary and mostly already true of the app: TLS 1.2 or above on the payment page ([Integration security guide](https://docs.stripe.com/security/guide)), Stripe.js loaded live from `js.stripe.com` and never bundled or self-hosted (stated in every Elements guide), and an annual self-attestation in the Dashboard's compliance documents.

So the real delta is: a redirect means no PCI artifact of our own; Elements means an annual **SAQ A** self-assessment, plus a standing rule that `stripe.js` is never vendored into the SvelteKit bundle. That is a modest, recurring cost — not a project.

### The by-hand Invoice

**There is no payable Stripe surface for a by-hand Invoice, and there cannot be one.** `#271` made `invoices.stripe_invoice_id` nullable and creates the row `open` with no Stripe call at all. Every mechanism above starts from a Stripe object: `confirmation_secret` is a field on a Stripe Invoice, `hosted_invoice_url` is a field on a Stripe Invoice, and a PaymentIntent has to be created against a connected account before Elements has a secret to mount with. With no Stripe object there is no client secret, no hosted page, and nothing for a webhook to report.

The only ways to make a by-hand Invoice payable in-chrome are (a) raise a Stripe Invoice for it after all, or (b) create a bare PaymentIntent or Checkout Session on the connected account for the same amount and reconcile it back to the by-hand row by hand. Both are "the Practice is on Stripe after all", which is precisely the premise `#271` set out to make optional. Neither is a Stripe limitation to work around; it is the by-hand rail's defining property. The portal's honest answer on that rail is to show the balance and the human-readable reference and say how to pay offline — not to offer a button.

## Sandbox versus live mode

Four differences, none of which change the answer:

1. **The "Sandbox" badge** on the hosted invoice page. Live mode does not render it.
2. **Test cards.** `pm_card_visa` / `4242 4242 4242 4242` only work in test mode.
3. **Invoice emails.** Not exercised in this run — whether test-mode finalization delivers mail to a real inbox was not verified, so the Sandbox has not been shown to demonstrate what a Client actually receives when `FinalizeInvoice` runs. What is documented either way is that the Dashboard's Invoice settings checkbox "Include a link to a payment page in the invoice email" governs whether the hosted link is in that email at all — [Hosted invoice page](https://docs.stripe.com/invoicing/hosted-invoice-page).
4. **Apple Pay / Google Pay** need domain registration and HTTPS in both modes, and the local test page had neither — Stripe.js said so explicitly. Cards, Link and the rest rendered anyway.

## What it costs to build the in-chrome surface

Concretely, on top of what already exists:

1. **Stop discarding the URL and start returning the secret.** `api/internal/payments/invoice.go:313-314` calls `FinalizeInvoice` and then does `_ = hostedInvoiceURL`. Whichever surface wins, that value (or the Invoice id, which is enough to re-derive both on demand) has to be persisted on the `invoices` row. For Elements, prefer re-fetching `confirmation_secret` on read rather than storing it: it is a bearer credential for one payment, and the Invoice can be retrieved with the expand at any time after finalize.
2. **A Client-gated portal read.** A third handler beside the two in `api/internal/portal/mount.go`, behind `clientauth.PortalPopulation`, returning the balance plus, for a Stripe-rail Invoice, `{ stripeAccountId, clientSecret }` fetched live from Stripe. This is the one genuinely new piece of server work, and it is a handler, not a subsystem. It also inherits the read-gate question `#969` is answering on the Staff side — the Client's own view is not covered by that ruling.
3. **A Svelte payment route.** Load `stripe.js` from `js.stripe.com` (never bundled), `Stripe(PUBLIC_STRIPE_PUBLISHABLE_KEY, { stripeAccount })`, `elements({ clientSecret })`, a Payment Element, `confirmPayment({ elements, return_url })` pointing at a portal confirmation route, and a state on that route that reads `payment_intent_client_secret` from the query string. Appearance API to match tokens.css.
4. **Nothing on the webhook.** Proven above.
5. **Domain registration** for the app's domains if wallets are wanted, and an annual **SAQ A** attestation.

The redirect alternative costs item 1 and a link, and nothing else — but it hands the Client a Stripe-branded page with the Practice's name on it, no way back into Doula Cloud, a link that expires within 120 days, and a save-my-card prompt on a page we do not control.

## Reproduction

Nothing here needs a running Doula Cloud. With the Sandbox secret key and `Stripe-Account: acct_1UDCmj1rKosUjGsh`: create a Customer, create an Invoice with `collection_method=send_invoice` and `days_until_due=30`, add an InvoiceItem, then `POST /v1/invoices/{id}/finalize?expand[]=confirmation_secret&expand[]=payments`. The `client_secret` in the response is the whole finding.
