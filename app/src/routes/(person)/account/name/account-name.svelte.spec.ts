import { page as testPage, userEvent } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { loadAccountSession, resetAccountSession } from '../../session.svelte.js';
import { session } from '../page.fixture.js';
import Page from './+page.svelte';

/*
 * Correcting her own name (#1537), on its own question page since #2044:
 * the Change on Your account's Name row. A save goes back to the summary
 * and says what it saved there.
 */
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession, apiBaseURL: () => '' }));

const gotoWithOutcome = vi.hoisted(() => vi.fn());
vi.mock('#lib/outcome.js', () => ({ gotoWithOutcome, takeOutcome: vi.fn() }));

interface SetupOptions {
	sessionResponse?: Response;
	saveResponse?: Response | Error;
}

async function setup({
	sessionResponse = jsonResponse(session),
	saveResponse = jsonResponse({ firstName: 'Mary Anne', lastName: 'Smith' })
}: SetupOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path === '/api/staff/session') return Promise.resolve(sessionResponse);
		return saveResponse instanceof Error ? Promise.reject(saveResponse) : Promise.resolve(saveResponse);
	});
	await render(Page, {});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	gotoWithOutcome.mockReset();
	resetAccountSession();
});

const firstName = () => testPage.getByLabelText('First name');
const lastName = () => testPage.getByLabelText('Last name');
const saveButton = () => testPage.getByRole('button', { name: 'Save your name' });

describe('the name question', () => {
	it('asks the question as the page heading, with a way back to the summary', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'What is your name?' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Back' })).toHaveAttribute('href', '/account');
	});

	it('shows her two names, each in its own field', async () => {
		await setup();

		await expect.element(firstName()).toHaveValue(session.firstName);
		await expect.element(lastName()).toHaveValue(session.lastName);
	});

	it('keeps what she has typed when the read lands after it', async () => {
		const read = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValue(read.promise);
		await render(Page, {});

		await firstName().fill('Mary Anne');
		read.resolve(jsonResponse(session));
		await loadAccountSession();

		await expect.element(firstName()).toHaveValue('Mary Anne');
		await expect.element(lastName()).toHaveValue('');
	});

	it('fills nothing when the read fails', async () => {
		await setup({ sessionResponse: jsonResponse('no matching staff account', 404) });

		await expect.element(firstName()).toBeVisible();
		await expect.element(firstName()).toHaveValue('');
	});

	it('sends both names, with no staff id, and goes back to the summary saying so', async () => {
		await setup();

		await firstName().fill('Mary Anne');
		await lastName().fill('Smith');
		await saveButton().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/name', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ firstName: 'Mary Anne', lastName: 'Smith' })
		});
		await vi.waitFor(() =>
			expect(gotoWithOutcome).toHaveBeenCalledWith('/account', 'Your name is Mary Anne Smith.')
		);
	});

	// The summary and the bar read a memoized session, which would still
	// hold the old name after the save.
	it('forgets the old session before it goes back, so the summary reads the new name', async () => {
		await setup();
		await expect.element(firstName()).toHaveValue(session.firstName);

		await saveButton().click();
		await vi.waitFor(() => expect(gotoWithOutcome).toHaveBeenCalled());
		const reads = apiFetchWithSession.mock.calls.filter(([path]) => path === '/api/staff/session').length;

		await loadAccountSession();
		expect(apiFetchWithSession.mock.calls.filter(([path]) => path === '/api/staff/session')).toHaveLength(
			reads + 1
		);
	});

	it('saves when she presses Enter in a name field', async () => {
		await setup();

		await lastName().fill('Smith');
		await lastName().click();
		await userEvent.keyboard('{Enter}');

		await vi.waitFor(() =>
			expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/name', expect.anything())
		);
	});

	it('refuses each empty name on its own, without calling the server', async () => {
		await setup();
		await expect.element(firstName()).toHaveValue(session.firstName);

		await firstName().fill('');
		await saveButton().click();

		await expect.element(testPage.getByText('Enter your first name').first()).toBeVisible();
		expect(apiFetchWithSession).not.toHaveBeenCalledWith('/api/staff/name', expect.anything());
		expect(testPage.getByText('Enter your last name').elements()).toHaveLength(0);

		await firstName().fill('Anne-Marie');
		await lastName().fill('');
		await saveButton().click();

		await expect.element(testPage.getByText('Enter your last name').first()).toBeVisible();
	});

	it("shows the server's own words when the save is refused, and stays", async () => {
		await setup({ saveResponse: jsonResponse('firstName and lastName are required', 400) });

		await saveButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('firstName and lastName are required');
		expect(gotoWithOutcome).not.toHaveBeenCalled();
	});

	it('owns a save that never reached the server at all', async () => {
		await setup({ saveResponse: new Error('The network dropped') });

		await saveButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
	});
});
