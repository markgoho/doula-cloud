import { readRoute } from './apiClient.ts';
import type { CloudRunUsage } from './server/usageQuery.ts';

/**
 * The route that pulls a fresh Cloud Monitoring read.
 */
export const USAGE_ENDPOINT = '/api/usage';

/**
 * Shown when the route failed but said nothing useful about why. It names
 * Cloud Run because a sync reads two upstreams and a banner has to say which
 * of them went wrong.
 */
export const UNKNOWN_USAGE_FAILURE_MESSAGE = 'Cloud Run usage could not be read.';

/**
 * Pulls one fresh set of Cloud Run usage figures.
 */
export async function loadCloudRunUsage(fetcher: typeof fetch): Promise<CloudRunUsage> {
	return readRoute<CloudRunUsage>(fetcher, USAGE_ENDPOINT, UNKNOWN_USAGE_FAILURE_MESSAGE);
}
