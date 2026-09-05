import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toApiResponder } from '../../routeFixture.js';
import { fixture, session } from './page.fixture.js';

// #283: on load, this screen probes for a live Staff session of its own
// and, if one exists, sends the visitor on exactly the way a fresh
// sign-in would -- reusing decideLanding rather than a second copy of
// that decision. These tests cover only that on-load probe; the sign-in
// form's own submit flow is unchanged and untouched here.

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const signInWithEmailAndPassword = vi.hoisted(() => vi.fn());
const signOut = vi.hoisted(() => vi.fn());
vi.mock('firebase/auth', () => ({ signInWithEmailAndPassword, signOut }));
vi.mock('#lib/firebase.js', () => ({ getFirebaseAuth: () => ({}) }));

/*
 * This screen calls `probeSession` directly rather than
 * `apiFetchWithSession` (#lib/api.js's own doc comment on why), so the
 * mock is one level lower, on `apiFetch`, with `probeSession` mirrored
 * here to close over it -- the same mirroring `route-continuum.svelte.
 * spec.ts` does for this route. That is what lets `toApiResponder(fixture)`
 * answer this route's fetch the same way it answers every other route's.
 */
const apiFetch = vi.hoisted(() => vi.fn());
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiBaseURL: () => '',
	apiFetchWithSession,
	probeSession: async <Session,>(path: string): Promise<Session | undefined> => {
		try {
			const response = await apiFetch(path);
			if (!response.ok) return undefined;
			return (await response.json()) as Session;
		} catch {
			return undefined;
		}
	}
}));

beforeEach(() => {
	for (const mock of [goto, apiFetch, apiFetchWithSession, signInWithEmailAndPassword, signOut])
		mock.mockReset();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

const [firstMembership, secondMembership] = session.memberships;

describe('Staff login -- on-load session probe (#283)', () => {
	it('redirects a signed-in visitor to her only Practice, without showing the form', async () => {
		// One Membership rather than the fixture's two, so this is a
		// departure from it -- written as a spread rather than a fresh
		// object that re-states the Membership fields it shares.
		apiFetch.mockResolvedValue(jsonResponse({ ...session, memberships: [firstMembership] }));

		await render(Page, {});

		await vi.waitFor(() =>
			expect(goto).toHaveBeenCalledWith(`/practices/${firstMembership.practiceId}`)
		);
		expect(apiFetch).toHaveBeenCalledWith('/api/staff/session');
	});

	it('shows the Practice picker for a signed-in visitor with several memberships and no last-used one', async () => {
		// The fixture's own session already has two Memberships and no
		// last-used one -- it is this test's happy path, not a reason to
		// invent a second one.
		apiFetch.mockImplementation(toApiResponder(fixture));

		await render(Page, {});

		await expect.element(testPage.getByRole('heading', { name: 'Choose a Practice' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: secondMembership.practiceName }))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('renders the ordinary login form for a signed-out visitor, with no session-ended messaging', async () => {
		apiFetch.mockResolvedValue(jsonResponse('no matching staff session', 404));

		await render(Page, {});

		await expect.element(testPage.getByLabelText('Email')).toBeVisible();
		expect(testPage.getByText(/session/i).elements()).toHaveLength(0);
		expect(goto).not.toHaveBeenCalled();
	});

	it('falls back to the ordinary form when the probe reports no session, network failure included', async () => {
		// probeSession itself swallows a thrown fetch and every non-OK
		// response (see its own tests in api.spec.ts); from this screen's
		// side, that failure is indistinguishable from "not signed in".
		apiFetch.mockResolvedValue(jsonResponse('no matching staff session', 404));

		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Log in' })).toBeVisible();
	});

	it('never probes the Client-portal session', async () => {
		apiFetch.mockResolvedValue(jsonResponse('no matching staff session', 404));

		await render(Page, {});

		await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(1));
		expect(apiFetch).not.toHaveBeenCalledWith('/api/portal/session');
	});
});

/*
 * #745: a credential Identity Platform accepts can still belong to
 * neither population -- a signup whose BFF half failed leaves exactly
 * that. This screen used to print the BFF's own `no matching staff
 * account` at her, which names an internal lookup and offers nothing to
 * do about it. These cover what the submit does with that 404 instead.
 */
async function signIn() {
	signInWithEmailAndPassword.mockResolvedValue({ user: { getIdToken: async () => 'id-token' } });
	// The session exchange (`POST /api/session`) is a plain, one-off
	// `fetch` rather than an `apiFetch`, so it is stubbed at that level.
	vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({}, 200)));

	await render(Page, {});
	await testPage.getByLabelText('Email').fill('priya@example.com');
	await testPage.getByLabelText('Password').fill('correct horse');
	await testPage.getByRole('button', { name: 'Log in' }).click();
}

describe('Staff login -- a credential that resolves to no Practice (#745)', () => {
	it('sends an identity that is in neither population to the no-Practice screen', async () => {
		apiFetch.mockResolvedValue(jsonResponse('no matching staff session', 404));
		apiFetchWithSession.mockResolvedValue(jsonResponse('no matching staff account', 404));

		await signIn();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/no-practice'));
	});

	// A Client signing in at the Staff door is a different problem, and
	// #610 owns it -- so the BFF's own refusal is left to stand.
	it('leaves a Client-portal identity with the refusal it already gets', async () => {
		apiFetch.mockImplementation(async (path: string) =>
			path === '/api/portal/session'
				? jsonResponse({ clientId: 'client-1', engagements: [] })
				: jsonResponse('no matching staff session', 404)
		);
		apiFetchWithSession.mockResolvedValue(jsonResponse('no matching staff account', 404));

		await signIn();

		await expect
			.element(testPage.getByText('no matching staff account', { exact: false }))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	// The same screen, reached the other way: her last Membership was
	// removed, so the session resolves to her and to no Practice at all.
	it('sends a Staff member with no Membership left to the same screen', async () => {
		apiFetch.mockResolvedValue(jsonResponse('no matching staff session', 404));
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({ ...session, memberships: [], lastPracticeId: undefined })
		);

		await signIn();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/no-practice'));
	});
});
