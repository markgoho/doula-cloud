import type { CostBreakdown } from './costBreakdown.ts';

/**
 * Where one sync stands. Drives the whole sidebar.
 */
export type SyncState = 'idle' | 'loading' | 'success' | 'error';

/**
 * The state behind the sync action.
 *
 * Stateless by design: every sync is a fresh server-side read that replaces
 * what is on screen. Nothing here starts a timer, polls, or keeps history —
 * a sync happens because a person asked for one.
 */
export class CostSync {
	readonly #load: () => Promise<CostBreakdown>;
	/**
	 * Epoch milliseconds, not a `Date`: reactive state that is never mutated.
	 */
	readonly #now: () => number;

	state = $state<SyncState>('idle');
	breakdown = $state<CostBreakdown | undefined>();
	errorMessage = $state<string | undefined>();
	syncedAt = $state<number | undefined>();

	constructor(load: () => Promise<CostBreakdown>, now: () => number = Date.now) {
		this.#load = load;
		this.#now = now;
	}

	/**
	 * Pulls a fresh breakdown. A second call while one is in flight is
	 * ignored, so an impatient second click cannot leave two responses racing
	 * to write the same fields.
	 */
	async sync(): Promise<void> {
		if (this.state === 'loading') return;

		this.state = 'loading';
		this.errorMessage = undefined;

		try {
			const breakdown = await this.#load();
			this.breakdown = breakdown;
			this.syncedAt = this.#now();
			this.state = 'success';
		} catch (error) {
			this.errorMessage = error instanceof Error ? error.message : String(error);
			this.state = 'error';
		}
	}
}
