import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { workStateReportedOn } from '#lib/workStates.js';
import { jsonResponse as buildResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { resetAccountSession } from './session.svelte.js';

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	// The real one reads a plain-text body or a {code, message} JSON body
	// without the caller knowing which; the screen's job is only to show
	// whatever it says, so the mock is the plain-text half.
	apiErrorMessage: (response: Response) => response.text()
}));

const REPORTED_AT = '2026-08-28T14:02:11Z';
const SAVED_AT = '2027-03-14T10:00:00Z';

function jsonResponse(body: unknown): Response {
	return buildResponse(body);
}

function refusal(status: number, message: string): Response {
	return buildResponse(message, status);
}

const session = {
	staffId: 'staff-1',
	name: 'Priya Sharma',
	workState: 'NY',
	workStateReportedAt: REPORTED_AT,
	lastPracticeId: 'practice-1',
	memberships: [{ practiceId: 'practice-1', practiceName: 'Rochester Doulas', roles: ['doula'] }]
};

interface MockOptions {
	sessionResponse?: Response;
	saveResponse?: Response;
	saveThrows?: boolean;
}

function mockApi({
	sessionResponse = jsonResponse(session),
	saveResponse = jsonResponse({ workState: 'NJ', workStateReportedAt: SAVED_AT }),
	saveThrows = false
}: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path === '/api/staff/session') return Promise.resolve(sessionResponse);
		if (saveThrows) return Promise.reject(new Error('The network dropped'));
		return Promise.resolve(saveResponse);
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	// loadAccountSession() memoizes its in-flight request at module scope
	// (#474), so a fresh test needs a clean slate rather than replaying the
	// previous test's fetch.
	resetAccountSession();
});

const saveButton = () => testPage.getByRole('button', { name: 'Save work state' });
const stateSelect = () => testPage.getByRole('combobox', { name: 'Which state do you work from?' });

describe('the account screen', () => {
	it('shows the work state she has already asserted, and the day she asserted it', async () => {
		mockApi();
		await render(Page, {});

		await expect.element(testPage.getByRole('heading', { name: 'Your account' })).toBeVisible();
		await expect.element(stateSelect()).toHaveValue('New York');
		await expect
			.element(testPage.getByText(`Last confirmed ${workStateReportedOn(REPORTED_AT)}.`))
			.toBeVisible();
	});

	// The consequence, before the choice rather than in a dialog after it
	// -- and the answer to the question it invites, which is whether a
	// correction reaches backwards. It does not (#420).
	it('states the sales tax consequence and that past purchases are not re-priced', async () => {
		mockApi();
		await render(Page, {});

		await expect
			.element(testPage.getByText(/sets how much sales tax your practice pays/))
			.toBeVisible();
		await expect.element(testPage.getByText(/are not re-priced/)).toBeVisible();
	});

	it('sends the USPS code for the state she picked, with no staff id of its own', async () => {
		mockApi();
		await render(Page, {});

		await stateSelect().selectOptions('New Jersey');
		await saveButton().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/work-state', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ workState: 'NJ' })
		});
	});

	it('confirms the save and moves the last-confirmed day to the one the server returned', async () => {
		mockApi();
		await render(Page, {});

		await stateSelect().selectOptions('New Jersey');
		await saveButton().click();

		await expect.element(testPage.getByRole('status')).toHaveTextContent(
			'Saved. You work from New Jersey.'
		);
		await expect
			.element(testPage.getByText(`Last confirmed ${workStateReportedOn(SAVED_AT)}.`))
			.toBeVisible();
	});

	// The re-assertion case, which is the reason the button is never
	// disabled on an unchanged value: "yes, still New York, as of today"
	// is the only staleness signal the design has.
	it('sends the same state again rather than treating it as a no-op', async () => {
		mockApi({ saveResponse: jsonResponse({ workState: 'NY', workStateReportedAt: SAVED_AT }) });
		await render(Page, {});

		await expect.element(saveButton()).toBeEnabled();
		await saveButton().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			'/api/staff/work-state',
			expect.objectContaining({ body: JSON.stringify({ workState: 'NY' }) })
		);
		await expect
			.element(testPage.getByText(`Last confirmed ${workStateReportedOn(SAVED_AT)}.`))
			.toBeVisible();
	});

	// The practices nav is layout chrome, not this page's business -- see
	// account-layout.svelte.spec.ts (#474).
});

describe('when the account screen cannot do its job', () => {
	// A verified identity with no staff row behind it. Signed in, but
	// nobody here yet -- so there is nothing to edit and no form to offer.
	it('says so, and offers no form, when there is no Staff account', async () => {
		mockApi({ sessionResponse: refusal(404, 'no matching staff account') });
		await render(Page, {});

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('no matching staff account');
		expect(saveButton().elements()).toHaveLength(0);
	});

	it("shows the server's own words when a save is refused", async () => {
		mockApi({
			saveResponse: refusal(
				400,
				'workState is required, and must be a two-letter US state abbreviation'
			)
		});
		await render(Page, {});

		await saveButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('workState is required, and must be a two-letter US state abbreviation');
	});

	/*
	 * The thrown message is the network's. Since #467 this screen owns the
	 * failure in words she can act on rather than repeating an exception,
	 * and reports it in the error summary above the title.
	 */
	it('owns a save that never reached the server at all', async () => {
		mockApi({ saveThrows: true });
		await render(Page, {});

		await saveButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
		await expect.element(testPage.getByText('The network dropped')).not.toBeInTheDocument();
	});
});
