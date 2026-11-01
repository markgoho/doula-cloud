import { describe, expect, it, vi } from 'vitest';
import { findStaff, LOOKUP_EMAIL_FIELD, lookupErrors } from './staffLookup.ts';

describe('lookupErrors', () => {
	it('asks for the email, then for one in the right form', () => {
		expect(lookupErrors(' ')).toEqual([
			{ message: 'Enter the email address the person signs in with', targetId: LOOKUP_EMAIL_FIELD }
		]);
		expect(lookupErrors('priya')).toEqual([
			{ message: 'Enter an email address in the correct format, like name@example.com', targetId: LOOKUP_EMAIL_FIELD }
		]);
		expect(lookupErrors('priya@example.com')).toEqual([]);
	});
});

describe('findStaff', () => {
	it('asks doula-ops for the Staff members with that sign-in email', async () => {
		const staff = [{ staffId: 's-1', name: 'Priya Raman', email: 'priya@example.com', practiceNames: ['Lakeside'] }];
		const fetcher = vi.fn(async () => Response.json({ staff }));

		const outcome = await findStaff(' priya+care@example.com ', fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/staff?email=priya%2Bcare%40example.com', { credentials: 'include' });
		expect(outcome).toEqual({ kind: 'ok', value: staff });
	});

	it('puts a refusal beside the email', async () => {
		const fetcher = vi.fn(async () =>
			Response.json({ code: 'INVALID_ARGUMENT', message: 'Correct the answer marked below.', details: { email: 'Enter an email address' } }, { status: 422 })
		);

		expect(await findStaff('x', fetcher)).toEqual({
			kind: 'refused',
			errors: [{ message: 'Enter an email address', targetId: LOOKUP_EMAIL_FIELD }]
		});
	});
});
