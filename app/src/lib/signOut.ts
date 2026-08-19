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
 * it. Short on purpose: someone standing at a shared laptop is waiting on
 * this, and an unregister that has not landed by now is not going to.
 */
export const UNREGISTER_TIMEOUT_MS = 3000;

export interface SignOutRequest {
	/**
	 * The Practice the current screen is scoped to, which the push
	 * unregister endpoint is keyed by. Every authenticated Staff screen
	 * sits under practices/[practiceId], so this is normally set; where a
	 * screen has no such scope the unregister is skipped rather than
	 * blocking sign-out.
	 */
	practiceId: string | undefined;
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
 * authenticates by the same session cookie (staffauth.Middleware), which
 * ending the session clears -- run it second and it 401s, leaving a
 * colleague who picks the laptop up later seeing this doula's Clients on
 * the lock screen. The unregister is still best-effort: it must never
 * keep someone signed in.
 *
 * Only this browser's session ends. The same person's sessions on other
 * devices are separate rows and no refresh token is revoked, so her own
 * phone stays signed in; cutting every device off is a separate
 * administrative action.
 */
export async function signOutOfSession({
	practiceId,
	fetcher,
	unregisterPush
}: SignOutRequest): Promise<SignOutOutcome> {
	if (practiceId !== undefined) {
		// Bounded, not merely guarded: a request that hangs -- the "no
		// network, BFF down" case -- never rejects, so no catch rescues it,
		// and this step runs before the end-session request. Left unbounded
		// it would strand someone on a live session behind a disabled button
		// with nothing to read.
		await bestEffort(
			() => unregisterPush(`/api/practices/${practiceId}/push-subscriptions`, fetcher),
			UNREGISTER_TIMEOUT_MS
		);
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
 */
async function bestEffort(work: () => Promise<void>, milliseconds: number): Promise<void> {
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
