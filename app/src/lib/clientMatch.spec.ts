import { describe, expect, it } from 'vitest';
import type { EngagementSummary } from './clientDetail.js';
import { matchHistory } from './clientMatch.js';

const ADDED = '2024-03-04T15:00:00Z';

function engagement(status: string, endedAt?: string): EngagementSummary {
	return {
		engagementId: `e-${status}-${endedAt ?? 'none'}`,
		kind: 'birth',
		status,
		createdAt: '2025-01-10T12:00:00Z',
		...(endedAt && { endedAt })
	};
}

describe('matchHistory', () => {
	it('says when a Client with no Engagement was added, and that there is none yet', () => {
		expect(matchHistory({ createdAt: ADDED, engagements: [] })).toBe('Added Mar 4, 2024 · No Engagements yet');
	});

	it('says one Engagement is open now', () => {
		expect(matchHistory({ createdAt: ADDED, engagements: [engagement('active')] })).toBe(
			'Added Mar 4, 2024 · 1 Engagement · open now'
		);
	});

	it('counts the open ones among several', () => {
		expect(
			matchHistory({
				createdAt: ADDED,
				engagements: [engagement('completed', '2025-06-01T12:00:00Z'), engagement('intake')]
			})
		).toBe('Added Mar 4, 2024 · 2 Engagements · 1 open now');
	});

	it('says when one ended Engagement ended', () => {
		expect(
			matchHistory({ createdAt: ADDED, engagements: [engagement('completed', '2026-06-12T12:00:00Z')] })
		).toBe('Added Mar 4, 2024 · 1 Engagement · ended Jun 12, 2026');
	});

	it('names the latest end among several, compared as instants rather than text', () => {
		expect(
			matchHistory({
				createdAt: ADDED,
				engagements: [
					// 03:00 UTC on the 15th, though its text sorts first.
					engagement('completed', '2026-06-14T22:00:00-05:00'),
					engagement('completed', '2026-06-14T23:00:00Z'),
					engagement('completed')
				]
			})
		).toBe('Added Mar 4, 2024 · 3 Engagements · most recent ended Jun 15, 2026');
	});

	it('says an ended Engagement ended without guessing a date the ledger does not hold', () => {
		expect(matchHistory({ createdAt: ADDED, engagements: [engagement('completed')] })).toBe(
			'Added Mar 4, 2024 · 1 Engagement · ended'
		);
		expect(
			matchHistory({ createdAt: ADDED, engagements: [engagement('completed'), engagement('completed')] })
		).toBe('Added Mar 4, 2024 · 2 Engagements · all ended');
	});
});
