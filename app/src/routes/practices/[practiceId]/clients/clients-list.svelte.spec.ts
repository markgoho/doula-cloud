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

const clients = [
	{
		clientId: 'client-1',
		name: 'Ada Lovelace',
		email: 'ada@example.com',
		hasWork: true,
		portalInviteStatus: 'sent'
	},
	{
		clientId: 'client-2',
		name: 'Grace Hopper',
		email: 'grace@example.com',
		hasWork: false
	}
];

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function setup(response: Response = jsonResponse(clients)) {
	apiFetchWithSession.mockResolvedValue(response);
	await render(Page, {});
}

describe('clients list screen', () => {
	it('renders a Portal invite column showing the label for each status', async () => {
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Portal invite' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Invite sent' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Never invited' })).toBeVisible();
	});

	it.each([
		['pending', 'Invite pending'],
		['bounced', 'Bounced — needs re-invite'],
		['dead_lettered', 'Dead-lettered — needs re-invite'],
		['complained', 'Marked as spam (no action needed)'],
		['accepted', 'Accepted']
	])('shows %s as %s', async (portalInviteStatus, label) => {
		await setup(
			jsonResponse([
				{
					clientId: 'client-1',
					name: 'Ada Lovelace',
					email: 'ada@example.com',
					hasWork: true,
					portalInviteStatus
				}
			])
		);

		await expect.element(testPage.getByRole('cell', { name: label })).toBeVisible();
	});

	it('distinguishes complained from bounced and dead-lettered as non-actionable', async () => {
		await setup(
			jsonResponse([
				{
					clientId: 'client-1',
					name: 'Complained Client',
					email: 'complained@example.com',
					hasWork: true,
					portalInviteStatus: 'complained'
				},
				{
					clientId: 'client-2',
					name: 'Bounced Client',
					email: 'bounced@example.com',
					hasWork: true,
					portalInviteStatus: 'bounced'
				}
			])
		);

		await expect
			.element(testPage.getByRole('cell', { name: 'Marked as spam (no action needed)' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Bounced — needs re-invite' })).toBeVisible();
	});

	it('shows the empty message when there are no Clients', async () => {
		await setup(jsonResponse([]));

		await expect.element(testPage.getByText('No Clients yet.')).toBeVisible();
	});

	it('shows an error notice when the Clients list fails to load', async () => {
		await setup(textResponse('Server rejected the Clients list request'));

		await expect.element(testPage.getByText('Server rejected the Clients list request')).toBeVisible();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
	});
});
