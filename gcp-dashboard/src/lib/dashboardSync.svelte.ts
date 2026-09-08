import type { CostBreakdown } from './costBreakdown.ts';
import type { DashboardData } from './dashboard.ts';
import type { CloudRunUsage } from './server/usageQuery.ts';

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
 *
 * One sync covers both halves of the screen: the cost breakdown and the
 * Cloud Run usage that produced it. They arrive together or not at all.
 */
export class DashboardSync {
	readonly #load: () => Promise<DashboardData>;
	/**
	 * Epoch milliseconds, not a `Date`: reactive state that is never mutated.
	 */
	readonly #now: () => number;

	state = $state<SyncState>('idle');
	breakdown = $state<CostBreakdown | undefined>();
	usage = $state<CloudRunUsage | undefined>();
	errorMessage = $state<string | undefined>();
	syncedAt = $state<number | undefined>();

	constructor(load: () => Promise<DashboardData>, now: () => number = Date.now) {
		this.#load = load;
		this.#now = now;
	}

	/**
	 * Pulls a fresh breakdown and fresh usage. A second call while one is in
	 * flight is ignored, so an impatient second click cannot leave two
	 * responses racing to write the same fields.
	 */
	async sync(): Promise<void> {
		if (this.state === 'loading') return;

		this.state = 'loading';
		this.errorMessage = undefined;

		try {
			const { breakdown, usage } = await this.#load();
			this.breakdown = breakdown;
			this.usage = usage;
			this.syncedAt = this.#now();
			this.state = 'success';
		} catch (error) {
			this.errorMessage = error instanceof Error ? error.message : String(error);
			this.state = 'error';
		}
	}
}
