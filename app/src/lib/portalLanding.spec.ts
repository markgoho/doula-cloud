import { describe, it, expect } from 'vitest';
import { decidePortalLanding, type Engagement } from './portalLanding';

const engagementA: Engagement = {
	engagementId: 'a',
	practiceName: 'Practice A',
	status: 'intake',
	createdAt: '2026-01-15T00:00:00Z'
};
const engagementB: Engagement = {
	engagementId: 'b',
	practiceName: 'Practice B',
	status: 'active',
	createdAt: '2026-03-12T00:00:00Z'
};

describe('decidePortalLanding', () => {
	it('redirects straight to the only Engagement when there is exactly one', () => {
		expect(decidePortalLanding({ engagements: [engagementA] })).toEqual({
			type: 'redirect',
			engagementId: 'a'
		});
	});

	it('falls back to a picker when there is more than one Engagement', () => {
		expect(decidePortalLanding({ engagements: [engagementA, engagementB] })).toEqual({
			type: 'picker',
			engagements: [engagementA, engagementB]
		});
	});

	it('falls back to a picker when there are no Engagements at all', () => {
		expect(decidePortalLanding({ engagements: [] })).toEqual({
			type: 'picker',
			engagements: []
		});
	});
});
