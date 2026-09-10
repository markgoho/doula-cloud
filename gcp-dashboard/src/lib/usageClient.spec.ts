import { describe, expect, it, vi } from 'vitest';
import type { UsageSnapshot } from './server/usageQuery.ts';
import { loadUsage, UNKNOWN_USAGE_FAILURE_MESSAGE, USAGE_ENDPOINT } from './usageClient.ts';

const usage: UsageSnapshot = {
	since: '2026-09-01T00:00:00.000Z',
	through: '2026-09-08T04:30:24.000Z',
	cloudRun: { requestCount: 4785 },
	cloudSql: { diskQuotaBytes: 10_464_022_528 },
	cloudStorage: { storedBytes: 567_149 },
	firestore: {},
	firebaseHosting: { sentBytes: 9_467_734 }
};

function respond(body: unknown, isOk = true): Response {
	return { ok: isOk, json: async () => body } as Response;
}

describe('loadUsage', () => {
	it('asks the usage route for a fresh read every time', async () => {
		const fetcher = vi.fn().mockResolvedValue(respond(usage));

		await loadUsage(fetcher);

		expect(fetcher).toHaveBeenCalledWith(USAGE_ENDPOINT, { cache: 'no-store' });
	});

	it('returns the usage the route sent, for every service on the screen', async () => {
		const fetcher = vi.fn().mockResolvedValue(respond(usage));

		await expect(loadUsage(fetcher)).resolves.toEqual(usage);
	});

	it('names Monitoring in a failure the route could not explain, so the banner says which half broke', async () => {
		const fetcher = vi.fn().mockResolvedValue(respond({}, false));

		await expect(loadUsage(fetcher)).rejects.toThrow(UNKNOWN_USAGE_FAILURE_MESSAGE);
	});
});
