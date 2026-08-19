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
}

function mockApi({ status = 'not_connected', roles = [], sessionOk = true }: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/session')) {
			return Promise.resolve(jsonResponse({ roles }, sessionOk));
		}
		return Promise.resolve(
			jsonResponse({ status, chargesEnabled: false, payoutsEnabled: false, detailsSubmitted: false })
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

		await expect.element(testPage.getByText('Stripe Connect status: Not connected')).toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).toBeInTheDocument();
	});

	it('tells a non-Owner to ask an Owner instead of showing a button', async () => {
		mockApi({ status: 'not_connected', roles: ['doula'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Ask a Practice Owner to connect Stripe.')).toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});

	it('hides the connect button once the account is active, even for an Owner', async () => {
		mockApi({ status: 'active', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('Stripe Connect status: Active')).toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: /Stripe/ })).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Ask a Practice Owner to connect Stripe.')).not.toBeInTheDocument();
	});

	it('offers to continue onboarding for an Owner mid-onboarding', async () => {
		mockApi({ status: 'onboarding_incomplete', roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Continue Stripe onboarding' })).toBeInTheDocument();
	});

	it('falls back to no-Owner UI if the session roles fetch fails, even for an actual Owner', async () => {
		mockApi({ status: 'not_connected', roles: [], sessionOk: false });
		await render(Page, {});

		await expect.element(testPage.getByText('Ask a Practice Owner to connect Stripe.')).toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});
});
