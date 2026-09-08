import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types.js';
import { createMonitoringUsageSource } from '#lib/server/monitoringUsageSource.js';

/**
Status for a failed read: this route depends on Cloud Monitoring upstream.
*/
const UPSTREAM_FAILURE = 502;

// The composition root: the only place that builds the real Monitoring
// client. Everything it calls is a tested seam in src/lib.
const usageSource = createMonitoringUsageSource();

/**
One fresh read of Cloud Run usage for the billing period. Nothing is cached.
*/
export const GET: RequestHandler = async () => {
	try {
		return json(await usageSource());
	} catch (error) {
		const message = error instanceof Error ? error.message : String(error);
		return json({ message }, { status: UPSTREAM_FAILURE });
	}
};
