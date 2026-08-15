import { describe, it, expect } from 'vitest';
import { decideLanding, type Membership } from './landing';

const practiceA: Membership = { practiceId: 'a', practiceName: 'Practice A', roles: ['owner'] };
const practiceB: Membership = { practiceId: 'b', practiceName: 'Practice B', roles: ['doula'] };

describe('decideLanding', () => {
	it('redirects straight to the only Practice when there is exactly one', () => {
		expect(decideLanding({ memberships: [practiceA], lastPracticeId: null })).toEqual({
			type: 'redirect',
			practiceId: 'a'
		});
	});

	it('redirects to the last-used Practice when it is still a current membership', () => {
		expect(decideLanding({ memberships: [practiceA, practiceB], lastPracticeId: 'b' })).toEqual({
			type: 'redirect',
			practiceId: 'b'
		});
	});

	it('falls back to a picker when there is no last-used Practice recorded', () => {
		expect(decideLanding({ memberships: [practiceA, practiceB], lastPracticeId: null })).toEqual({
			type: 'picker',
			memberships: [practiceA, practiceB]
		});
	});

	it('falls back to a picker when the last-used Practice is no longer a membership', () => {
		expect(decideLanding({ memberships: [practiceA, practiceB], lastPracticeId: 'stale' })).toEqual({
			type: 'picker',
			memberships: [practiceA, practiceB]
		});
	});

	it('falls back to a picker when there are no memberships at all', () => {
		expect(decideLanding({ memberships: [], lastPracticeId: null })).toEqual({
			type: 'picker',
			memberships: []
		});
	});
});
