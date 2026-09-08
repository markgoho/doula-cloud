/*
 * #768's setting: how many days after an Invoice is raised it falls due.
 * Read by any Staff member, set by an Owner or an Admin, and defaulting
 * to 30 days at a Practice that has never chosen -- the screen says which
 * of the two it is showing, so "30" chosen and "30" inherited do not look
 * identical.
 *
 * Its own file rather than more of `payments-settings.svelte.spec.ts`,
 * whose mocks are shaped around the Connect status sequencing this
 * section is deliberately outside of.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

const termsPath = '/api/practices/practice-1/payments/payment-terms';

/** Answers every read this screen makes, with the payment terms under the
 * test's control and everything else fixed -- the Connect status and the
 * website are other sections' subject, not this one's. */
function mockApi(terms: { netDays: number; isDefault: boolean }, roles = ['owner']) {
	pageState.data = {
		session: { practiceId: 'practice-1', practiceName: 'Riverside Doula Collective', roles, isContractor: false }
	};
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (path.endsWith('/payments/payment-terms')) {
			if (init?.method === 'PUT') {
				const body = JSON.parse(String(init.body)) as { netDays: number };
				return Promise.resolve(jsonResponse({ netDays: body.netDays, isDefault: false }));
			}
			return Promise.resolve(jsonResponse(terms));
		}
		if (path.endsWith('/payments/billing-mode')) {
			return Promise.resolve(jsonResponse({ billingMode: 'stripe' }));
		}
		if (path.endsWith('/website')) {
			return Promise.resolve(
				jsonResponse({
					mode: 'own',
					ownUrl: 'https://rochesterdoulas.com',
					serviceDescription: '',
					cancellationPolicy: '',
					updatedBy: '',
					updatedAt: '',
					pageState: '',
					pageCheckedAt: '',
					pageCheckDetail: '',
					pageUrl: ''
				})
			);
		}
		return Promise.resolve(
			jsonResponse({
				status: 'not_connected',
				cardPaymentsStatus: 'unsupported',
				payoutsStatus: 'unsupported',
				requirementsDue: []
			})
		);
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	pageState.url = new URL(fixture.url);
});

describe('payments settings screen: payment terms (#768)', () => {
	it('names the default as a default, rather than as a number the Practice chose', async () => {
		mockApi({ netDays: 30, isDefault: true });
		await render(Page, {});

		await expect.element(testPage.getByText(/That is the default/)).toBeVisible();
	});

	it("states a Practice's own terms plainly once it has set them", async () => {
		mockApi({ netDays: 45, isDefault: false });
		await render(Page, {});

		await expect.element(testPage.getByText('Invoices are due 45 days after they are raised.')).toBeVisible();
	});

	it('lets an Owner change the terms, and says the change reaches the next invoice only', async () => {
		mockApi({ netDays: 30, isDefault: true });
		await render(Page, {});

		await expect.element(testPage.getByText(/That is the default/)).toBeVisible();
		await expect.element(testPage.getByText(/keeps the terms it was billed under/)).toBeVisible();
		await testPage.getByLabelText('Days to pay').fill('15');
		await testPage.getByRole('button', { name: 'Update payment terms' }).click();

		await expect
			.poll(() =>
				apiFetchWithSession.mock.calls.some(
					(call: unknown[]) =>
						call[0] === termsPath && (call[1] as RequestInit | undefined)?.method === 'PUT'
				)
			)
			.toBe(true);
		await expect.element(testPage.getByText('Invoices are due 15 days after they are raised.')).toBeVisible();
	});

	it('refuses an impossible term in the words of the field, before the request is made', async () => {
		mockApi({ netDays: 30, isDefault: true });
		await render(Page, {});

		await expect.element(testPage.getByText(/That is the default/)).toBeVisible();
		await testPage.getByLabelText('Days to pay').fill('0');
		await testPage.getByRole('button', { name: 'Update payment terms' }).click();

		await expect.element(testPage.getByText(/whole number of days between 1 and 365/)).toBeVisible();
		expect(
			apiFetchWithSession.mock.calls.some(
				(call: unknown[]) => (call[1] as RequestInit | undefined)?.method === 'PUT'
			)
		).toBe(false);
	});

	it('shows a Doula the terms without a control to change them', async () => {
		mockApi({ netDays: 45, isDefault: false }, ['doula']);
		await render(Page, {});

		await expect.element(testPage.getByText('Invoices are due 45 days after they are raised.')).toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Update payment terms' }))
			.not.toBeInTheDocument();
	});
});
