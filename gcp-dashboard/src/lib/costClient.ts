import { readRoute } from './apiClient.ts';
import type { CostBreakdown } from './costBreakdown.ts';

/**
 * The route that pulls a fresh billing-export read.
 */
export const COST_ENDPOINT = '/api/cost';

/**
 * Shown when the route failed but said nothing useful about why.
 */
export const UNKNOWN_FAILURE_MESSAGE = 'The billing export could not be read.';

/**
 * Pulls one fresh cost breakdown.
 */
export async function loadCostBreakdown(fetcher: typeof fetch): Promise<CostBreakdown> {
	return readRoute<CostBreakdown>(fetcher, COST_ENDPOINT, UNKNOWN_FAILURE_MESSAGE);
}
