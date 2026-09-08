import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types.js';
import { summarizeCost } from '#lib/costBreakdown.js';
import { createBigQueryCostSource } from '#lib/server/bigQueryCostSource.js';

/**
Status for a failed read: this route depends on BigQuery upstream.
*/
const UPSTREAM_FAILURE = 502;

// The composition root: the only place that builds the real BigQuery client.
// Everything it calls is a tested seam in src/lib.
const costSource = createBigQueryCostSource();

/**
One fresh read of the billing export. Nothing is cached.
*/
export const GET: RequestHandler = async () => {
	try {
		return json(summarizeCost(await costSource()));
	} catch (error) {
		const message = error instanceof Error ? error.message : String(error);
		return json({ message }, { status: UPSTREAM_FAILURE });
	}
};
