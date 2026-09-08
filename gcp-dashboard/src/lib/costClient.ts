import type { CostBreakdown } from './costBreakdown.ts';

/**
 * The route that pulls a fresh billing-export read.
 */
export const COST_ENDPOINT = '/api/cost';

/**
 * Shown when the route failed but said nothing useful about why.
 */
export const UNKNOWN_FAILURE_MESSAGE = 'The billing export could not be read.';

interface FailureBody {
	readonly message?: unknown;
}

async function readFailureMessage(response: Response): Promise<string> {
	try {
		const body = (await response.json()) as FailureBody;
		if (typeof body.message === 'string') return body.message;
	} catch {
		// A failure that is not even JSON explains itself no better than the
		// fallback does.
	}

	return UNKNOWN_FAILURE_MESSAGE;
}

/**
 * Pulls one fresh cost breakdown.
 *
 * `fetch` is a parameter rather than the global so a spec can drive every
 * branch without a network. Nothing is cached: each call is a new read, which
 * is the whole sync model for this tool.
 */
export async function loadCostBreakdown(fetcher: typeof fetch): Promise<CostBreakdown> {
	const response = await fetcher(COST_ENDPOINT, { cache: 'no-store' });

	if (!response.ok) throw new Error(await readFailureMessage(response));

	return (await response.json()) as CostBreakdown;
}
