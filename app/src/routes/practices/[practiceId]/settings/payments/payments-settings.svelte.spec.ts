import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: {
		params: { practiceId: 'practice-1' },
		url: new URL('https://test.local/practices/practice-1/settings/payments')
	}
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native Response.ok property this mock stands in for
function jsonResponse(body: unknown, ok = true): Response {
	return { ok, text: () => Promise.resolve(JSON.stringify(body)), json: () => Promise.resolve(body) } as Response;
}

interface MockOptions {
	status?: string;
	roles?: string[];
	sessionOk?: boolean;
	requirementsDue?: string[];
}

function mockApi({ status = 'not_connected', roles = [], sessionOk = true, requirementsDue = [] }: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/session')) {
			return Promise.resolve(jsonResponse({ roles }, sessionOk));
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

	it('lists what Stripe is still waiting on', async () => {
		mockApi({
			status: 'onboarding_incomplete',
			roles: ['owner'],
			requirementsDue: ['configuration.merchant.mcc', 'configuration.merchant.support.phone']
		});
		await render(Page, {});

		await expect.element(testPage.getByText('Stripe is still waiting on:')).toBeVisible();
		await expect.element(testPage.getByText('configuration.merchant.mcc')).toBeVisible();
		await expect.element(testPage.getByText('configuration.merchant.support.phone')).toBeVisible();
	});
});
