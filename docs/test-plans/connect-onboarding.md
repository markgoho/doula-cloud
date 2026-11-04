# Onboarding a Practice's Stripe Connect account for a walk

Written from the walk that first completed it ([#236](https://github.com/markgoho/doula-cloud/issues/236)), and rewritten from the one that last did ([#2171](https://github.com/markgoho/doula-cloud/issues/2171), 2026-10-28). Every later walk that needs a Practice able to raise Invoices follows this rather than re-deriving it.

**Drive it in the walk Chrome profile, through Claude in Chrome** ([#2687](https://github.com/markgoho/doula-cloud/issues/2687)). That is a separate profile of the founder's own Chrome on Windows, kept for walks and signed in to nothing but the Claude in Chrome extension (no Google account, no Stripe), never the profile the founder works in. Stripe fingerprints an automation-launched browser, not the absence of a human, so the browser is the whole trick:

| Browser | Result |
| --- | --- |
| Playwright-launched Chromium, headless | CAPTCHA at the email step |
| Playwright-launched Chromium, headed | CAPTCHA at the password step |
| chrome-devtools MCP's headless Chrome in WSL | hCaptcha at the email step (#2171's first pass, 2026-10-27) |
| The founder's own Chrome on Windows, through Claude in Chrome | **no challenge on any screen** |

Claude in Chrome works from a session in WSL, and the Windows Chrome reaches the WSL stack on `localhost` (`http://localhost:5173`), so nothing else is needed to get there. It replaces `playwriter`, which this recipe used to name. Never solve a challenge; if Stripe serves one, stop and hand over.

The extension connects once per profile, so the walk profile shows in `list_connected_browsers` as a browser of its own. Pick it there with `select_browser` before the first page of the walk; if the session is already pointed at another browser, it is the wrong one. If the walk profile is not listed, the founder opens it and presses **Connect** in the extension.

**The founder has to be there.** The agent fills the screens that hold nothing sensitive. These are the founder's, and only the founder's, because each one is a credential or a legal agreement:

- the Stripe login: email, password, and the two-step code;
- the SSN's last four;
- Link's **Agree and continue**, on the payout screen;
- the final **Agree and submit**.

**Two-step authentication needs an authenticator app.** Stripe offers a text message, but the Persona's phone number is a fictional 555 number that receives nothing, and `000000` is refused at that step. Choose the authenticator app, and the founder scans it. Never write the password, the authenticator secret or a backup code into this repository, an issue or a comment.

**It signs that Chrome into Stripe as the Persona, which is why it is a separate profile.** A Practice's connected account has a full Stripe Dashboard, so the login step creates a real Stripe _user_ for the Owner's fake address, such as `maya.okonkwo.2565@sim.doula.cloud`, and the hosted flow signs the browser into it. That user can see only the Practice's account; **Switch sandbox** is grayed out, so there is no switching back. Signing back in as the founder needs the founder's password and two-step code, so no script can undo it.

In the walk profile that costs nothing. Run in the founder's everyday Chrome, as it was until 2026-11-02, it swapped out the founder's Stripe session: the Dashboard opened on Maya, and a `stripe login` approved from that Chrome offered only Maya's account, which left the Stripe CLI answering `403` "Permission denied" to every platform call. If that happens, wait until no walk is mid-onboarding, then the founder signs out of the Persona in the Dashboard and back in as themselves, and runs `stripe logout && stripe login` again.

**Approve `stripe login` only from the founder's everyday Chrome, signed in as the founder,** and pick **DoulaCloud sandbox** (`acct_1U7N3e1rKoVEA79v`) on its "Choose an account" screen. Never approve it from the walk profile.

## Before starting

- `bun run dev:full` in `app/`.
- `./scripts/stripe-listen.sh`, from the same worktree as `dev:full`. It forwards to that worktree's own BFF, `18080 + 100 × <.port-offset>` (no file is offset 0), and prints the address it chose; check that it is the port your `dev:full` listens on ([`docs/environment.md`](../environment.md#local-webhook-delivery)). **Check the secret it prints equals `STRIPE_WEBHOOK_SECRET` in `app/.env.local`**; all three webhook secrets there hold the CLI's one secret locally, and both of the script's listeners print it. Read the log for `<--` lines, not `-->`: a `-->` with no `<--` means the event arrived and was never delivered.
- **Check `app/.env.local` has `STRIPE_PUBLISHABLE_KEY=pk_test_…`** ([`docs/environment.md`](../environment.md)) before `dev:full` starts. Without it the BFF refuses the Client's pay control in the portal ("Your Practice cannot take this payment yet."), even on an active account, and the Client can pay only on Stripe's hosted Invoice page. Adding it later means restarting the BFF.
- The stack's Postgres is torn down with its volumes (`app/e2e/stack.ts`), so **a connected account does not survive the session that made it.** Do the onboarding after the day's last `dev:full` start, inside the session that needs it.

## The walk

Sign in as the Owner, open **Settings → Getting paid**. If it asks the website question first, answer it (**Your website** does not lead back; return to **Getting paid** yourself). Press **Connect Stripe**, or **Continue Stripe onboarding** if an account was already made.

Stripe's screens, in order, with the values that pass in the Sandbox. **(Founder)** marks a step only the founder does.

1. **Stripe login.** Email: the Owner's address. Password: any, 10 or more characters **(Founder)**. Two-step: choose the authenticator app, then the code **(Founder)**.
2. **Business type.** Unregistered business. EIN: No.
3. **Personal details.** The Owner's name, DOB `01/01/1990`, street `address_full_match`, the Practice's city, state and ZIP (Rochester, New York, 14604 for Maya), phone `(201) 555-0123`. SSN last 4 `0000` **(Founder)**.
4. **Business details.** The website is prefilled from the Practice; add a short product description. A website on `example.com` is refused as "Not a valid URL", so a Practice needs a plausible one.
5. **Payout account.** Choose the "Test (Non-OAuth)" institution. Link's **Agree and continue** **(Founder)**, then **Connect accounts**, **Finish without saving**, **Done**. Pick the Success account ••6789, Manual schedule, **Continue**.
6. **Public details.** Prefilled from the Practice; **Continue**.
7. **Radar.** The Lite link did not respond on 2026-10-28, so **Continue with Pro**, which is free in test mode.
8. **Climate.** **No thanks**.
9. **Stripe Tax.** **Not right now**.
10. **Review.** **Agree and submit** **(Founder)**.

## After

Stripe returns to **Getting paid** on `?connect=return`, which reads "Awaiting Stripe review" while Stripe verifies; on 2026-10-28 that took about 40 seconds. The screen re-reads status on a short poll and has **Check status again**. It reads "Taking payments" once the `capability_status_updated` webhook has written `card_payments` `active`, and `practices.stripe_connect_status_event_id` then holds that event's id. For a script, poll `GET /api/practices/{id}/payments/connect` until `cardPaymentsStatus` is `active`.

Then, on an Engagement with a signed Contract, **Create Invoice**, choose **By card, through Stripe.**, and press **Create Invoice**. The Client pays on Stripe's hosted Invoice page with `4242 4242 4242 4242`, any future expiry, any CVC. Stripe sends no mail from a Sandbox, so the walk reads the page's address from the Invoice's `hosted_invoice_url`; uncheck Link's "Save my information" on that page. `invoice.paid` marks the Invoice **Paid** on both sides.
