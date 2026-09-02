import { createRawSnippet } from 'svelte';
import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import { jsonResponse } from '#lib/testResponse.js';
import Layout from './+layout.svelte';
import { resetAccountSession } from './session.svelte.js';

const apiFetchWithSession = vi.hoisted(() => vi.fn());
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiFetch,
	apiErrorMessage: (response: Response) => response.text()
}));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const signOutOfSession = vi.hoisted(() => vi.fn<() => Promise<SignOutOutcome>>());
vi.mock('#lib/signOut.js', () => ({ signOutOfSession }));

const session = {
	staffId: 'staff-1',
	name: 'Priya Sharma',
	email: 'priya@example.test',
	workState: 'NY',
	workStateReportedAt: '2026-08-28T14:02:11Z',
	lastPracticeId: 'practice-1',
	memberships: [{ practiceId: 'practice-1', practiceName: 'Rochester Doulas', roles: ['doula'] }]
};

async function renderLayout(sessionResponse = jsonResponse(session)) {
	// Pinned wide, same as practices-layout.svelte.spec.ts: StaffTopBar keeps
	// both its wide and narrow trees in the document with one display:none,
	// so which one is visible -- and so which holds the accessible avatar
	// button this spec queries by role -- is a fact about the viewport.
	await testPage.viewport(1440, 900);
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(() => Promise.resolve(sessionResponse));
	apiFetch.mockReset();
	goto.mockReset();
	signOutOfSession.mockReset();
	signOutOfSession.mockResolvedValue({ ok: true });
	// loadAccountSession() memoizes its in-flight request at module scope
	// (#474), so a fresh test needs a clean slate rather than replaying the
	// previous test's fetch.
	resetAccountSession();
	await render(Layout, {
		children: createRawSnippet(() => ({ render: () => '<p>account page content</p>' }))
	});
	return { avatar: testPage.getByRole('button', { name: 'Your account, Priya Sharma' }) };
}

describe('the account route layout', () => {
	it('renders the page content it wraps', async () => {
		await renderLayout();

		await expect.element(testPage.getByText('account page content')).toBeVisible();
	});

	it('gives the page a main landmark for the skip link to target', async () => {
		await renderLayout();

		await expect.element(testPage.getByRole('main')).toBeVisible();
	});

	it('puts a skip link ahead of the bar, because every authenticated Staff route carries one', async () => {
		await renderLayout();

		await expect
			.element(testPage.getByRole('link', { name: 'Skip to main content' }))
			.toHaveAttribute('href', '#main');
	});

	it('offers no Practice-scoped nav items -- there is no Practice on this route', async () => {
		await renderLayout();

		const practiceNav = testPage.getByRole('navigation', { name: 'Practice' });
		expect(practiceNav.getByRole('link').elements()).toHaveLength(0);
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

	it('signs out without an unregister -- /account never carries a Practice', async () => {
		const { avatar } = await renderLayout();

		await avatar.click();
		await testPage.getByRole('button', { name: 'Sign out' }).click();

		expect(signOutOfSession).toHaveBeenCalledWith(
			expect.objectContaining({ unsubscribeURL: undefined, fetcher: apiFetch })
		);
		expect(goto).toHaveBeenCalledWith('/login');
	});

	it('stays put and reports a sign-out that failed', async () => {
		const { avatar } = await renderLayout();
		signOutOfSession.mockResolvedValue({ ok: false, message: 'Sign-out failed.' });

		await avatar.click();
		await testPage.getByRole('button', { name: 'Sign out' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('Sign-out failed.');
		expect(goto).not.toHaveBeenCalled();
	});
});
