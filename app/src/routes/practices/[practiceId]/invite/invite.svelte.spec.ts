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

interface SetupOptions {
	response?: Response;
	rejectWith?: Error;
}

async function setup({ response, rejectWith }: SetupOptions = {}) {
	if (rejectWith) {
		apiFetchWithSession.mockRejectedValue(rejectWith);
	} else {
		apiFetchWithSession.mockResolvedValue(response ?? jsonResponse({ invitationId: 'invitation-1' }));
	}
	return render(Page, {});
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

	// #425: the page is a FormPage instance, not hand-rolled layout. The
	// h1 and the form measure come from the Template, and neither of the
	// two groups is named, so `invite` must print no legend it did not ask
	// for -- Roles and Employment type are MembershipFields' own.
	it('renders through FormPage, with no legend either group did not ask for', async () => {
		const { container } = await setup();

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'Invite a Staff member' }))
			.toBeVisible();
		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--form-max)');
		expect([...container.querySelectorAll('legend')].map((legend) => legend.textContent)).toEqual([
			'Roles',
			'Employment type'
		]);
	});

	// Fitts's Law, per the brief: the submit sits at the end of the form
	// the person just filled in, not in a toolbar away from it.
	it('renders the submit at the end of the form, after the last field', async () => {
		await setup();

		const lastField = testPage.getByRole('radio', { name: 'Contractor' }).element();
		const submit = testPage.getByRole('button', { name: 'Send invite' }).element();

		expect(
			lastField.compareDocumentPosition(submit) & Node.DOCUMENT_POSITION_FOLLOWING
		).toBeTruthy();
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
			response: jsonResponse('that address already holds a membership at this practice', 409)
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
