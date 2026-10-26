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
	apiFetch
}));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const signOutOfSession = vi.hoisted(() => vi.fn<() => Promise<SignOutOutcome>>());
vi.mock('#lib/signOut.js', () => ({ signOutOfSession }));

const session = {
	staffId: 'staff-1',
	name: 'Priya Sharma',
	firstName: 'Priya',
	lastName: 'Sharma',
	email: 'priya@example.test',
	workState: 'NY',
	workStateReportedAt: '2026-08-28T14:02:11Z',
	lastPracticeId: 'practice-1',
	memberships: [{ practiceId: 'practice-1', practiceName: 'Rochester Doulas', roles: ['doula'] }]
};

/*
 * The two sides of StaffTopBar's 57.5rem content floor, named the way
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

	// #1837: an empty landmark announces a section with nothing in it.
	it('offers no Practice nav at all -- there is no Practice on this route', async () => {
		await renderLayout();

		await expect
			.element(testPage.getByRole('navigation', { name: 'Practice', exact: true }))
			.not.toBeInTheDocument();
	});

	/*
	 * #673, then #1837. /account is scoped to the person, so it hands the
	 * bar no Practice and no section. Under the bar's content floor a Menu
	 * button would open a sheet with nothing in it, so the bar draws none,
	 * and the avatar stays the way back to the account and to sign-out.
	 */
	it('draws no Menu button under the narrow floor', async () => {
		await renderLayout({ viewport: NARROW });

		await expect
			.element(testPage.getByRole('button', { name: 'Your account, Priya Sharma' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Menu' })).not.toBeInTheDocument();
		// The lockup takes the room the Menu button left, as on every other
		// Staff screen's bar. `.last()`: the wide tree's lockup comes first in
		// the document and is display:none here.
		await expect.element(testPage.getByRole('banner').getByText('DoulaCloud').last()).toBeVisible();
	});

	// #2044: the way back is Your account's own Back link, at the top of
	// its column, so the shell draws none at its foot.
	it('draws no way back of its own', async () => {
		await renderLayout();

		await expect.element(testPage.getByText('account page content')).toBeVisible();
		expect(testPage.getByRole('navigation', { name: 'Your practices' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('link', { name: 'Rochester Doulas' }).elements()).toHaveLength(0);
	});

	// A save on one of the account's question pages forgets the memoized
	// session before it goes back; the bar reads it again then.
	it('names her by the name she has just saved, without a reload', async () => {
		await renderLayout();
		await expect.element(testPage.getByRole('button', { name: 'Your account, Priya Sharma' })).toBeVisible();

		apiFetchWithSession.mockImplementation(() =>
			Promise.resolve(jsonResponse({ ...session, firstName: 'Priya', lastName: 'Sharma-Okafor' }))
		);
		resetAccountSession();

		await expect
			.element(testPage.getByRole('button', { name: 'Your account, Priya Sharma-Okafor' }))
			.toBeVisible();
	});

	it('names no one when the session read fails', async () => {
		await renderLayout({ sessionResponse: jsonResponse('no matching staff account', 404) });

		await expect.element(testPage.getByText('account page content')).toBeVisible();
		expect(testPage.getByRole('button', { name: /Priya/ }).elements()).toHaveLength(0);
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

describe('the Pilot banner and Feedback drawer (#1527)', () => {
	it('shows the Staff banner sentence on /account too', async () => {
		await renderLayout();

		await expect
			.element(
				testPage.getByText('DoulaCloud is new, and you are one of the first to use it.', { exact: false })
			)
			.toBeVisible();
	});

	it('posts a send to /api/staff/feedback with no practiceId, since /account carries none', async () => {
		await renderLayout();

		await testPage.getByRole('button', { name: 'Tell us what is not working or what you need.' }).click();
		await testPage.getByLabelText('Something else').click();
		await testPage.getByRole('button', { name: 'Send feedback' }).click();

		await expect
			.poll(() => apiFetchWithSession.mock.calls.some((call: unknown[]) => call[0] === '/api/staff/feedback'))
			.toBe(true);
		const [, init] = apiFetchWithSession.mock.calls.find(
			(call: unknown[]) => call[0] === '/api/staff/feedback'
		) as [string, RequestInit];
		expect(JSON.parse(init.body as string)).toMatchObject({ kind: 'something_else' });
		expect(JSON.parse(init.body as string).practiceId).toBeUndefined();
	});
});
