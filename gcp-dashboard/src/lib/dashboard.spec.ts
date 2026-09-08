import { describe, expect, it, vi } from 'vitest';
import { COST_ENDPOINT } from './costClient.ts';
import { loadDashboard } from './dashboard.ts';
import { USAGE_ENDPOINT } from './usageClient.ts';

const breakdown = { total: 12.49, services: [] };
const usage = {
	since: '2026-09-01T00:00:00.000Z',
	through: '2026-09-08T04:30:24.000Z',
	metrics: { requestCount: 4785 }
};

function route(endpoint: string): Response {
	const body = endpoint === COST_ENDPOINT ? breakdown : usage;
	return { ok: true, json: async () => body } as Response;
}

describe('loadDashboard', () => {
	it('pulls the cost and the usage that produced it in the one sync', async () => {
		const fetcher = vi.fn().mockImplementation(async (endpoint: string) => route(endpoint));

		await expect(loadDashboard(fetcher)).resolves.toEqual({ breakdown, usage });
		expect(fetcher.mock.calls.map(([endpoint]) => endpoint)).toEqual([
			COST_ENDPOINT,
			USAGE_ENDPOINT
		]);
	});

	it('fails the whole sync when either half fails, naming the half that did', async () => {
		const fetcher = vi.fn().mockImplementation(async (endpoint: string) => {
			if (endpoint === USAGE_ENDPOINT) {
				return { ok: false, json: async () => ({ message: 'monitoring quota exceeded' }) } as Response;
			}
			return route(endpoint);
		});

		await expect(loadDashboard(fetcher)).rejects.toThrow('monitoring quota exceeded');
	});
});
