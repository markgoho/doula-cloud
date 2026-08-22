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

const staff = [
	{ staffId: 'staff-1', name: 'Ada Lovelace', email: 'ada@example.com', roles: ['owner'] },
	{ staffId: 'staff-2', name: 'Grace Hopper', email: 'grace@example.com', roles: [] }
];

interface MockOptions {
	staffList?: typeof staff;
	listOk?: boolean;
	sessionsResponse?: Response;
}

function mockApi({ staffList = staff, listOk = true, sessionsResponse }: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (path.endsWith('/sessions') && init?.method === 'DELETE') {
			return Promise.resolve(sessionsResponse ?? jsonResponse({}));
		}
		return Promise.resolve(listOk ? jsonResponse(staffList) : textResponse('Server rejected the Staff list request'));
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function setup(options: MockOptions = {}) {
	mockApi(options);
	await render(Page, {});
}

describe('staff list screen', () => {
	it('renders Name, Email, and Roles columns for each Staff member', async () => {
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Name' })).toBeVisible();
		await expect.element(testPage.getByRole('columnheader', { name: 'Email' })).toBeVisible();
		await expect.element(testPage.getByRole('columnheader', { name: 'Roles' })).toBeVisible();

		await expect.element(testPage.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'ada@example.com' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'owner' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'no roles yet' })).toBeVisible();
	});

	it('shows the empty message with no table when there is no Staff', async () => {
		await setup({ staffList: [] });

		await expect.element(testPage.getByText('No Staff yet.')).toBeVisible();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
	});

	it('shows an error notice when the Staff list fails to load', async () => {
		await setup({ listOk: false });

		await expect.element(testPage.getByText('Server rejected the Staff list request')).toBeVisible();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
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
