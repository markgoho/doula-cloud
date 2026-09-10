/**
 * The three ways back in when the phone holding a second factor is gone
 * (#615), as the screens #694 built for them call them.
 *
 * The paths live here rather than inline in each `+page.svelte` for the
 * same reason `mfaRequirement.ts`'s do: `formErrors.usage.spec.ts` greps
 * every quoted string in a `.svelte` file for GOV.UK's banned words, and
 * a `.ts` module is outside that glob, which is where an API identifier
 * that happens to collide with banned prose belongs anyway.
 *
 * Every function here throws on a non-2xx, carrying whatever the BFF
 * said and never a sentence invented here -- the spend endpoint in
 * particular answers a wrong code and an unknown address identically
 * (#168), and this module must not undo that by branching on which one it
 * thinks it saw. `refusalMessage` rather than `refusalError`: none of
 * these three endpoints sends `APIError.details`, so there is no field map
 * to carry, and `refusalMessage` is the one that reads a bodyless 5xx as
 * "there is a problem with the service" rather than as an empty sentence.
 */
import type { Fetcher } from './fetcher.js';

import { refusalMessage } from './formErrors.js';

function vouchPath(practiceId: string, staffId: string): string {
	return `/api/practices/${practiceId}/staff/${staffId}/mfa-recovery/vouch`;
}

const rotatePath = '/api/staff/mfa-recovery/saved-codes/rotate';

const spendPath = '/api/staff/mfa-recovery/spend';

/**
 * Mints a single-use, 24-hour code for one Staff member at this Practice
 * and mails it to the **vouching Owner's own address**, never the locked-
 * out person's -- she reads it out over a call she has already satisfied
 * herself about. Owner-only server-side.
 *
 * `idToken` is a *freshly* minted Identity Platform ID token, from a
 * step-up re-authentication run immediately before this call: the
 * endpoint's `RequireRecentAuth` refuses anything older than five
 * minutes, and its `RequireConfirmed` refuses a request with no
 * `X-Confirmed` header. The session cookie still authenticates the
 * request; the Bearer token is a second, additional proof of recency
 * (api/internal/staffauth/reauth.go).
 *
 * The `fetcher` a caller passes must be `apiFetch`, not
 * `apiFetchWithSession`: a stale step-up token is refused with 401, and
 * `apiFetchWithSession` reads any 401 as a dead session and signs her out
 * of a session that is in fact perfectly alive.
 */
export async function vouchForStaff(
	fetcher: Fetcher,
	practiceId: string,
	staffId: string,
	idToken: string
): Promise<void> {
	const response = await fetcher(vouchPath(practiceId, staffId), {
		method: 'POST',
		headers: { Authorization: `Bearer ${idToken}`, 'X-Confirmed': 'true' }
	});
	if (!response.ok) {
		throw new Error(await refusalMessage(response));
	}
}

/**
 * Revokes whatever saved recovery codes the caller holds and mints a
 * fresh set, returning every plaintext once.
 *
 * This is the only path by which a saved code's plaintext is ever seen:
 * the set minted at the Membership event that made her a sole Owner is
 * discarded unread, and so is the replacement minted when she spends one.
 * So "shown when they are minted" and "rotated" are one act on one
 * screen, not two -- the first reveal *is* a rotate, and the screen has
 * to say plainly that anything she already wrote down stops working.
 *
 * 403s anyone who is not currently the sole Owner of some Practice.
 */
export async function rotateSavedCodes(fetcher: Fetcher): Promise<string[]> {
	const response = await fetcher(rotatePath, { method: 'POST' });
	if (!response.ok) {
		throw new Error(await refusalMessage(response));
	}
	const body: { codes: string[] } = await response.json();
	return body.codes;
}

/**
 * Spends a recovery code -- an Owner-vouched one or a sole Owner's own
 * saved one, the endpoint works out which -- clearing the named account's
 * second-factor enrolment.
 *
 * It mints no session, deliberately: Identity Platform challenges for a
 * second factor on every sign-in while one exists, so a locked-out person
 * cannot sign in first and spend a code afterwards. She signs in again
 * once this succeeds, with her password alone.
 *
 * Unauthenticated, so a caller passes `apiFetch` rather than
 * `apiFetchWithSession`: there is no session to expire, and the person
 * calling it is by definition signed out.
 */
export async function spendRecoveryCode(
	fetcher: Fetcher,
	email: string,
	code: string
): Promise<void> {
	const response = await fetcher(spendPath, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email, code })
	});
	if (!response.ok) {
		throw new Error(await refusalMessage(response));
	}
}
