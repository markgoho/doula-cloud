import { describe, expect, it } from 'vitest';
import { asRoles, joinedSentence } from './joined.js';

describe('asRoles', () => {
	it.each([
		[['doula'], 'a Doula'],
		[['admin'], 'an Admin'],
		[['admin', 'doula'], 'an Admin and Doula'],
		[['owner', 'admin', 'doula'], 'an Owner, Admin and Doula']
	])('names %j as %s', (roles, words) => {
		expect(asRoles(roles)).toBe(words);
	});
});

describe('joinedSentence (#2239)', () => {
	it('says she joined, which Practice, and as what', () => {
		expect(
			joinedSentence({ practiceName: 'Rooted', roles: ['admin'], employmentType: 'employee', hasOtherPractices: false })
		).toBe("You've joined Rooted. You're an Admin, as an employee.");
	});

	it('says her other Practices are unchanged and the work state that carried over', () => {
		expect(
			joinedSentence({
				practiceName: 'Ridgeline Doulas',
				roles: ['doula'],
				employmentType: 'contractor',
				hasOtherPractices: true,
				workState: 'New York'
			})
		).toBe("You've joined Ridgeline Doulas. You're a Doula, as a contractor. Your other Practices are unchanged. You still work from New York.");
	});
});
