import { createRawSnippet } from 'svelte';
import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import { jsonResponse } from '#lib/testResponse.js';
import Layout from './+layout.svelte';
import { resetAccountSession } from './session.svelte.js';
// The skip link parks itself off-screen with a transform written in
// spacing tokens, so without these the token is invalid, the transform
// computes to none, and an unfocused skip link sits over the top bar --
// where it swallows a click meant for the hamburger.
import '#lib/styles/tokens.css';

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

/*
 * The two sides of StaffTopBar's 49.25rem content floor, named the way
 * StaffTopBar's own spec names them. Which one is pinned decides which of
 * the bar's two trees is the visible one, so it is never left to the
 * runner's default.
 */
const WIDE = [1440, 900] as const;
const NARROW = [390, 844] as const;

interface RenderOptions {
	sessionResponse?: Response;
	viewport?: readonly [number, number];
}

async function renderLayout({
	sessionResponse = jsonResponse(session),
	viewport = WIDE
}: RenderOptions = {}) {
	// Pinned wide by default, same as practices-layout.svelte.spec.ts:
	// StaffTopBar keeps both its wide and narrow trees in the document with
	// one display:none, so which one is visible -- and so which holds the
	// accessible avatar button this spec queries by role -- is a fact about
	// the viewport. One test below pins it narrow on purpose, to reach the
	// sheet that only exists under the bar's content floor.
	await testPage.viewport(...viewport);
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

	/*
	 * #673. /account is scoped to the person, so it hands the bar no
	 * Practice at all -- and under the bar's content floor the nav and the
	 * switcher move into a sheet. The sheet has to drop its Practice
	 * heading here, not print it over an empty slot.
	 */
	it('shows no bare Practice heading in the narrow sheet', async () => {
		await renderLayout({ viewport: NARROW });

		await testPage.getByRole('button', { name: 'Menu' }).click();

		const sheet = testPage.getByRole('dialog');
		await expect.element(sheet).toBeVisible();
		await expect.element(sheet.getByText('Practice', { exact: true })).not.toBeInTheDocument();
	});

	it('offers a way back to every Practice she belongs to', async () => {
		await renderLayout();

		const back = testPage.getByRole('navigation', { name: 'Your practices' });
		await expect.element(back.getByRole('link', { name: 'Rochester Doulas' })).toBeVisible();
	});

	it('lists one link per Practice when she works at several', async () => {
		await renderLayout({
			sessionResponse: jsonResponse({
				...session,
				memberships: [
					...session.memberships,
					{ practiceId: 'practice-2', practiceName: 'Finger Lakes Birth', roles: ['doula'] }
				]
			})
		});

		const back = testPage.getByRole('navigation', { name: 'Your practices' });
		await expect.element(back.getByRole('link', { name: 'Finger Lakes Birth' })).toBeVisible();
		expect(back.getByRole('link').elements()).toHaveLength(2);
	});

	it('shows no nav when the session read fails', async () => {
		await renderLayout({ sessionResponse: jsonResponse('no matching staff account', 404) });

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
