import { createRawSnippet } from 'svelte';
import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Layout from './+layout.svelte';
import { resetAccountSession } from './session.svelte.js';

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

const session = {
	staffId: 'staff-1',
	name: 'Priya Sharma',
	workState: 'NY',
	workStateReportedAt: '2026-08-28T14:02:11Z',
	lastPracticeId: 'practice-1',
	memberships: [{ practiceId: 'practice-1', practiceName: 'Rochester Doulas', roles: ['doula'] }]
};

function renderLayout(sessionResponse = jsonResponse(session)) {
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(() => Promise.resolve(sessionResponse));
	// loadAccountSession() memoizes its in-flight request at module scope
	// (#474), so a fresh test needs a clean slate rather than replaying the
	// previous test's fetch.
	resetAccountSession();
	return render(Layout, {
		children: createRawSnippet(() => ({ render: () => '<p>account page content</p>' }))
	});
}

describe('the account route layout', () => {
	it('renders the page content it wraps', async () => {
		await renderLayout();

		await expect.element(testPage.getByText('account page content')).toBeVisible();
	});

	it('offers a way back to every Practice she belongs to', async () => {
		await renderLayout();

		const back = testPage.getByRole('navigation', { name: 'Your practices' });
		await expect.element(back.getByRole('link', { name: 'Rochester Doulas' })).toBeVisible();
	});

	it('lists one link per Practice when she works at several', async () => {
		await renderLayout(
			jsonResponse({
				...session,
				memberships: [
					...session.memberships,
					{ practiceId: 'practice-2', practiceName: 'Finger Lakes Birth', roles: ['doula'] }
				]
			})
		);

		const back = testPage.getByRole('navigation', { name: 'Your practices' });
		await expect.element(back.getByRole('link', { name: 'Finger Lakes Birth' })).toBeVisible();
		expect(back.getByRole('link').elements()).toHaveLength(2);
	});

	it('shows no nav when the session read fails', async () => {
		await renderLayout(jsonResponse('no matching staff account', 404));

		expect(testPage.getByRole('navigation', { name: 'Your practices' }).elements()).toHaveLength(
			0
		);
	});
});
