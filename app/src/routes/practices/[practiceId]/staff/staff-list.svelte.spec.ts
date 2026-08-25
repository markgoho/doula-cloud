import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1' } }
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

function jsonResponse(body: unknown): Response {
	return { ok: true, text: () => Promise.resolve(JSON.stringify(body)), json: () => Promise.resolve(body) } as Response;
}

function textResponse(body: string): Response {
	return { ok: false, text: () => Promise.resolve(body) } as Response;
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
		deliveryFailed: false
	},
	{
		invitationId: 'invitation-2',
		address: 'undeliverable@example.com',
		roles: ['admin'],
		employmentType: 'employee',
		expiresAt: '2026-09-02T00:00:00Z',
		deliveryFailed: true
	}
];

interface MockOptions {
	roster?: { members: typeof members; invitations: typeof invitations };
	listOk?: boolean;
	sessionsResponse?: Response;
	membershipResponse?: Response;
	revokeResponse?: Response;
}

// The API double is stateful: a successful PATCH or revoke changes what
// the next roster read returns, so a test can assert on the screen the
// Owner ends up looking at rather than on the request that got her there.
function mockApi({
	roster = { members, invitations },
	listOk = true,
	sessionsResponse,
	membershipResponse,
	revokeResponse
}: MockOptions = {}) {
	const state = structuredClone(roster);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (path.endsWith('/sessions') && init?.method === 'DELETE') {
			return Promise.resolve(sessionsResponse ?? jsonResponse({}));
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
			listOk ? jsonResponse(state) : textResponse('Server rejected the Staff list request')
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

		await expect
			.element(testPage.getByRole('cell', { name: 'lena@example.com' }))
			.not.toBeInTheDocument();
		await expect
			.element(testPage.getByRole('cell', { name: 'undeliverable@example.com' }))
			.toBeVisible();
	});

	it('shows a per-row error notice when revoking fails', async () => {
		await setup({ revokeResponse: textResponse('no pending invitation found at this practice') });

		await testPage.getByRole('button', { name: 'Revoke' }).first().click();

		await expect
			.element(testPage.getByText('no pending invitation found at this practice'))
			.toBeVisible();
	});

	it('ends sessions for a Staff member and shows a success notice for that row only', async () => {
		await setup();

		const buttons = testPage.getByRole('button', { name: 'End sessions everywhere' });
		await buttons.nth(1).click();

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

		await expect.element(testPage.getByText('Failed to end sessions')).toBeVisible();
	});
});
