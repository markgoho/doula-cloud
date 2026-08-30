import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1' } }
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

function textResponse(body: string): Response {
	return jsonResponse(body, 403);
}

const members = [
	{
		staffId: 'staff-1',
		name: 'Ada Lovelace',
		email: 'ada@example.com',
		roles: ['owner'],
		employmentType: 'employee'
	},
	{
		staffId: 'staff-2',
		name: 'Grace Hopper',
		email: 'grace@example.com',
		roles: [],
		employmentType: 'contractor'
	}
];

const invitations = [
	{
		invitationId: 'invitation-1',
		address: 'lena@example.com',
		roles: ['doula'],
		employmentType: 'contractor',
		expiresAt: '2026-09-01T00:00:00Z',
		expired: false,
		deliveryFailed: false
	},
	{
		invitationId: 'invitation-2',
		address: 'undeliverable@example.com',
		roles: ['admin'],
		employmentType: 'employee',
		expiresAt: '2026-09-02T00:00:00Z',
		expired: true,
		deliveryFailed: true
	}
];

// One page of #459's work state history, keyed by staff id. memberSince
// dates the Membership, which is what lets the screen mark an assertion
// made before she joined -- a contractor doula carries her earlier
// Practice's rows in with her.
const workStateHistories: Record<string, unknown> = {
	'staff-1': {
		memberSince: '2026-08-01T00:00:00Z',
		items: [
			{
				eventId: 'event-2',
				previousWorkState: 'NY',
				workState: 'NJ',
				createdAt: '2027-03-14T09:30:00Z'
			},
			{ eventId: 'event-1', workState: 'NY', createdAt: '2026-08-28T12:00:00Z' }
		],
		hasMore: false
	},
	'staff-2': {
		// Asserted at another Practice, a year before this Membership.
		memberSince: '2026-08-01T00:00:00Z',
		items: [{ eventId: 'event-3', workState: 'CA', createdAt: '2025-05-04T12:00:00Z' }],
		hasMore: false
	}
};

interface MockOptions {
	roster?: { members: typeof members; invitations: typeof invitations };
	listOk?: boolean;
	sessionsResponse?: Response;
	membershipResponse?: Response;
	revokeResponse?: Response;
	removeResponse?: Response;
	historyResponse?: Response;
}

// The API double is stateful: a successful PATCH or revoke changes what
// the next roster read returns, so a test can assert on the screen the
// Owner ends up looking at rather than on the request that got her there.
function mockApi({
	roster = { members, invitations },
	listOk = true,
	sessionsResponse,
	membershipResponse,
	revokeResponse,
	removeResponse,
	historyResponse
}: MockOptions = {}) {
	const state = structuredClone(roster);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (path.includes('/work-state-history')) {
			if (historyResponse) {
				return Promise.resolve(historyResponse);
			}
			const staffId = path.split('/').at(-2);
			return Promise.resolve(jsonResponse(workStateHistories[String(staffId)]));
		}
		if (path.endsWith('/sessions') && init?.method === 'DELETE') {
			return Promise.resolve(sessionsResponse ?? jsonResponse({}));
		}
		if (path.endsWith('/membership') && init?.method === 'DELETE') {
			if (removeResponse) {
				return Promise.resolve(removeResponse);
			}
			const staffId = path.split('/').at(-2);
			state.members = state.members.filter((member) => member.staffId !== staffId);
			return Promise.resolve(jsonResponse({}));
		}
		if (path.endsWith('/membership')) {
			if (membershipResponse) {
				return Promise.resolve(membershipResponse);
			}
			const staffId = path.split('/').at(-2);
			const patch = JSON.parse(String(init?.body));
			state.members = state.members.map((member) =>
				member.staffId === staffId
					? { ...member, roles: patch.roles, employmentType: patch.employmentType }
					: member
			);
			return Promise.resolve(jsonResponse({}));
		}
		if (path.endsWith('/revoke')) {
			if (revokeResponse) {
				return Promise.resolve(revokeResponse);
			}
			const invitationId = path.split('/').at(-2);
			state.invitations = state.invitations.filter(
				(invitation) => invitation.invitationId !== invitationId
			);
			return Promise.resolve(jsonResponse({}));
		}
		return Promise.resolve(
			listOk
				? jsonResponse({
						members: state.members,
						invitations: { items: state.invitations, hasMore: false }
					})
				: textResponse('Server rejected the Staff list request')
		);
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function setup(options: MockOptions = {}) {
	mockApi(options);
	await render(Page, {});
}

describe('staff screen', () => {
	it('lists members with their roles and employment type', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Members' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'ada@example.com' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'owner' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'no roles yet' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'contractor' }).first()).toBeVisible();
	});

	// #261: a pending invitation must be tellable apart from a member who
	// holds no roles -- which is why they are two groups, not one list.
	it('lists pending invitations as their own group, apart from the members', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: 'Pending invitations' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'lena@example.com' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'doula' })).toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Revoke' }).first())
			.toBeVisible();
	});

	// #339's dead-lettered send has no other read surface.
	it('flags an invitation whose email could not be delivered', async () => {
		await setup();

		const flags = testPage.getByText('Email could not be delivered');
		await expect.element(flags).toBeVisible();
		expect(flags.elements()).toHaveLength(1);
	});

	it('shows both empty messages when nobody is here and nobody is invited', async () => {
		await setup({ roster: { members: [], invitations: [] } });

		await expect.element(testPage.getByText('No Staff yet.')).toBeVisible();
		await expect.element(testPage.getByText('No pending invitations.')).toBeVisible();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
	});

	it('shows an error notice when the roster fails to load', async () => {
		await setup({ listOk: false });

		await expect.element(testPage.getByText('Server rejected the Staff list request')).toBeVisible();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
	});

	// RA-G2 (#261): roles and employment type on one form, one change.
	it('edits a membership roles and employment type together', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Edit membership' }).first().click();
		await expect.element(testPage.getByRole('group', { name: 'Roles' })).toBeVisible();
		await testPage.getByRole('checkbox', { name: 'Doula' }).click();
		await testPage.getByRole('radio', { name: 'Contractor' }).click();
		await testPage.getByRole('button', { name: 'Save membership' }).click();

		// The form closes and Ada's row now reads the edited Membership --
		// both halves, from one save.
		await expect.element(testPage.getByRole('group', { name: 'Roles' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('cell', { name: 'owner, doula' })).toBeVisible();
		const contractorCells = testPage.getByRole('cell', { name: 'contractor' });
		await expect.element(contractorCells.first()).toBeVisible();
		expect(contractorCells.elements()).toHaveLength(3);
	});

	it('shows an error notice and keeps the form open when saving a membership fails', async () => {
		await setup({ membershipResponse: textResponse('a practice must keep at least one Owner') });

		await testPage.getByRole('button', { name: 'Edit membership' }).first().click();
		await testPage.getByRole('button', { name: 'Save membership' }).click();

		await expect
			.element(testPage.getByText('a practice must keep at least one Owner'))
			.toBeVisible();
		await expect.element(testPage.getByRole('group', { name: 'Roles' })).toBeVisible();
	});

	it('closes the edit form without changing the membership when cancelled', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Edit membership' }).first().click();
		await testPage.getByRole('checkbox', { name: 'Doula' }).click();
		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(testPage.getByRole('group', { name: 'Roles' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('cell', { name: 'owner' })).toBeVisible();
	});

	it('removes a revoked invitation from the pending group', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Revoke' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Revoke invitation' });
		await dialog.getByRole('button', { name: 'Revoke invitation' }).click();

		await expect
			.element(testPage.getByRole('cell', { name: 'lena@example.com' }))
			.not.toBeInTheDocument();
		await expect
			.element(testPage.getByRole('cell', { name: 'undeliverable@example.com' }))
			.toBeVisible();
	});

	// #291: a lapsed Invitation still holds its address slot, so it stays
	// on the screen with something the Owner can do about it.
	it('flags a lapsed invitation instead of hiding it', async () => {
		await setup();

		await expect
			.element(testPage.getByText('Expired -- invite again or revoke'))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: 'undeliverable@example.com' }))
			.toBeVisible();
	});

	// #291: the route that was missing -- without it a roster row nobody
	// wants can never be taken off.
	it('removes a membership from the practice', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Remove from practice' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Remove from Practice' });
		await dialog.getByRole('button', { name: 'Remove from Practice' }).click();

		await expect
			.element(testPage.getByRole('cell', { name: 'Ada Lovelace' }))
			.not.toBeInTheDocument();
		await expect.element(testPage.getByRole('cell', { name: 'Grace Hopper' })).toBeVisible();
	});

	it('shows a per-row error notice when removing a membership fails', async () => {
		await setup({ removeResponse: textResponse('a practice must keep at least one Owner') });

		await testPage.getByRole('button', { name: 'Remove from practice' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Remove from Practice' });
		await dialog.getByRole('button', { name: 'Remove from Practice' }).click();

		await expect
			.element(testPage.getByText('a practice must keep at least one Owner'))
			.toBeVisible();
	});

	it('shows a per-row error notice when revoking fails', async () => {
		await setup({ revokeResponse: textResponse('no pending invitation found at this practice') });

		await testPage.getByRole('button', { name: 'Revoke' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Revoke invitation' });
		await dialog.getByRole('button', { name: 'Revoke invitation' }).click();

		await expect
			.element(testPage.getByText('no pending invitation found at this practice'))
			.toBeVisible();
	});

	it('ends sessions for a Staff member and shows a success notice for that row only', async () => {
		await setup();

		const buttons = testPage.getByRole('button', { name: 'End sessions everywhere' });
		await buttons.nth(1).click();
		const dialog = testPage.getByRole('dialog', { name: 'End sessions everywhere' });
		await dialog.getByRole('button', { name: 'End sessions everywhere' }).click();

		const successNotices = testPage.getByText('Sessions ended.');
		await expect.element(successNotices).toBeVisible();
		expect(successNotices.elements()).toHaveLength(1);

		const rows = testPage.getByRole('row');
		await expect.element(rows.nth(1)).not.toHaveTextContent('Sessions ended.');
	});

	it('shows a per-row error notice when ending sessions fails', async () => {
		await setup({ sessionsResponse: textResponse('Failed to end sessions') });

		const buttons = testPage.getByRole('button', { name: 'End sessions everywhere' });
		await buttons.nth(0).click();
		const dialog = testPage.getByRole('dialog', { name: 'End sessions everywhere' });
		await dialog.getByRole('button', { name: 'End sessions everywhere' }).click();

		await expect.element(testPage.getByText('Failed to end sessions')).toBeVisible();
	});

	// #459: the roster column shows the current value and the day it was
	// asserted, which answers "how did this get set?" only while the value
	// has never moved. These four hold the four things it had to decide.
	describe('work state history', () => {
		it('is closed until it is opened, and then names both sides of a move', async () => {
			await setup();

			const disclosures = testPage.getByText('Work state history');
			await expect.element(disclosures.first()).toBeVisible();
			// Nothing is fetched until she asks for it: the roster read is
			// the only request so far.
			expect(
				apiFetchWithSession.mock.calls.filter((call: unknown[]) =>
					String(call[0]).includes('/work-state-history')
				)
			).toHaveLength(0);

			await disclosures.first().click();

			await expect
				.element(testPage.getByText('Changed from New York to New Jersey'))
				.toBeVisible();
		});

		// A first assertion has no previous value (migration 00043 leaves it
		// NULL), and printing it as a change would invent one she never made.
		it('prints a first assertion as a report, not as a change', async () => {
			await setup();

			await testPage.getByText('Work state history').first().click();

			await expect.element(testPage.getByText('Reported New York')).toBeVisible();
		});

		// A contractor doula who asserted her work state at another Practice
		// carries that row into this one, and the screen must not read as
		// though she said it here.
		it('marks an assertion made before she joined this practice', async () => {
			await setup();

			await testPage.getByText('Work state history').nth(1).click();

			await expect.element(testPage.getByText('Reported California')).toBeVisible();
			await expect.element(testPage.getByText('(before joining this practice)')).toBeVisible();
		});

		it('shows a per-row error notice when the history fails to load', async () => {
			await setup({ historyResponse: textResponse('Failed to load work state history') });

			await testPage.getByText('Work state history').first().click();

			await expect.element(testPage.getByText('Failed to load work state history')).toBeVisible();
		});
	});
});
