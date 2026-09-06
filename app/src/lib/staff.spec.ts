import { describe, expect, it, vi } from 'vitest';
import {
	endSessions,
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
