import { describe, expect, it, vi } from 'vitest';
import type { RequestEvent } from './$types.ts';

const costSource = vi.hoisted(() => vi.fn());
vi.mock('#lib/server/bigQueryCostSource.js', () => ({
	createBigQueryCostSource: () => costSource
}));

const { GET } = await import('./+server.ts');

// GET ignores the event entirely -- see +server.ts -- so a bare cast stands
// in for the RequestEvent this route's own handler never reads.
const event = {} as RequestEvent;

describe('GET /api/cost', () => {
	it('returns the summarized breakdown when the upstream read succeeds', async () => {
		costSource.mockResolvedValue([
			{ service: 'Cloud Run', sku: 'CPU Allocation Time', cost: 6.5, latestUsage: undefined }
		]);

		const response = await GET(event);
		const body = await response.json();

		expect(response.status).toBe(200);
		expect(body).toEqual({
			total: 6.5,
			services: [
				{
					service: 'Cloud Run',
					cost: 6.5,
					share: 1,
					skus: [{ sku: 'CPU Allocation Time', cost: 6.5 }],
					usageDetailAvailable: true
				}
			],
			costsThrough: undefined
		});
	});

	it('maps a thrown upstream error to the documented failure status', async () => {
		costSource.mockRejectedValue(new Error('query timed out'));

		const response = await GET(event);
		const body = await response.json();

		expect(response.status).toBe(502);
		expect(body).toEqual({ message: 'query timed out' });
	});
});
