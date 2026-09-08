import { describe, expect, it, vi } from 'vitest';
import type { DashboardData } from './dashboard.ts';
import { DashboardSync } from './dashboardSync.svelte.ts';

const data: DashboardData = {
	breakdown: { total: 12.49, services: [] },
	usage: {
		since: '2026-09-01T00:00:00.000Z',
		through: '2026-09-08T04:30:00.000Z',
		cloudRun: { requestCount: 4785 },
		cloudSql: { diskQuotaBytes: 10_464_022_528 },
		cloudStorage: { storedBytes: 567_149 },
		firestore: {},
		firebaseHosting: { monthlySentBytes: 9_467_734 }
	}
};
const syncedAt = Date.parse('2026-09-07T06:12:00Z');

describe('DashboardSync', () => {
	it('starts with nothing loaded and nothing to report', () => {
		const sync = new DashboardSync(async () => data);

		expect(sync.state).toBe('idle');
		expect(sync.breakdown).toBeUndefined();
		expect(sync.usage).toBeUndefined();
		expect(sync.syncedAt).toBeUndefined();
	});

	it('does not read anything until a person asks for a sync', () => {
		const load = vi.fn().mockResolvedValue(data);

		const sync = new DashboardSync(load);

		expect(load).not.toHaveBeenCalled();
		expect(sync.state).toBe('idle');
	});

	it('is loading while the read is in flight', async () => {
		const pending = Promise.withResolvers<DashboardData>();
		const sync = new DashboardSync(() => pending.promise);

		const inFlight = sync.sync();
		expect(sync.state).toBe('loading');

		pending.resolve(data);
		await inFlight;
		expect(sync.state).toBe('success');
	});

	it('keeps the cost, the usage that produced it, and the time they arrived', async () => {
		const sync = new DashboardSync(
			async () => data,
			() => syncedAt
		);

		await sync.sync();

		expect(sync.breakdown).toEqual(data.breakdown);
		expect(sync.usage).toEqual(data.usage);
		expect(sync.syncedAt).toBe(syncedAt);
		expect(sync.errorMessage).toBeUndefined();
	});

	it('reports why a failed read failed', async () => {
		const sync = new DashboardSync(async () => {
			throw new Error('query timed out');
		});

		await sync.sync();

		expect(sync.state).toBe('error');
		expect(sync.errorMessage).toBe('query timed out');
	});

	it('reports a thrown non-Error too', async () => {
		const sync = new DashboardSync(async () => {
			throw 'no credentials';
		});

		await sync.sync();

		expect(sync.errorMessage).toBe('no credentials');
	});

	it('clears an earlier failure when a retry succeeds', async () => {
		const load = vi
			.fn()
			.mockRejectedValueOnce(new Error('query timed out'))
			.mockResolvedValueOnce(data);
		const sync = new DashboardSync(load);

		await sync.sync();
		await sync.sync();

		expect(sync.state).toBe('success');
		expect(sync.errorMessage).toBeUndefined();
	});

	it('ignores a second sync while one is already in flight', async () => {
		const load = vi.fn().mockResolvedValue(data);
		const sync = new DashboardSync(load);

		const first = sync.sync();
		await sync.sync();
		await first;

		expect(load).toHaveBeenCalledTimes(1);
	});

	it('defaults to the wall clock for the sync time', async () => {
		const before = Date.now();
		const sync = new DashboardSync(async () => data);

		await sync.sync();

		expect(sync.syncedAt).toBeGreaterThanOrEqual(before);
	});
});
