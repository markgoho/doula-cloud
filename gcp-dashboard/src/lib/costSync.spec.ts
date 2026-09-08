import { describe, expect, it, vi } from 'vitest';
import type { CostBreakdown } from './costBreakdown.ts';
import { CostSync } from './costSync.svelte.ts';

const breakdown: CostBreakdown = { total: 12.49, services: [] };
const syncedAt = Date.parse('2026-09-07T06:12:00Z');

describe('CostSync', () => {
	it('starts with nothing loaded and nothing to report', () => {
		const sync = new CostSync(async () => breakdown);

		expect(sync.state).toBe('idle');
		expect(sync.breakdown).toBeUndefined();
		expect(sync.syncedAt).toBeUndefined();
	});

	it('does not read anything until a person asks for a sync', () => {
		const load = vi.fn().mockResolvedValue(breakdown);

		const sync = new CostSync(load);

		expect(load).not.toHaveBeenCalled();
		expect(sync.state).toBe('idle');
	});

	it('is loading while the read is in flight', async () => {
		const pending = Promise.withResolvers<CostBreakdown>();
		const sync = new CostSync(() => pending.promise);

		const inFlight = sync.sync();
		expect(sync.state).toBe('loading');

		pending.resolve(breakdown);
		await inFlight;
		expect(sync.state).toBe('success');
	});

	it('keeps the breakdown and the time it arrived', async () => {
		const sync = new CostSync(
			async () => breakdown,
			() => syncedAt
		);

		await sync.sync();

		expect(sync.breakdown).toEqual(breakdown);
		expect(sync.syncedAt).toBe(syncedAt);
		expect(sync.errorMessage).toBeUndefined();
	});

	it('reports why a failed read failed', async () => {
		const sync = new CostSync(async () => {
			throw new Error('query timed out');
		});

		await sync.sync();

		expect(sync.state).toBe('error');
		expect(sync.errorMessage).toBe('query timed out');
	});

	it('reports a thrown non-Error too', async () => {
		const sync = new CostSync(async () => {
			throw 'no credentials';
		});

		await sync.sync();

		expect(sync.errorMessage).toBe('no credentials');
	});

	it('clears an earlier failure when a retry succeeds', async () => {
		const load = vi
			.fn()
			.mockRejectedValueOnce(new Error('query timed out'))
			.mockResolvedValueOnce(breakdown);
		const sync = new CostSync(load);

		await sync.sync();
		await sync.sync();

		expect(sync.state).toBe('success');
		expect(sync.errorMessage).toBeUndefined();
	});

	it('ignores a second sync while one is already in flight', async () => {
		const load = vi.fn().mockResolvedValue(breakdown);
		const sync = new CostSync(load);

		const first = sync.sync();
		await sync.sync();
		await first;

		expect(load).toHaveBeenCalledTimes(1);
	});

	it('defaults to the wall clock for the sync time', async () => {
		const before = Date.now();
		const sync = new CostSync(async () => breakdown);

		await sync.sync();

		expect(sync.syncedAt).toBeGreaterThanOrEqual(before);
	});
});
