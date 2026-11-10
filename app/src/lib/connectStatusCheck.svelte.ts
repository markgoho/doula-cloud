/**
 * The Getting paid screen's check of a Practice's Stripe Connect status
 * (#259, #2565), as one value: how the person arrived, where the check
 * is, and the latest status any read reported.
 *
 *     arrived (plain | return | refresh)
 *       -> reading   the first read is in flight
 *       -> polling   back from Stripe, and Stripe is catching up
 *       -> settled   no read of its own is scheduled
 *     reading -> failed   the first read did not go through
 *
 * The on-demand "Check status again" runs beside any of these. Before
 * this the screen held the same facts as a `status`, an `isPolling`, a
 * non-reactive poll handle and a destroyed flag, with two writers of the
 * status: the poll, and the on-demand check, which never told the poll
 * anything. A check that answered `active` mid-poll then drew "Taking
 * payments" under "Stripe is checking your details" until the poll's
 * next read, up to 30 seconds later, and a slow check could land after a
 * newer poll read and put the older answer back.
 *
 * So there is one writer of `status` (`#land`), every read takes a
 * ticket as it goes out and only a read newer than the one on screen may
 * land, and whichever read reports the final status (`active`) ends the
 * poll. The banner and the badge are both derived from here, so they
 * cannot disagree.
 */

import {
	canConnectStatusStillMove,
	loadConnectStatus,
	pollConnectStatus,
	CONNECT_STATUS_CHECK_FAILED_MESSAGE,
	CONNECT_STATUS_POLL_DELAYS_MS,
	type ConnectStatusPollHandle,
	type ConnectStatusResult
} from './payments.js';
import type { Fetcher } from './fetcher.js';
import { readFailure } from './pageRead.svelte.js';

/**
How the person reached the screen: directly, back from Stripe's form
(`?connect=return`), or on an Account Link that expired
(`?connect=refresh`).
*/
export type ConnectArrival = 'plain' | 'return' | 'refresh';

export function connectArrival(url: { readonly searchParams: Pick<URLSearchParams, 'get'> }): ConnectArrival {
	const parameter = url.searchParams.get('connect');
	if (parameter === 'return' || parameter === 'refresh') return parameter;
	return 'plain';
}

/**
Where the check is. `failed` is the first read only: a re-read that
fails keeps the last good status and says so in `refreshError`.
*/
export type ConnectCheckPhase =
	| { phase: 'reading' }
	| { phase: 'polling' }
	| { phase: 'settled' }
	| { phase: 'failed'; message: string };

/**
The one banner the screen shows about the trip to Stripe, if any.
*/
export type ConnectBanner = 'still-restricted' | 'checking' | 'finished' | 'link-expired' | undefined;

/**
What a failed first read says when what was thrown has no words.
*/
export const CONNECT_STATUS_LOAD_FAILED_MESSAGE = 'Failed to load Stripe Connect status';

export class ConnectStatusCheck {
	readonly #fetcher: Fetcher;
	readonly #practiceId: string;
	readonly #delaysMs: readonly number[];
	/* The ticket the last read to go out took, and the one whose answer is
	   on screen. A read lands only while its ticket is newer than that. */
	#issued = 0;
	#landed = 0;
	#pollTicket = 0;
	#manualTicket = 0;
	#poll: ConnectStatusPollHandle | undefined;

	arrival = $state<ConnectArrival>('plain');
	phase = $state<ConnectCheckPhase>({ phase: 'reading' });
	/**
	The latest status a read reported; `undefined` until the first lands.
	*/
	status = $state<ConnectStatusResult | undefined>();
	/**
	Why the last re-read did not go through, or the empty string.
	*/
	refreshError = $state('');
	/**
	Whether the on-demand check is in flight.
	*/
	isRechecking = $state(false);

	constructor(fetcher: Fetcher, practiceId: string, delaysMs: readonly number[] = CONNECT_STATUS_POLL_DELAYS_MS) {
		this.#fetcher = fetcher;
		this.#practiceId = practiceId;
		this.#delaysMs = delaysMs;
	}

	#ticket(): number {
		this.#issued += 1;
		return this.#issued;
	}

	#isCurrent(ticket: number): boolean {
		return ticket > this.#landed;
	}

	/**
	The one writer of `status`. Reports whether the answer landed.
	*/
	#land(ticket: number, result: ConnectStatusResult): boolean {
		if (!this.#isCurrent(ticket)) return false;
		this.#landed = ticket;
		this.status = result;
		if (this.phase.phase === 'polling' && !canConnectStatusStillMove(result)) this.#settle();
		return true;
	}

	#settle(): void {
		this.#poll?.stop();
		this.#poll = undefined;
		if (this.phase.phase === 'polling') this.phase = { phase: 'settled' };
	}

	#read(): Promise<ConnectStatusResult> {
		return loadConnectStatus(this.#fetcher, this.#practiceId);
	}

	/**
	 * For the first minutes after the return, a read that still lists
	 * requirements is Stripe catching up on the form just submitted, so
	 * nothing is shown as outstanding until the poll ends (#2565).
	 */
	get isStripeCatchingUp(): boolean {
		return this.phase.phase === 'polling' && this.status?.status === 'onboarding_incomplete';
	}

	/**
	 * Coming back from Stripe is not the same as being finished: the
	 * banner reads the status rather than assuming the trip worked (#421),
	 * and promises to keep checking only while a poll is running (#259).
	 */
	get banner(): ConnectBanner {
		if (this.arrival === 'refresh') return 'link-expired';
		if (this.arrival === 'plain') return undefined;
		if (this.phase.phase === 'polling') return 'checking';
		return this.status?.status === 'onboarding_incomplete' ? 'still-restricted' : 'finished';
	}

	/**
	 * The first read. On the return from Stripe, a status that can still
	 * move puts the check in `polling` as it lands, so the screen never
	 * draws a first read as settled while Stripe catches up; the poll
	 * itself waits for `startPolling`. Resolves `true` only when this
	 * read's answer is the one on screen.
	 */
	async first(arrival: ConnectArrival): Promise<boolean> {
		this.arrival = arrival;
		const ticket = this.#ticket();
		let result: ConnectStatusResult;
		try {
			result = await this.#read();
		} catch (error) {
			if (this.#isCurrent(ticket)) {
				this.phase = { phase: 'failed', message: readFailure(error, CONNECT_STATUS_LOAD_FAILED_MESSAGE) };
			}
			return false;
		}
		if (!this.#isCurrent(ticket)) return false;
		this.phase = { phase: arrival === 'return' && canConnectStatusStillMove(result) ? 'polling' : 'settled' };
		return this.#land(ticket, result);
	}

	/**
	 * Starts the poll the first read asked for, and nothing otherwise:
	 * arriving without `connect=return`, or with a status already final,
	 * never reads again on its own.
	 */
	startPolling(): void {
		if (this.phase.phase !== 'polling' || this.#poll !== undefined) return;
		this.#poll = pollConnectStatus(
			this.#fetcher,
			this.#practiceId,
			this.status!,
			{
				onRead: () => {
					this.#pollTicket = this.#ticket();
				},
				onResult: (result) => {
					if (this.#land(this.#pollTicket, result)) this.refreshError = '';
				},
				onError: () => {
					if (this.#isCurrent(this.#pollTicket)) this.refreshError = CONNECT_STATUS_CHECK_FAILED_MESSAGE;
				},
				onStopped: () => {
					this.#poll = undefined;
					this.#settle();
				}
			},
			this.#delaysMs
		);
	}

	/**
	 * The on-demand half of #259: reads again now, whatever the poll is
	 * doing. An answer of `active` ends the poll there and then. A failure
	 * keeps the last good status and says so in `refreshError`.
	 */
	async recheck(): Promise<void> {
		this.refreshError = '';
		this.isRechecking = true;
		const ticket = this.#ticket();
		this.#manualTicket = ticket;
		try {
			this.#land(ticket, await this.#read());
		} catch {
			if (this.#isCurrent(ticket)) this.refreshError = CONNECT_STATUS_CHECK_FAILED_MESSAGE;
		} finally {
			if (ticket === this.#manualTicket) this.isRechecking = false;
		}
	}

	/**
	 * The screen is gone: the poll stops, and no read in flight or started
	 * later lands. Safe to call at any point, including before the first
	 * read has answered.
	 */
	stop(): void {
		this.#landed = Infinity;
		this.#settle();
	}
}
