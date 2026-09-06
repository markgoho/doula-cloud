import type { Fetcher } from './pushRegistration.js';

/**
 * What a person sees when sign-out did not go through. Deliberately says
 * they are *still signed in*: a sign-out that quietly failed would leave
 * someone walking away from a shared laptop believing the session is
 * closed (#152).
 */
export const SIGN_OUT_FAILED_MESSAGE = 'Sign-out failed. You are still signed in on this device.';

/**
 * The result of a sign-out attempt -- success, or a message to show the
 * person. Not a thrown error: a failed sign-out is an ordinary outcome
 * the screen renders, not an exception.
 */
export type SignOutOutcome = { ok: true } | { ok: false; message: string };

/**
 * How long the push unregister gets before sign-out goes ahead without
 * it. Short on purpose: someone standing at a shared laptop or a borrowed
 * phone is waiting on this, and an unregister that has not landed by now
 * is not going to.
 */
export const UNREGISTER_TIMEOUT_MS = 3000;

export interface SignOutRequest {
	/**
	 * Where to take this device off push -- the push-subscriptions
	 * endpoint scoped to whatever the current screen belongs to: a
	 * Practice for Staff (#152), an Engagement for a Client (#153). Built
	 * by the caller, the same way the register call sites build their own
	 * subscribe URL, so this module stays out of the routing table.
	 * Undefined where a screen carries no such scope, which skips the
	 * unregister rather than blocking sign-out.
	 */
	unsubscribeURL: string | undefined;
	/**
	 * Sends the request with the browser's session cookie.
	 */
	fetcher: Fetcher;
	/**
	 * pushRegistration.ts's unregisterPushSubscription, injected so the
	 * ordering below is provable without the Service Worker APIs.
	 */
	unregisterPush: (unsubscribeURL: string, fetcher: Fetcher) => Promise<void>;
}

/**
 * Ends this browser's session, after taking this device off push.
 *
 * The order is mandatory, not tidiness: the unregister endpoint
 * authenticates by the same session cookie (staffauth.Middleware for
 * Staff, clientauth.Middleware for the Client portal), which ending the
 * session clears -- run it second and it 401s, leaving whoever picks the
 * device up later reading the last person's pushes on the lock screen.
 * The unregister is still best-effort: it must never keep someone signed
 * in.
 *
 * Only this browser's session ends. The same person's sessions on other
 * devices are separate rows and no refresh token is revoked, so their own
 * phone stays signed in; cutting every device off is a distinct action --
 * a Staff Owner's administrative revoke, or a Client's own
 * sign-out-everywhere (#618) -- built separately from this one.
 */
export async function signOutOfSession({
	unsubscribeURL,
	fetcher,
	unregisterPush
}: SignOutRequest): Promise<SignOutOutcome> {
	if (unsubscribeURL !== undefined) {
		// Bounded, not merely guarded: a request that hangs -- the "no
		// network, BFF down" case -- never rejects, so no catch rescues it,
		// and this step runs before the end-session request. Left unbounded
		// it would strand someone on a live session behind a disabled button
		// with nothing to read.
		await bestEffort(() => unregisterPush(unsubscribeURL, fetcher), UNREGISTER_TIMEOUT_MS);
	}

	try {
		const response = await fetcher('/api/session', { method: 'DELETE' });
		if (!response.ok) return { ok: false, message: SIGN_OUT_FAILED_MESSAGE };
	} catch {
		return { ok: false, message: SIGN_OUT_FAILED_MESSAGE };
	}
	return { ok: true };
}

/**
 * Runs work, giving it milliseconds at most and swallowing whatever it
 * throws: the caller carries on either way. Work that outlives its
 * deadline is abandoned, not cancelled -- there is nothing to cancel,
 * which is the point of it being best-effort.
 *
 * Exported so #618's sign-out-everywhere can bound its own push
 * unregister the same way this file's own signOutOfSession does --
 * ending every session on every device is still, from this browser's
 * side, this device going off push too.
 */
export async function bestEffort(work: () => Promise<void>, milliseconds: number): Promise<void> {
	const { promise: expired, resolve: expire } = Promise.withResolvers<void>();
	const timer = setTimeout(expire, milliseconds);
	try {
		await Promise.race([work(), expired]);
	} catch {
		// Swallowed -- see the doc comment above.
	} finally {
		clearTimeout(timer);
	}
}
