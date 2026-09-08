import type { CostBreakdown } from './costBreakdown.ts';
import { loadCostBreakdown } from './costClient.ts';
import type { CloudRunUsage } from './server/usageQuery.ts';
import { loadCloudRunUsage } from './usageClient.ts';

/**
 * Everything one sync puts on the screen: what was spent, and the usage that
 * produced it.
 */
export interface DashboardData {
	readonly breakdown: CostBreakdown;
	readonly usage: CloudRunUsage;
}

/**
 * Pulls cost and Cloud Run usage together, as one sync.
 *
 * Both reads go out at once, and either one failing fails the sync: a screen
 * whose whole point is a usage figure beside the cost it produced is worth
 * little with half a pair on it, and each read's error names its own upstream.
 */
export async function loadDashboard(fetcher: typeof fetch): Promise<DashboardData> {
	const [breakdown, usage] = await Promise.all([
		loadCostBreakdown(fetcher),
		loadCloudRunUsage(fetcher)
	]);

	return { breakdown, usage };
}
