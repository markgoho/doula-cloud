import { describe, expect, it, vi } from 'vitest';
import type { CloudRunUsage } from './server/usageQuery.ts';
import {
	loadCloudRunUsage,
	UNKNOWN_USAGE_FAILURE_MESSAGE,
	USAGE_ENDPOINT
} from './usageClient.ts';

const usage: CloudRunUsage = {
	since: '2026-09-01T00:00:00.000Z',
	through: '2026-09-08T04:30:24.000Z',
	metrics: { requestCount: 4785 }
};

function respond(body: unknown, isOk = true): Response {
	return { ok: isOk, json: async () => body } as Response;
}

describe('loadCloudRunUsage', () => {
	it('asks the usage route for a fresh read every time', async () => {
		const fetcher = vi.fn().mockResolvedValue(respond(usage));

		await loadCloudRunUsage(fetcher);

		expect(fetcher).toHaveBeenCalledWith(USAGE_ENDPOINT, { cache: 'no-store' });
	});

	it('returns the usage the route sent', async () => {
		const fetcher = vi.fn().mockResolvedValue(respond(usage));

		await expect(loadCloudRunUsage(fetcher)).resolves.toEqual(usage);
	});

	it('names Cloud Run in a failure the route could not explain, so the banner says which half broke', async () => {
		const fetcher = vi.fn().mockResolvedValue(respond({}, false));

		await expect(loadCloudRunUsage(fetcher)).rejects.toThrow(UNKNOWN_USAGE_FAILURE_MESSAGE);
	});
});
