import { describe, it, expect } from 'vitest';
import { decideLanding, type Membership } from './landing';

const practiceA: Membership = { practiceId: 'a', practiceName: 'Practice A', roles: ['owner'] };
const practiceB: Membership = { practiceId: 'b', practiceName: 'Practice B', roles: ['doula'] };

describe('decideLanding', () => {
	it('redirects straight to the only Practice when there is exactly one', () => {
		expect(decideLanding({ memberships: [practiceA], lastPracticeId: undefined })).toEqual({
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
		expect(decideLanding({ memberships: [practiceA, practiceB], lastPracticeId: undefined })).toEqual({
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

	// #745: not a picker with nothing in it. She is either a Staff member
	// her last Practice removed or a signup that half-landed, and both
	// need a screen that names the state and offers a way on.
	it('reports no Practice at all as its own outcome, not an empty picker', () => {
		expect(decideLanding({ memberships: [], lastPracticeId: undefined })).toEqual({
			type: 'no-practice'
		});
	});
});
