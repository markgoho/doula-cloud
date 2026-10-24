import type { APIResponse } from '@playwright/test';
import { resetRateLimits } from './stack';

/**
 * Sends a request, and -- if the BFF's rate limiter refused it -- clears
 * every counter the stack is holding and sends it exactly once more
 * (#1138).
 *
 * This is the one place that argument is written out; stack.ts's
 * resetRateLimits and the three call sites cite it rather than restate it.
 *
 * The suite arrives at the BFF from one address, so every rule keyed on
 * the caller's IP counts the whole run as one very busy person. Counted on
 * 2026-10-24 (#1858), one pass of the parallel project at four workers
 * made 59 signups, 71 sign-ins and 13 Staff invitation acceptances from
 * 127.0.0.1, and the clock project a few more: past `bootstrapRules`'
 * 50 signups per address per hour, and three quarters of `loginRules`'
 * 100 sign-ins (both in `api/internal/staffauth`). A browser-driven
 * signup cannot come through here, so stack.ts sets
 * E2E_ADDRESS_RATE_LIMIT_SCALE to 10 (api/internal/stackswitch): the
 * stack's per-address caps are 500 and 1000, and only those -- a Bearer
 * token, a session or an email is still held to its own cap.
 *
 * A *repeated* batch can still spend that: #827's three specs seed six
 * founding Owners and about nine sign-ins per repeat, `--repeat-each=30`
 * across four specs asks for roughly 180 signups and 270 sign-ins, and
 * nine repeats of the whole suite ask for more than 500 signups. The
 * windows are an hour long, which no run outlasts, so the repeat that
 * crosses the cap fails for a reason that has nothing to do with the
 * flake the batch went looking for -- and "run it many times over" is how
 * every flake on this tracker has been confirmed fixed.
 *
 * All the counters, not the refused endpoint's: clearing one endpoint at a
 * time only moves the wall to the next ceiling the batch reaches, which is
 * what a repeated run of those three specs actually did.
 *
 * Reactive, and deliberately so. Nothing is pre-cleared and no budget is
 * raised past the stack's scale, so the limiter is a live participant in
 * every request made through here. Only a refusal that has actually happened triggers a clear.
 *
 * One retry, not a loop: a second 429 means something other than this
 * suite's own volume is doing the refusing, and the caller's own
 * assertion should say so as loudly as it always has.
 *
 * See resetRateLimits (stack.ts) for why clearing the counters is not a
 * switch that could exist against a deployed BFF, and
 * `TestSignupRefusesAGenuineBurst` (api/internal/staffauth) for the
 * refusal this must never quietly turn off.
 */
export async function retryPastRateLimit(send: () => Promise<APIResponse>): Promise<APIResponse> {
	const first = await send();
	if (first.status() !== 429) {
		return first;
	}

	// The refused response is never handed back, so free what Playwright is
	// holding for it rather than leaving a body nobody will read.
	await first.dispose();
	resetRateLimits();
	return send();
}
