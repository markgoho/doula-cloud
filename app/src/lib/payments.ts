/**
 * A Practice's Stripe Connect linkage (#79): an Owner starts hosted
 * onboarding via `connect`, and an Owner or an Admin reads the current
 * status via `loadConnectStatus` -- ADR-0008's Stripe Connect state row
 * (#267), the same pair that reads Invoice history. Both are read live
 * from Stripe -- see
 * api/internal/payments/connect.go's doc comments.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './api.js';

export type ConnectStatus =
	| 'not_connected'
	| 'onboarding_incomplete'
	| 'pending'
	| 'payouts_restricted'
	| 'active';

/** The status Stripe reports for one capability on a v2 Account's merchant
 * configuration. Accounts v1 reported booleans; v2 reports four values, and
 * `pending` is the one a boolean could not express -- Stripe is reviewing,
 * and there is nothing left for the Owner to do (#247). */
export type CapabilityStatus = 'active' | 'pending' | 'restricted' | 'unsupported';

export interface ConnectStatusResult {
	status: ConnectStatus;
	/**
	Whether the Practice can be paid by card at all.
	*/
	cardPaymentsStatus: CapabilityStatus;
	/** Whether that money can reach the Practice's bank. Moves
	 * independently of cardPaymentsStatus. */
	payoutsStatus: CapabilityStatus;
	/** Stripe field paths still awaiting the Owner. Always present -- the
	 * backend sends an empty list rather than omitting it. */
	requirementsDue: string[];
}

function connectPath(practiceId: string): string {
	return `/api/practices/${practiceId}/payments/connect`;
}

/** Loads a Practice's current Stripe Connect status. Throws with the
 * response body's message on a non-2xx response, mirroring loadBalance's
 * error-surfacing convention. */
export async function loadConnectStatus(fetcher: Fetcher, practiceId: string): Promise<ConnectStatusResult> {
	const response = await fetcher(connectPath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Whether a Connect status can still change on its own, with nobody
 * doing anything -- Stripe is reviewing (`pending`), or the account is
 * not yet payable and reports nothing outstanding (an empty
 * `requirementsDue` on a status other than `active`). Once this is
 * false, either the account is already `active` or the Owner has to
 * supply something before the status will ever move again -- reading
 * again on a timer would only repeat today's answer. */
export function canConnectStatusStillMove(result: ConnectStatusResult): boolean {
	return result.status !== 'active' && result.requirementsDue.length === 0;
}

/** What a failed re-read (poll or on-demand) tells the person. Exported
 * so the screen and its spec share one sentence rather than two that can
 * drift apart. */
export const CONNECT_STATUS_CHECK_FAILED_MESSAGE =
	'The status check did not go through. Try the button again, or reload the page.';

/** How many times, and after what growing delays, `pollConnectStatus`
 * re-reads Connect status before giving up. Growing rather than fixed:
 * Stripe's own account propagation is not instant, and a fixed short
 * delay would mostly land before Stripe does while a fixed long one
 * would make the common case wait longer than it has to. The ceiling --
 * five reads, a little under a minute in total -- keeps this a small,
 * bounded number of extra calls to a live-from-Stripe endpoint, never an
 * open-ended timer. */
export const CONNECT_STATUS_POLL_DELAYS_MS = [2000, 4000, 8000, 16_000, 30_000];

export interface ConnectStatusPollHandle {
	/** Stops any scheduled re-read. Idempotent, and safe to call after the
	 * poll has already stopped on its own. */
	stop: () => void;
}

interface ConnectStatusPollCallbacks {
	/** Called with each successful re-read, replacing the caller's status
	 * wholesale -- the badge, the explanation, the requirement count and
	 * the banner are one derived view of a single value, so there is
	 * never a moment where two of them disagree. */
	onResult: (result: ConnectStatusResult) => void;
	/** Called when a re-read fails. The poll stops after this -- see
	 * `pollConnectStatus`'s doc comment -- so the caller's own status is
	 * left exactly as it was; this is only the cue to tell the person the
	 * check did not succeed. */
	onError: () => void;
	/** Called exactly once, whenever the poll stops scheduling further
	 * reads of its own accord -- the status settled, the delay schedule
	 * ran out, or a read failed. Never called for an explicit `stop()`,
	 * which is the caller ending it, not the poll ending itself. */
	onStopped: () => void;
}

/**
 * Re-reads Connect status on a growing delay while it can still move on
 * its own, for the one moment that actually races Stripe: landing back on
 * `?connect=return` before Stripe's own Account object has caught up with
 * the form the Owner just submitted. Stops the moment the status settles
 * (`canConnectStatusStillMove` turns false), the delay schedule runs out,
 * or a read fails -- whichever comes first. A failed read is not retried
 * automatically; the person's own on-demand check is what recovers from
 * it, so this hands the failure to `onError` and stops rather than
 * silently trying again on the same schedule.
 *
 * `initial` is the status already on screen -- the poll never re-reads
 * before the schedule's first delay, so there is no reason to fetch it
 * again just to decide whether to start.
 */
export function pollConnectStatus(
	fetcher: Fetcher,
	practiceId: string,
	initial: ConnectStatusResult,
	callbacks: ConnectStatusPollCallbacks,
	delaysMs: readonly number[] = CONNECT_STATUS_POLL_DELAYS_MS
): ConnectStatusPollHandle {
	let isStopped = false;
	let timer: ReturnType<typeof setTimeout> | undefined;

	function scheduleNext(attempt: number, previous: ConnectStatusResult) {
		if (attempt >= delaysMs.length || !canConnectStatusStillMove(previous)) {
			isStopped = true;
			callbacks.onStopped();
			return;
		}
		timer = setTimeout(async () => {
			let next: ConnectStatusResult | undefined;
			let hasErrored = false;
			try {
				next = await loadConnectStatus(fetcher, practiceId);
			} catch {
				hasErrored = true;
			}
			// An explicit stop() can land while this fetch is in flight --
			// the screen was left between the request going out and it
			// coming back. Whatever it returned is stale by definition, so
			// neither callback below runs and nothing schedules again.
			if (isStopped) return;
			if (hasErrored) {
				isStopped = true;
				callbacks.onError();
				callbacks.onStopped();
				return;
			}
			callbacks.onResult(next!);
			scheduleNext(attempt + 1, next!);
		}, delaysMs[attempt]);
	}

	scheduleNext(0, initial);

	return {
		stop() {
			if (isStopped) return;
			isStopped = true;
			clearTimeout(timer);
		}
	};
}

/** Starts (or resumes) Stripe Connect onboarding and returns the
 * Stripe-hosted Account Link URL the caller's browser must navigate to.
 *
 * Throws with whatever sentence the server sent on a non-2xx response --
 * a non-Owner attempting the request, which `RequireOwner` rejects, or a
 * Practice who has not declared a website, which #442 refuses with
 * docs/api-design.md section 7's structured body. Read through
 * `apiErrorMessage` rather than as raw text, so the second of those
 * reaches the screen as a sentence and not as JSON. */
export async function connect(fetcher: Fetcher, practiceId: string): Promise<string> {
	const response = await fetcher(connectPath(practiceId), { method: 'POST' });
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const body: { onboardingUrl: string } = await response.json();
	return body.onboardingUrl;
}

/** What the screen offers before the ask is made (#917, ADR-0035). Here
 * beside the two sentences below rather than inline on the screen: all
 * three state the same weekly bound, and a bound restated in three
 * places has to be findable from one. */
export const CONNECT_NUDGE_OFFER_MESSAGE =
	'Doula Cloud can email every Practice Owner about this. It sends this reminder at most once a week.';

/** What the screen says once the nudge is queued (#917, ADR-0035).
 * Exported so the screen and its spec share one sentence rather than two
 * that can drift apart, the way `CONNECT_STATUS_CHECK_FAILED_MESSAGE`
 * already is. It says the mail is on its way rather than that it
 * arrived, because ADR-0010's outbox is exactly the difference between
 * those two claims. */
export const CONNECT_NUDGE_SENT_MESSAGE =
	'Every Practice Owner is being emailed about connecting Stripe. Doula Cloud sends this at most once a week.';

/** What the screen tells a non-Owner reader in the unconnected statuses
 * that are *not* `not_connected` (#917, ADR-0035). There is no control in
 * those, because #343's payout notification mails every Owner once per
 * episode on its own -- so the honest thing is to say that the mail is
 * already handled, rather than offering a second send she cannot know is
 * a duplicate.
 *
 * Present tense, not "has already been emailed": #343 waits out a
 * 48-hour grace window before it sends, and skips the mail entirely if
 * the Owner finishes inside it. A past-tense claim would be false for
 * the first two days of every episode. This sentence is true at every
 * instant, which is the same standard ADR-0037 held its own derived
 * facts to. */
export const CONNECT_OWNERS_ALREADY_EMAILED_MESSAGE =
	'Doula Cloud emails every Practice Owner when Stripe asks for something, so there is nothing to send from here.';

/** Asks Doula Cloud to email every Practice Owner that Stripe still has
 * to be connected (#917, ADR-0035).
 *
 * Offered only to a reader who may read Connect status and may not act
 * on it, and only while the status is `not_connected` -- in every other
 * unconnected status #343's payout notification has already mailed the
 * Owners on its own. The server refuses a Practice that has already
 * connected and a second ask inside its cooldown; both refusals are
 * sentences a person reads, so they come back through `apiErrorMessage`
 * like every other refusal on this screen. */
export async function nudgeOwnersToConnect(fetcher: Fetcher, practiceId: string): Promise<void> {
	const response = await fetcher(`${connectPath(practiceId)}/nudge`, { method: 'POST' });
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
}
