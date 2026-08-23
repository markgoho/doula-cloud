# Onboarding a Practice's Stripe Connect account for a walk

Written from the walk that first completed it
([#236](https://github.com/markgoho/doula-cloud/issues/236)). Every later walk that
needs a Practice able to raise Invoices follows this rather than re-deriving it.

**The browser matters.** Stripe served a CAPTCHA to a Playwright-launched Chromium
at the email step and did not challenge the user's own Chrome. Drive this through
a real browser — `playwriter` against the user's Chrome worked. That is why the
step is `blocked` for an unattended run and `manual` for an attended one.

## Before starting

- `bun run dev:full` in `app/`.
- `./scripts/stripe-listen.sh`. **Check the secret it prints equals
  `STRIPE_WEBHOOK_SECRET` in `app/.env.local`** — all three secrets there hold the
  one session secret locally. Read the log for `<--` lines, not `-->`: a `-->`
  with no `<--` means the event arrived and was never delivered.
- The stack's Postgres is torn down with its volumes (`app/e2e/stack.ts:171`), so
  **a connected account does not survive the session that made it.** Do the
  onboarding inside the session that needs it.

## The walk

Sign in as the Owner, open **Payments**, click **Connect Stripe** (or **Continue
Stripe onboarding** if `POST .../payments/connect` already made an account).

### Stripe user account — the outer page, not an iframe

| Step | Selector | Value |
| --- | --- | --- |
| Email | `#email` | the Owner's address |
| Password | `#password` | any, 10+ characters |
| Two-step auth | **Enter code manually instead** reveals a base32 TOTP secret | generate codes from it; fill `getByRole('textbox', { name: 'Verification code' })`, which auto-submits |
| Backup code | `[data-testid="backup-code-submit-button"]` | — |

A six-digit TOTP from a base32 secret is ~10 lines of `node:crypto`
(`createHmac('sha1', key)` over the 30-second counter, dynamic truncation).

### The account form — inside `iframe[name*="account-onboarding"]`

Reach it with `page.frameLocator('iframe').first()`. Two traps:

- **`snapshot({ frame })` times out on this iframe.** Read
  `fr.locator('body').innerText()` and enumerate controls with
  `fr.locator('input,select,textarea,button').evaluateAll(...)`.
- **The Industry dropdown renders in a different frame** —
  `connect-js.stripe.com/accessory_layer_*`, not the onboarding iframe. So does
  the bank-account modal. Find it with
  `page.frames().find((f) => f.url().includes('accessory_layer'))`. Its search box
  placeholder is `Search…` with a Unicode ellipsis, not three dots.

| Screen | What worked |
| --- | --- |
| Business type | `input[value="unregistered_business"]`.check() — **clicking the label fails**, the radio intercepts pointer events. Then EIN `input[type=radio][value="no"]` |
| Personal details | `#first_name` and `#last_name` are aria-label only, so `getByLabel` misses them; use the ids. The rest answer to `getByLabel`: Month, Day, Year, Street address, City, Postal code, Phone number, State. SSN is `#ssn_last_4` |
| Business details | `#business_profile[url]`, `#business_profile[product_description]`, and the Industry combobox — an `<a role=button>`, not a `<select>` |
| Bank details | **Enter test bank account credentials instead** opens a modal in the accessory frame with a **Use test account** button that fills and saves it. Its **Save** stays `aria-disabled` if you type the numbers in yourself |
| Public details | Prefilled from the Practice name — the statement descriptor comes out as `ROOTEDBIRT`, which is the `display_name` fix from `7261a59` |
| Radar / Climate / Tax | **Continue with Pro**, **No thanks**, **Not right now** |
| Review | **Agree and submit** |

Test values: street `address_full_match`, DOB `01/01/1990`, SSN last 4 `0000`,
phone `2015550123`, routing `110000000`, account `000123456789`, card
`4242 4242 4242 4242`. A website on `example.com` is rejected as "Not a valid
URL" — Stripe blocks the reserved domain, so use a plausible one.

## After

Poll `GET /api/practices/{id}/payments/connect` until `cardPaymentsStatus` is
`active`. **Do not read the Payments screen to decide** — it fetches once in
`onMount` and never again (**MO-G11**), so it shows the state at page load and its
"Status updates once Stripe confirms" banner never comes true without a reload.
