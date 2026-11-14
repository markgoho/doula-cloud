import { describe, expect, it, vi } from 'vitest';
import {
	datedWaitingHref,
	datedWaitingLabel,
	isWaitingLineShown,
	loadWaitingVisits,
	undatedWaitingLabel,
	waitingEngagementLabel,
	type WaitingVisits
} from './waitingVisits.js';
import { jsonResponse } from './testResponse.js';

const waiting: WaitingVisits = { total: 3, datedCount: 2, undatedCount: 1, items: [], hasMore: false };
const schedule = '/practices/p1/schedule';
const now = new Date('2026-11-14T15:00:00Z');

describe('loadWaitingVisits', () => {
	it('reads the Practice-wide waiting Visits', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(waiting));

		expect(await loadWaitingVisits(fetcher, 'p1')).toEqual(waiting);
		expect(fetcher).toHaveBeenCalledWith('/api/practices/p1/visits/waiting');
	});

	it('throws the refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ message: 'No.' }, 403));

		await expect(loadWaitingVisits(fetcher, 'p1')).rejects.toThrow();
	});
});

describe('isWaitingLineShown', () => {
	it('is drawn where a Visit waits or the read failed, and not otherwise', () => {
		expect(isWaitingLineShown(waiting)).toBe(true);
		expect(isWaitingLineShown('unavailable')).toBe(true);
		expect(isWaitingLineShown({ ...waiting, total: 0 })).toBe(false);
		expect(isWaitingLineShown(undefined)).toBe(false);
	});
});

describe('the words', () => {
	it('count Visits, singular and plural', () => {
		expect(datedWaitingLabel(waiting)).toBe('2 Visits on the Schedule');
		expect(datedWaitingLabel({ ...waiting, datedCount: 1 })).toBe('1 Visit on the Schedule');
		expect(undatedWaitingLabel(waiting)).toBe('1 Visit with no date yet');
		expect(waitingEngagementLabel({ engagementId: 'e', clientName: 'Pat Lee', undatedCount: 1 })).toBe('Pat Lee');
		expect(waitingEngagementLabel({ engagementId: 'e', clientName: 'Pat Lee', undatedCount: 2 })).toBe('Pat Lee, 2 Visits');
	});
});

describe('datedWaitingHref', () => {
	it('is the default days when the last Visit is within them', () => {
		expect(datedWaitingHref(schedule, { ...waiting, latestScheduledAt: '2026-12-01T15:00:00Z' }, now, 'UTC')).toBe(
			`${schedule}?needsDoula=true`
		);
		expect(datedWaitingHref(schedule, waiting, now, 'UTC')).toBe(`${schedule}?needsDoula=true`);
	});

	it('names the days from today through the last Visit when it is further out', () => {
		expect(datedWaitingHref(schedule, { ...waiting, latestScheduledAt: '2027-03-01T15:00:00Z' }, now, 'UTC')).toBe(
			`${schedule}?from=2026-11-14&to=2027-03-01&needsDoula=true`
		);
	});

	it('counts the days in the Practice zone', () => {
		const late = { ...waiting, latestScheduledAt: '2027-03-01T02:00:00Z' };

		expect(datedWaitingHref(schedule, late, now, 'America/New_York')).toBe(
			`${schedule}?from=2026-11-14&to=2027-02-28&needsDoula=true`
		);
	});
});
