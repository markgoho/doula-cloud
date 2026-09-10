import { describe, expect, it } from 'vitest';

import { membershipChangeSentence } from './membershipHistory.js';
import type { MembershipChange } from './staff.js';

/*
 * The whole point of these assertions is the words: #872's AC says roles
 * and employment type are named "in the team's words, not the stored
 * values", so every expectation below is a display word the stored value
 * had to be mapped to (#262's `roles.ts`).
 */

function change(overrides: Partial<MembershipChange>): MembershipChange {
	return {
		eventId: 'event-1',
		action: 'joined',
		actorName: 'Renata Alvarez',
		createdAt: '2026-08-28T12:00:00Z',
		...overrides
	};
}

describe('membershipChangeSentence', () => {
	it('names what a person arrived as, both halves of it', () => {
		expect(
			membershipChangeSentence(
				change({ action: 'joined', roles: ['owner', 'admin', 'doula'], employmentType: 'employee' })
			)
		).toBe('Joined as Owner, Admin, Doula, Employee');
	});

	it('reads a joining event missing its employment type as the half it has', () => {
		expect(membershipChangeSentence(change({ action: 'joined', roles: ['doula'] }))).toBe(
			'Joined as Doula'
		);
	});

	it('names both sides of a role change', () => {
		expect(
			membershipChangeSentence(
				change({ action: 'roles_changed', previousRoles: ['doula'], roles: ['admin', 'doula'] })
			)
		).toBe('Roles changed from Doula to Admin, Doula');
	});

	it('names both sides of an employment-type change', () => {
		expect(
			membershipChangeSentence(
				change({
					action: 'employment_type_changed',
					previousEmploymentType: 'employee',
					employmentType: 'contractor'
				})
			)
		).toBe('Employment type changed from Employee to Contractor');
	});

	it('says plainly that a Membership ended', () => {
		expect(membershipChangeSentence(change({ action: 'removed', previousRoles: ['doula'] }))).toBe(
			'Removed from this practice'
		);
	});

	it('says what an ended-sessions entry actually did', () => {
		expect(membershipChangeSentence(change({ action: 'sessions_ended' }))).toBe(
			'Signed out of every device'
		);
	});

	/*
	 * An entry always carries the facts its own action moved -- the BFF
	 * omits a field only when it did not move -- so the three cases below
	 * are malformed rows rather than states the product produces. They
	 * are pinned anyway, and pinned as *rendering* rather than throwing,
	 * because a history is the last screen that should go down over a row
	 * it does not recognize: the same leniency `roleLabel` applies to a
	 * role it has no word for, applied one level up.
	 */
	describe('a malformed entry', () => {
		it('renders a joining event carrying no roles as the half it has', () => {
			expect(membershipChangeSentence(change({ action: 'joined', employmentType: 'employee' }))).toBe(
				'Joined as Employee'
			);
		});

		it('renders a role change carrying neither side', () => {
			expect(membershipChangeSentence(change({ action: 'roles_changed' }))).toBe(
				'Roles changed from  to '
			);
		});

		it('renders an employment-type change carrying neither side', () => {
			expect(membershipChangeSentence(change({ action: 'employment_type_changed' }))).toBe(
				'Employment type changed from  to '
			);
		});
	});

	it('falls back to the generic phrase for an action this build has no words for', () => {
		expect(membershipChangeSentence(change({ action: 'something_new_happened' }))).toBe(
			'Something new happened'
		);
	});
});
