import { describe, expect, it, vi } from 'vitest';
import {
	doulaOptions,
	endSessions,
	loadDoulas,
	loadDoulasOrNone,
	loadStaff,
	loadWorkStateHistory,
	removeMember,
	revokeInvitation,
	updateMembership
} from './staff.js';
import { jsonResponse as response } from './testResponse.js';

describe('loadStaff', () => {
	it('fetches the practice staff path with no query on the first page', async () => {
		const roster = {
			members: [
				{
					staffId: 'staff-1',
					name: 'Anne-Marie Ochieng-Whitfield',
					email: 'anne-marie@example.test',
					roles: ['owner'],
					employmentType: 'employee',
					workState: 'NY',
					workStateReportedAt: '2026-01-01T00:00:00Z'
				}
			],
			invitations: { items: [], hasMore: false }
		};
		const fetcher = vi.fn().mockResolvedValue(response(roster));

		const result = await loadStaff(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/staff');
		expect(result).toEqual(roster);
	});

	it('carries a later page’s cursor on the query string', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(response({ members: [], invitations: { items: [], hasMore: false } }));

		await loadStaff(fetcher, 'practice-1', 'cursor-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/staff?cursor=cursor-1');
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('Server rejected the Staff list request', 500));

		await expect(loadStaff(fetcher, 'practice-1')).rejects.toThrow(
			'Server rejected the Staff list request'
		);
	});
});

describe('loadWorkStateHistory', () => {
	it('fetches the member’s history path with no query on the first page', async () => {
		const history = {
			memberSince: '2026-08-01T00:00:00Z',
			items: [{ eventId: 'event-1', workState: 'NY', createdAt: '2026-08-28T12:00:00Z' }],
			hasMore: false
		};
		const fetcher = vi.fn().mockResolvedValue(response(history));

		const result = await loadWorkStateHistory(fetcher, 'practice-1', 'staff-1');

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/staff/staff-1/work-state-history'
		);
		expect(result).toEqual(history);
	});

	it('carries a later page’s cursor on the query string', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(response({ memberSince: '2026-08-01T00:00:00Z', items: [], hasMore: false }));

		await loadWorkStateHistory(fetcher, 'practice-1', 'staff-1', 'cursor-1');

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/staff/staff-1/work-state-history?cursor=cursor-1'
		);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('Failed to load work state history', 500));

		await expect(loadWorkStateHistory(fetcher, 'practice-1', 'staff-1')).rejects.toThrow(
			'Failed to load work state history'
		);
	});
});

describe('updateMembership', () => {
	it('PATCHes the roles and employment type together', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({}));

		await updateMembership(fetcher, 'practice-1', 'staff-1', ['doula', 'admin'], 'contractor');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/staff/staff-1/membership', {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ roles: ['doula', 'admin'], employmentType: 'contractor' })
		});
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(response('a practice must keep at least one Owner', 409));

		await expect(
			updateMembership(fetcher, 'practice-1', 'staff-1', [], 'employee')
		).rejects.toThrow('a practice must keep at least one Owner');
	});
});

describe('removeMember', () => {
	it('DELETEs the membership, confirmed', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({}));

		await removeMember(fetcher, 'practice-1', 'staff-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/staff/staff-1/membership', {
			method: 'DELETE',
			headers: { 'X-Confirmed': 'true' }
		});
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(response('a practice must keep at least one Owner', 409));

		await expect(removeMember(fetcher, 'practice-1', 'staff-1')).rejects.toThrow(
			'a practice must keep at least one Owner'
		);
	});
});

describe('revokeInvitation', () => {
	it('POSTs the revoke path, confirmed', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({}));

		await revokeInvitation(fetcher, 'practice-1', 'invitation-1');

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/staff/invitations/invitation-1/revoke',
			{ method: 'POST', headers: { 'X-Confirmed': 'true' } }
		);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(response('no pending invitation found at this practice', 404));

		await expect(revokeInvitation(fetcher, 'practice-1', 'invitation-1')).rejects.toThrow(
			'no pending invitation found at this practice'
		);
	});
});

describe('endSessions', () => {
	it('DELETEs the member’s sessions, confirmed', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({}));

		await endSessions(fetcher, 'practice-1', 'staff-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/staff/staff-1/sessions', {
			method: 'DELETE',
			headers: { 'X-Confirmed': 'true' }
		});
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('Failed to end sessions', 500));

		await expect(endSessions(fetcher, 'practice-1', 'staff-1')).rejects.toThrow(
			'Failed to end sessions'
		);
	});
});


// One roster row, with only the three fields `loadDoulas` reads spelled
// out per case -- at the outer scope because eslint's
// unicorn/consistent-function-scoping asks for it there.
function member(staffId: string, name: string, roles: string[], employmentType: string) {
	return {
		staffId,
		name,
		email: `${staffId}@example.test`,
		roles,
		employmentType,
		workState: 'NY',
		workStateReportedAt: '2026-01-01T00:00:00Z'
	};
}

describe('loadDoulas', () => {
	const roster = {
		members: [
			member('staff-1', 'Maya Oyelaran-Fitzgerald', ['doula'], 'contractor'),
			member('staff-2', 'Ada Brennan', ['owner'], 'employee'),
			member('staff-3', 'Bess Nakamura-Oduya', ['admin', 'doula'], 'employee')
		],
		invitations: { items: [] }
	};

	it('keeps only the roster members holding the Doula role', async () => {
		const fetcher = vi.fn().mockResolvedValue(response(roster));

		expect(await loadDoulas(fetcher, 'practice-1')).toEqual([
			{ staffId: 'staff-1', name: 'Maya Oyelaran-Fitzgerald', employmentType: 'contractor' },
			{ staffId: 'staff-3', name: 'Bess Nakamura-Oduya', employmentType: 'employee' }
		]);
	});

	it('throws the refusal, the same as loadStaff', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('not permitted to read this', 403));

		await expect(loadDoulas(fetcher, 'practice-1')).rejects.toThrow('not permitted to read this');
	});

	// A Practice with no Doulas on its roster yet is an empty list, which
	// is a different answer from `undefined` below -- the picker renders
	// with nothing in it rather than not rendering at all.
	it('answers an empty list for a roster with no Doulas on it', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ members: [], invitations: { items: [] } }));

		expect(await loadDoulasOrNone(fetcher, 'practice-1')).toEqual([]);
	});

	// The refusal a plain Doula gets: "not for you", said in the type, so
	// the screen leaves the picker out rather than showing it broken.
	it('answers undefined when the roster read refuses', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('not permitted to read this', 403));

		expect(await loadDoulasOrNone(fetcher, 'practice-1')).toBeUndefined();
	});

	it('answers undefined when the session has expired', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('sign in again', 401));

		expect(await loadDoulasOrNone(fetcher, 'practice-1')).toBeUndefined();
	});

	// The line between "not yours" and "broken". A 500 answered as
	// `undefined` would take the Add-a-Visit form and both pickers off an
	// Owner's screen and tell her nothing, which reads as a permission she
	// has lost rather than as an outage.
	it('throws rather than hides when the roster read fails', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('the database is down', 500));

		await expect(loadDoulasOrNone(fetcher, 'practice-1')).rejects.toThrow('the database is down');
	});

	// A dropped connection rejects with a TypeError carrying no status at
	// all, which is the case a bare `catch {}` used to swallow.
	it('throws when the roster read never reaches the BFF', async () => {
		const fetcher = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));

		await expect(loadDoulasOrNone(fetcher, 'practice-1')).rejects.toThrow('Failed to fetch');
	});
});

describe('doulaOptions', () => {
	// Keyed on the staff id, never on the name: two Doulas at one agency
	// can share a name, and keying on the word would silently narrow to
	// the wrong person.
	it('pairs each staff id with the name shown for it', () => {
		expect(
			doulaOptions([
				{ staffId: 'staff-1', name: 'Maya Oyelaran-Fitzgerald', employmentType: 'contractor' },
				{ staffId: 'staff-9', name: 'Maya Oyelaran-Fitzgerald', employmentType: 'employee' }
			])
		).toEqual([
			{ value: 'staff-1', label: 'Maya Oyelaran-Fitzgerald' },
			{ value: 'staff-9', label: 'Maya Oyelaran-Fitzgerald' }
		]);
	});
});
