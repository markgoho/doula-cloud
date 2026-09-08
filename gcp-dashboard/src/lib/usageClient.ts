import { readRoute } from './apiClient.ts';
import type { UsageSnapshot } from './server/usageQuery.ts';

/**
 * The route that pulls a fresh Cloud Monitoring read.
 */
export const USAGE_ENDPOINT = '/api/usage';

/**
 * Shown when the route failed but said nothing useful about why. It names
 * Monitoring because a sync reads two upstreams and a banner has to say which
 * of them went wrong.
 */
export const UNKNOWN_USAGE_FAILURE_MESSAGE = 'Cloud Monitoring usage could not be read.';

/**
 * Pulls one fresh set of usage figures, covering every service the panels
 * show.
 */
export async function loadUsage(fetcher: typeof fetch): Promise<UsageSnapshot> {
	return readRoute<UsageSnapshot>(fetcher, USAGE_ENDPOINT, UNKNOWN_USAGE_FAILURE_MESSAGE);
}
