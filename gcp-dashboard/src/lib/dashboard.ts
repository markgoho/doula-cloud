import type { CostBreakdown } from './costBreakdown.ts';
import { loadCostBreakdown } from './costClient.ts';
import type { UsageSnapshot } from './server/usageQuery.ts';
import { loadUsage } from './usageClient.ts';

/**
 * Everything one sync puts on the screen: what was spent, and the usage that
 * produced it.
 */
export interface DashboardData {
	readonly breakdown: CostBreakdown;
	readonly usage: UsageSnapshot;
}

/**
 * Pulls cost and usage together, as one sync.
 *
 * Both reads go out at once, and either one failing fails the sync: a screen
 * whose whole point is a usage figure beside the cost it produced is worth
 * little with half a pair on it, and each read's error names its own upstream.
 */
export async function loadDashboard(fetcher: typeof fetch): Promise<DashboardData> {
	const [breakdown, usage] = await Promise.all([
		loadCostBreakdown(fetcher),
		loadUsage(fetcher)
	]);

	return { breakdown, usage };
}
