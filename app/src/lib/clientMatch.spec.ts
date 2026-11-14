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
		expect(matchHistory({ createdAt: ADDED, engagements: [] }, 'America/New_York')).toBe('Added Mar 4, 2024 · No work started yet');
	});

	it('says one Engagement is open now', () => {
		expect(matchHistory({ createdAt: ADDED, engagements: [engagement('active')] }, 'America/New_York')).toBe(
			'Added Mar 4, 2024 · Birth · open now'
		);
	});

	it('counts the open ones among several', () => {
		expect(
			matchHistory({
				createdAt: ADDED,
				engagements: [engagement('completed', '2025-06-01T12:00:00Z'), engagement('intake')]
			}, 'America/New_York')
		).toBe('Added Mar 4, 2024 · Birth, Birth · 1 open now');
	});

	it('says when one ended Engagement ended', () => {
		expect(
			matchHistory({ createdAt: ADDED, engagements: [engagement('completed', '2026-06-12T12:00:00Z')] }, 'America/New_York')
		).toBe('Added Mar 4, 2024 · Birth · ended Jun 12, 2026');
	});

	it('names the latest end among several, compared as instants rather than text', () => {
		expect(
			matchHistory({
				createdAt: ADDED,
				engagements: [
					// Noon UTC on the 15th, though its text sorts before the
					// next one, which is noon UTC on the 14th.
					engagement('completed', '2026-06-15T00:00:00-12:00'),
					engagement('completed', '2026-06-15T02:00:00+14:00'),
					engagement('completed')
				]
			}, 'America/New_York')
		).toBe('Added Mar 4, 2024 · Birth, Birth, Birth · most recent ended Jun 15, 2026');
	});

	it('says an ended Engagement ended without guessing a date the ledger does not hold', () => {
		expect(matchHistory({ createdAt: ADDED, engagements: [engagement('completed')] }, 'America/New_York')).toBe(
			'Added Mar 4, 2024 · Birth · ended'
		);
		expect(
			matchHistory({ createdAt: ADDED, engagements: [engagement('completed'), engagement('completed')] }, 'America/New_York')
		).toBe('Added Mar 4, 2024 · Birth, Birth · all ended');
	});
});
