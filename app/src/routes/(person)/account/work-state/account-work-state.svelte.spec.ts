import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { loadAccountSession, resetAccountSession } from '../../session.svelte.js';
import { session } from '../page.fixture.js';
import Page from './+page.svelte';

/*
 * The work state (#437), on its own question page since #2044: the
 * Change on Your account's "Where you work" row. A save goes back to
 * the summary and says what it saved there.
 */
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession, apiBaseURL: () => '' }));

const gotoWithOutcome = vi.hoisted(() => vi.fn());
vi.mock('#lib/outcome.js', () => ({ gotoWithOutcome, takeOutcome: vi.fn() }));

const SAVED_AT = '2027-03-14T10:00:00Z';

interface SetupOptions {
	sessionResponse?: Response;
	saveResponse?: Response | Error;
}

async function setup({
	sessionResponse = jsonResponse(session),
	saveResponse = jsonResponse({ workState: 'NJ', workStateReportedAt: SAVED_AT })
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

const stateSelect = () => testPage.getByRole('combobox', { name: 'Which state do you work from?' });
const saveButton = () => testPage.getByRole('button', { name: 'Save where you work' });

describe('the work-state question', () => {
	it('asks the question as the page heading, answered with the state she reported', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'Which state do you work from?' }))
			.toBeVisible();
		await expect.element(stateSelect()).toHaveValue('New York');
		await expect.element(testPage.getByRole('link', { name: 'Back' })).toHaveAttribute('href', '/account');
	});

	// The consequence, before the choice rather than in a dialog after it
	// -- and the answer to the question it invites, which is whether a
	// correction reaches backwards. It does not (#420). One hint carries
	// both (#2044), and the select is described by it.
	it('states the sales tax consequence and that past purchases are not re-priced, in its one hint', async () => {
		await setup();

		await expect
			.element(stateSelect())
			.toHaveAccessibleDescription(/sales tax on your Practice's Credits.*are not re-priced/);
	});

	it('keeps a state she chose before the read landed', async () => {
		const read = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValue(read.promise);
		await render(Page, {});

		await stateSelect().selectOptions('Vermont');
		read.resolve(jsonResponse(session));
		await loadAccountSession();

		await expect.element(stateSelect()).toHaveValue('Vermont');
	});

	it('sends the USPS code for the state she picked, with no staff id, and goes back saying so', async () => {
		await setup();

		await stateSelect().selectOptions('New Jersey');
		await saveButton().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/work-state', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ workState: 'NJ' })
		});
		await vi.waitFor(() =>
			expect(gotoWithOutcome).toHaveBeenCalledWith('/account', 'You work from New Jersey.')
		);
	});

	// The re-assertion case, which is the reason the button is never
	// disabled on an unchanged value: "yes, still New York, as of today"
	// is the only staleness signal the design has.
	it('sends the same state again rather than treating it as a no-op', async () => {
		await setup({ saveResponse: jsonResponse({ workState: 'NY', workStateReportedAt: SAVED_AT }) });
		await expect.element(stateSelect()).toHaveValue('New York');

		await expect.element(saveButton()).toBeEnabled();
		await saveButton().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			'/api/staff/work-state',
			expect.objectContaining({ body: JSON.stringify({ workState: 'NY' }) })
		);
		await vi.waitFor(() =>
			expect(gotoWithOutcome).toHaveBeenCalledWith('/account', 'You work from New York.')
		);
	});

	it('refuses an empty choice against the select, without calling the server', async () => {
		await setup({ sessionResponse: jsonResponse({ ...session, workState: '' }) });

		await saveButton().click();

		await expect.element(stateSelect()).toHaveAccessibleDescription(/Choose the state you work from/);
		expect(apiFetchWithSession).not.toHaveBeenCalledWith('/api/staff/work-state', expect.anything());
	});

	it("shows the server's own words when a save is refused, and stays", async () => {
		await setup({
			saveResponse: jsonResponse('workState is required, and must be a two-letter US state abbreviation', 400)
		});
		await expect.element(stateSelect()).toHaveValue('New York');

		await saveButton().click();

		await expect
			.element(testPage.getByRole('alert').first())
			.toHaveTextContent('workState is required, and must be a two-letter US state abbreviation');
		expect(gotoWithOutcome).not.toHaveBeenCalled();
	});

	it('owns a save that never reached the server at all', async () => {
		await setup({ saveResponse: new Error('The network dropped') });
		await expect.element(stateSelect()).toHaveValue('New York');

		await saveButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
		await expect.element(testPage.getByText('The network dropped')).not.toBeInTheDocument();
	});
});
