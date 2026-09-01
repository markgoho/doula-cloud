import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1', engagementId: 'engagement-1' } }
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: vi.fn(async (response: Response) => await response.text())
}));

// A real service-worker push subscription would fail in headless Chromium
// -- the summary this spec covers doesn't depend on it, so it's stood in
// with a no-op unsubscribe.
vi.mock('#lib/pushRefresh.js', () => ({ subscribeToThreadPushMessages: vi.fn(() => vi.fn()) }));

const DETAIL_URL = '/api/practices/practice-1/engagements/engagement-1';

interface Detail {
	engagementId: string;
	clientId: string;
	clientName: string;
	status: string;
	createdAt: string;
	dueDate?: string;
}

// onMount chains through Visits, Messages, both Plan types, Contract,
// Invoices and Offers after the Engagement read itself -- every other
// endpoint is answered with a refusal, which each section's own loader
// already treats as "nothing to show" rather than throwing.
async function setup(detail: Detail) {
	await testPage.viewport(1440, 900);
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path === DETAIL_URL) return Promise.resolve(jsonResponse(detail));
		return Promise.resolve(jsonResponse('not available', 403));
	});
	await render(Page);
}

describe('Staff Engagement detail summary', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('shows the due date under its own label, alongside Created (#538)', async () => {
		await setup({
			engagementId: 'engagement-1',
			clientId: 'client-1',
			clientName: 'Tasha Bell',
			status: 'active',
			createdAt: '2027-05-01T00:00:00Z',
			dueDate: '2027-06-15'
		});

		await expect.element(testPage.getByText('Due date')).toBeVisible();
		await expect.element(testPage.getByText('Jun 15, 2027')).toBeVisible();
		await expect.element(testPage.getByText('Created')).toBeVisible();
	});

	// ADR-0017: a postpartum-only Engagement has no due date. #538 asks for
	// nothing to show, not a blank row and not a placeholder -- the row is
	// left out of the DescriptionList's own items entirely.
	it('shows nothing for a null due date -- no blank label, no placeholder', async () => {
		await setup({
			engagementId: 'engagement-1',
			clientId: 'client-1',
			clientName: 'Tasha Bell',
			status: 'active',
			createdAt: '2027-05-01T00:00:00Z'
		});

		await expect.element(testPage.getByText('active')).toBeVisible();
		await expect.element(testPage.getByText(/due date/i)).not.toBeInTheDocument();
	});
});
