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
 * the caller's IP counts the whole run as one very busy person. A single
 * pass is nowhere near any ceiling. A *repeated* batch is: #827's three
 * specs seed six founding Owners and about nine sign-ins per repeat, so
 * ten repeats asks for 60 signups against `bootstrapRules`' budget of 50
 * and 90 sign-ins against `loginRules`' budget of 100 (both in `api/
 * internal`, and both per address per hour). The windows are an hour long,
 * which no run outlasts, so the ninth repeat onward fails for a reason
 * that has nothing to do with the flake the batch went looking for -- and
 * "run it many times over" is how every flake on this tracker has been
 * confirmed fixed.
 *
 * All the counters, not the refused endpoint's: clearing one endpoint at a
 * time only moves the wall to the next ceiling the batch reaches, which is
 * what a repeated run of those three specs actually did.
 *
 * Reactive, and deliberately so. Nothing is pre-cleared and no budget is
 * raised, so a single-pass run and CI behave exactly as they did before
 * and the limiter is a live participant in every request made through
 * here. Only a refusal that has actually happened triggers a clear.
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
