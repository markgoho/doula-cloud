import { describe, expect, it, vi } from 'vitest';
import { COST_ENDPOINT, UNKNOWN_FAILURE_MESSAGE, loadCostBreakdown } from './costClient.ts';

function respond(body: unknown, isOk = true): Response {
	return { ok: isOk, json: async () => body } as Response;
}

describe('loadCostBreakdown', () => {
	it('asks the cost route for a fresh read every time', async () => {
		const fetcher = vi.fn().mockResolvedValue(respond({ total: 0, services: [] }));

		await loadCostBreakdown(fetcher);

		expect(fetcher).toHaveBeenCalledWith(COST_ENDPOINT, { cache: 'no-store' });
	});

	it('returns the breakdown the route sent', async () => {
		const breakdown = { total: 1, services: [] };
		const fetcher = vi.fn().mockResolvedValue(respond(breakdown));

		await expect(loadCostBreakdown(fetcher)).resolves.toEqual(breakdown);
	});

	it('throws with the route’s own explanation of a failure', async () => {
		const fetcher = vi.fn().mockResolvedValue(respond({ message: 'query timed out' }, false));

		await expect(loadCostBreakdown(fetcher)).rejects.toThrow('query timed out');
	});

	it('falls back to a plain message when the failure body says nothing useful', async () => {
		const fetcher = vi.fn().mockResolvedValue(respond({ message: 42 }, false));

		await expect(loadCostBreakdown(fetcher)).rejects.toThrow(UNKNOWN_FAILURE_MESSAGE);
	});

	it('falls back when the failure body is not JSON at all', async () => {
		const fetcher = vi.fn().mockResolvedValue({
			ok: false,
			json: async () => {
				throw new Error('not JSON');
			}
		} as unknown as Response);

		await expect(loadCostBreakdown(fetcher)).rejects.toThrow(UNKNOWN_FAILURE_MESSAGE);
	});
});
