import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';

/* The screen reads the `connect` query parameter Stripe redirects back
   with, so the mocked URL has to be settable per test rather than fixed
   at module scope. */
const mockPage = vi.hoisted(() => ({
	params: { practiceId: 'practice-1' },
	url: new URL('https://test.local/practices/practice-1/settings/payments')
}));
vi.mock('$app/state', () => ({ page: mockPage }));

function returnedFromStripe(parameter: 'return' | 'refresh') {
	mockPage.url = new URL(`https://test.local/practices/practice-1/settings/payments?connect=${parameter}`);
}

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	// website.ts reads a failure through this; the screen only ever hits
	// the happy path here, but the mock has to carry every export the
	// module tree imports.
	apiErrorMessage: (response: Response) => response.text()
}));

interface MockOptions {
	status?: string;
	roles?: string[];
	sessionOk?: boolean;
	requirementsDue?: string[];
	/* What #440's endpoint reports. `own` is the default because it is the
	   precondition every other assertion on this screen depends on --
	   without a declared website there is no button to assert about. */
	websiteMode?: 'undeclared' | 'own' | 'hosted';
	/* Whether the page we publish for her has been confirmed to load
	   (#443). `pending` is the default because it is where every hosted
	   page starts and where the happy path passes through. */
	pageState?: '' | 'pending' | 'live' | 'failed';
}

function mockApi({
	status = 'not_connected',
	roles = [],
	sessionOk = true,
	requirementsDue = [],
	websiteMode = 'own',
	pageState = 'pending'
}: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/session')) {
			return Promise.resolve(jsonResponse({ roles }, sessionOk ? 200 : 401));
		}
		if (path.endsWith('/website')) {
			return Promise.resolve(
				jsonResponse({
					mode: websiteMode,
					ownUrl: websiteMode === 'own' ? 'https://rochesterdoulas.com' : '',
					serviceDescription: '',
					cancellationPolicy: '',
					updatedBy: '',
					updatedAt: '',
					pageState: websiteMode === 'hosted' ? pageState : '',
					pageCheckedAt: '',
					pageCheckDetail: '',
					pageUrl: websiteMode === 'hosted' ? 'https://doula.cloud/p/rochester-doulas' : ''
				})
			);
		}
		return Promise.resolve(
			jsonResponse({
				status,
				cardPaymentsStatus: 'unsupported',
				payoutsStatus: 'unsupported',
				requirementsDue
			})
		);
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	mockPage.url = new URL('https://test.local/practices/practice-1/settings/payments');
});

describe('payments settings screen', () => {
	it('shows a Connect Stripe button for an Owner when not connected', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Not connected')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
	});

	it('tells a non-Owner to ask an Owner instead of showing a button', async () => {
		mockApi({ status: 'not_connected', roles: ['doula'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Ask a Practice Owner to connect Stripe.')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	it('hides the connect button once the account is active, even for an Owner', async () => {
		mockApi({ status: 'active', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Stripe Connect status:')).toBeVisible();
		await expect.element(testPage.getByText('Active', { exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: /Stripe/ })).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Ask a Practice Owner to connect Stripe.')).not.toBeInTheDocument();
	});

	it('offers to continue onboarding for an Owner mid-onboarding', async () => {
		mockApi({ status: 'onboarding_incomplete', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();
	});

	it('falls back to no-Owner UI if the session roles fetch fails, even for an actual Owner', async () => {
		mockApi({ status: 'not_connected', roles: [], sessionOk: false });
		await render(Page, {});

		await expect.element(testPage.getByText('Ask a Practice Owner to connect Stripe.')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});
});

describe('payments settings screen: the states Accounts v1 could not report', () => {
	it('offers no onboarding button while Stripe is reviewing', async () => {
		mockApi({ status: 'pending', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Awaiting Stripe review')).toBeVisible();
		await expect
			.element(testPage.getByText('Stripe is reviewing the details you submitted. Nothing is needed from you.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).not.toBeInTheDocument();
	});

	it('says invoicing works when only payouts are held up', async () => {
		mockApi({ status: 'payouts_restricted', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Taking payments, payouts on hold')).toBeVisible();
		await expect
			.element(
				testPage.getByText('Clients can pay their invoices, but Stripe cannot send the money to your bank yet.')
			)
			.toBeVisible();
	});

	it('counts what Stripe is still waiting on without leaking its field paths', async () => {
		mockApi({
			status: 'onboarding_incomplete',
			roles: ['owner'],
			requirementsDue: ['configuration.merchant.mcc', 'configuration.merchant.support.phone']
		});
		await render(Page, {});

		await expect.element(testPage.getByText('Stripe needs 2 more details from you.')).toBeVisible();
		await expect.element(testPage.getByText('configuration.merchant.mcc')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();
	});

	it('offers no onboarding button when payouts are held up with nothing to supply', async () => {
		mockApi({ status: 'payouts_restricted', roles: ['owner'], requirementsDue: [] });
		await render(Page, {});

		await expect.element(testPage.getByText('Taking payments, payouts on hold')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).not.toBeInTheDocument();
	});
});

describe('payments settings screen: what #442 refuses and what it warns about', () => {
	it('refuses the button, and says where to fix it, when no website has been declared', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'], websiteMode: 'undeclared' });
		await render(Page, {});

		await expect
			.element(
				testPage.getByText(
					'Stripe will not let you take Client payments until it can see where you are online.',
					{ exact: false }
				)
			)
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Answer the website question' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	it('opens the flow once a page is published here, not only when she has her own site', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'], websiteMode: 'hosted' });
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
	});

	/* #443. The URL Stripe would be handed 404s, and #382 established the
	   review of that URL is ongoing with no published SLA -- so the
	   rejection arrives weeks later with no visible cause. Blocked on the
	   same rule as the missing answer above, and PostConnectHandler
	   refuses the request too. */
	it('refuses the button when the page published for her does not load', async () => {
		mockApi({
			status: 'not_connected',
			roles: ['owner'],
			websiteMode: 'hosted',
			pageState: 'failed'
		});
		await render(Page, {});

		await expect
			.element(testPage.getByText('The page we publish for you is not loading', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Go to website settings' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	/* A page still waiting for its deploy must not block her: every
	   Practice passes through `pending` on the way to `live`. */
	it('opens the flow while her page is still waiting for its deploy', async () => {
		mockApi({
			status: 'not_connected',
			roles: ['owner'],
			websiteMode: 'hosted',
			pageState: 'pending'
		});
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeVisible();
	});

	it('says what Stripe will ask for before the button, not after it', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByRole('heading', { name: 'What Stripe will ask you for' })).toBeVisible();
		await expect.element(testPage.getByText('two-step authentication', { exact: false })).toBeVisible();
		await expect
			.element(testPage.getByText('last four digits of your Social Security number', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByText('bank routing and account numbers', { exact: false })).toBeVisible();
	});

	it('tells her where the text on her Clients card statements comes from', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'] });
		await render(Page, {});

		await expect
			.element(
				testPage.getByText("Stripe puts a short version of your Practice's name on your Clients' card statements", {
					exact: false
				})
			)
			.toBeVisible();
	});

	it('does not congratulate a Practice who came back from Stripe still restricted', async () => {
		returnedFromStripe('return');
		mockApi({
			status: 'onboarding_incomplete',
			roles: ['owner'],
			requirementsDue: ['defaults.profile.business_url']
		});
		await render(Page, {});

		await expect
			.element(testPage.getByText('Stripe still needs something from you before Clients can pay.', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByText('Stripe onboarding finished.', { exact: false })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeVisible();
	});
});

describe('payments settings screen: the one question the two website answers do not share', () => {
	it("warns a Practice on her own site that Stripe wants a description she has not written", async () => {
		mockApi({ status: 'not_connected', roles: ['owner'], websiteMode: 'own' });
		await render(Page, {});

		await expect
			.element(testPage.getByText('A short description of what your Practice offers', { exact: false }))
			.toBeVisible();
	});

	it('does not warn a Practice whose published page already carries one', async () => {
		mockApi({ status: 'not_connected', roles: ['owner'], websiteMode: 'hosted' });
		await render(Page, {});

		await expect
			.element(testPage.getByText('A short description of what your Practice offers', { exact: false }))
			.not.toBeInTheDocument();
	});
});
