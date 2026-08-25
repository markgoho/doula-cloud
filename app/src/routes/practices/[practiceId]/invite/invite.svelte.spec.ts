import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1' } }
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

interface SetupOptions {
	response?: Response;
	rejectWith?: Error;
}

async function setup({ response, rejectWith }: SetupOptions = {}) {
	if (rejectWith) {
		apiFetchWithSession.mockRejectedValue(rejectWith);
	} else {
		apiFetchWithSession.mockResolvedValue(
			response ??
				({ ok: true, json: () => Promise.resolve({ invitationId: 'invitation-1' }) } as Response)
		);
	}
	await render(Page, {});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function fillAndSubmit(email: string) {
	await testPage.getByLabelText('Their email').fill(email);
	await testPage.getByRole('button', { name: 'Send invite' }).click();
}

describe('invite a Staff member screen', () => {
	// #266: an Invitation carries the Membership, so the form has to ask
	// for it -- a zero-role membership is no longer reachable.
	it('asks for the roles and employment type the invitee will hold', async () => {
		await setup();

		await expect.element(testPage.getByRole('group', { name: 'Roles' })).toBeVisible();
		await expect.element(testPage.getByRole('checkbox', { name: 'Doula' })).toBeChecked();
		await expect.element(testPage.getByRole('radio', { name: 'Employee' })).toBeChecked();
	});

	// The Invitation has no name column: a person names herself when she
	// accepts, so the Owner is never asked to.
	it('does not ask the Owner for the invitee name', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Their name')).not.toBeInTheDocument();
	});

	// The AC's point: the token is mailed to the invited address and to
	// nowhere else, so this screen has no link to copy.
	it('confirms the email is on its way without showing an accept link', async () => {
		await setup();

		await fillAndSubmit('lena@example.com');

		await expect
			.element(testPage.getByText('An email with a link to join is on its way to lena@example.com.'))
			.toBeVisible();
		await expect.element(testPage.getByText('accept-invite?token=')).not.toBeInTheDocument();
	});

	it('refuses to send an invitation carrying no roles', async () => {
		await setup();

		await testPage.getByRole('checkbox', { name: 'Doula' }).click();
		await fillAndSubmit('lena@example.com');

		await expect.element(testPage.getByText('Choose at least one role.')).toBeVisible();
		// Nothing was sent, so nothing is confirmed as on its way.
		await expect
			.element(testPage.getByText('An email with a link to join is on its way to lena@example.com.'))
			.not.toBeInTheDocument();
	});

	it('shows the error the server gives back', async () => {
		await setup({
			response: {
				ok: false,
				text: () => Promise.resolve('that address already holds a membership at this practice')
			} as Response
		});

		await fillAndSubmit('here@example.com');

		await expect
			.element(testPage.getByText('that address already holds a membership at this practice'))
			.toBeVisible();
	});

	it('shows a message when the request itself fails', async () => {
		await setup({ rejectWith: new Error('Network down') });

		await fillAndSubmit('lena@example.com');

		await expect.element(testPage.getByText('Network down')).toBeVisible();
	});
});
