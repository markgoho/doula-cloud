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

/*
 * A field-targeted refusal renders twice by GOV.UK's own design (#467):
 * once as a link in the error summary, once beside the control itself --
 * `LabeledField`'s own `<p role="alert">`. `getByText` cannot tell the
 * two apart, since both carry the identical words; this reads the one
 * beside the control, by the id `LabeledField` derives from the field's
 * own id, the same way `WorkStateField.svelte.spec.ts` reads a hint by
 * id where no accessible query can single it out either.
 */
async function fieldError(id: string, message: string) {
	await vi.waitFor(() => {
		expect(document.querySelector(`#${id}-error`)?.textContent).toBe(message);
	});
}

const signInWithEmailAndPassword = vi.hoisted(() => vi.fn());
const signOut = vi.hoisted(() => vi.fn());
const getMultiFactorResolver = vi.hoisted(() => vi.fn());
const assertionForSignIn = vi.hoisted(() => vi.fn());
vi.mock('firebase/auth', () => ({
	signInWithEmailAndPassword,
	signOut,
	getMultiFactorResolver,
	TotpMultiFactorGenerator: { assertionForSignIn }
}));
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
	for (const mock of [
		goto,
		apiFetch,
		apiFetchWithSession,
		signInWithEmailAndPassword,
		signOut,
		getMultiFactorResolver,
		assertionForSignIn
	])
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

/*
 * #606: the sign-in challenge for a Staff member who has already
 * enrolled a TOTP authenticator. `signInWithEmailAndPassword` itself
 * raises `auth/multi-factor-auth-required` and issues no credential at
 * all -- the credential only ever arrives from `resolver.resolveSignIn`,
 * once the code is confirmed.
 */
describe('Staff login -- the TOTP sign-in challenge (#606)', () => {
	const resolver = { hints: [{ uid: 'enrollment-1' }], resolveSignIn: vi.fn() };

	async function reachChallenge() {
		signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/multi-factor-auth-required' });
		getMultiFactorResolver.mockReturnValue(resolver);

		await render(Page, {});
		await testPage.getByLabelText('Email').fill('priya@example.com');
		await testPage.getByLabelText('Password').fill('correct horse');
		await testPage.getByRole('button', { name: 'Log in' }).click();

		await expect
			.element(testPage.getByLabelText('Authenticator app code'))
			.toBeVisible();
	}

	beforeEach(() => {
		resolver.resolveSignIn.mockReset();
	});

	it('challenges for the code instead of showing a refusal, and asks for nothing else', async () => {
		await reachChallenge();

		expect(testPage.getByLabelText('Email').elements()).toHaveLength(0);
		expect(testPage.getByLabelText('Password').elements()).toHaveLength(0);
		expect(testPage.getByText(/is not correct/i).elements()).toHaveLength(0);
	});

	it('refuses an empty code without resolving sign-in', async () => {
		await reachChallenge();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await fieldError('login-totp-code', 'Enter the 6-digit code from your authenticator app');
		expect(resolver.resolveSignIn).not.toHaveBeenCalled();
	});

	it('resolves sign-in with the one enrolled factor and finishes exactly like a plain sign-in', async () => {
		assertionForSignIn.mockReturnValue({ assertion: true });
		resolver.resolveSignIn.mockResolvedValue({
			user: { getIdToken: async () => 'id-token-after-mfa' }
		});
		vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({}, 200)));
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...session, memberships: [firstMembership] }));

		await reachChallenge();
		await testPage.getByLabelText('Authenticator app code').fill('123456');
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(assertionForSignIn).toHaveBeenCalledWith('enrollment-1', '123456');
		await vi.waitFor(() =>
			expect(goto).toHaveBeenCalledWith(`/practices/${firstMembership.practiceId}`)
		);
	});

	// #606's own AC: fails as a sign-in failure, not an app error, and she
	// can retry the code without being sent back to re-enter her password.
	it('shows a wrong or expired code as a refusal she can retry in place', async () => {
		resolver.resolveSignIn.mockRejectedValue({ code: 'auth/invalid-verification-code' });

		await reachChallenge();
		await testPage.getByLabelText('Authenticator app code').fill('000000');
		await testPage.getByRole('button', { name: 'Continue' }).click();

		await fieldError(
			'login-totp-code',
			'The code is not correct. Enter the 6-digit code from your authenticator app.'
		);
		expect(apiFetchWithSession).not.toHaveBeenCalled();
		expect(testPage.getByLabelText('Password').elements()).toHaveLength(0);
	});
});

/*
 * #610: a browser holds exactly one Doula Cloud session, so signing in
 * here while the Client portal's session is live ends that one. The BFF
 * refuses the first exchange and says what continuing costs; the page
 * shows it and offers the press-through.
 */
describe('Staff login -- signing in over a live portal session (#610)', () => {
	const WARNING = 'Continuing signs you out of the client portal in this browser.';

	/*
	 * Signs in far enough for the exchange to be refused, and hands back
	 * the fetch mock so a test can change what the second press gets.
	 */
	async function reachWarning() {
		apiFetch.mockResolvedValue(jsonResponse('no matching staff session', 404));
		signInWithEmailAndPassword.mockResolvedValue({
			user: { getIdToken: () => Promise.resolve('id-token') }
		});
		const exchange = vi
			.fn()
			.mockResolvedValue(jsonResponse({ code: 'FAILED_PRECONDITION', message: WARNING }, 409));
		vi.stubGlobal('fetch', exchange);

		await render(Page, {});
		await testPage.getByLabelText('Email').fill('priya@example.com');
		await testPage.getByLabelText('Password').fill('correct horse');
		await testPage.getByRole('button', { name: 'Log in' }).click();

		await expect.element(testPage.getByText(WARNING)).toBeVisible();
		return exchange;
	}

	it('shows what continuing costs instead of signing her in', async () => {
		const exchange = await reachWarning();

		await expect
			.element(testPage.getByRole('button', { name: 'Continue and sign out' }))
			.toBeVisible();
		// Refused, not failed: the first press minted nothing, and nothing
		// went on to read a session that does not exist.
		expect(apiFetchWithSession).not.toHaveBeenCalled();
		expect(exchange).toHaveBeenCalledTimes(1);
		expect(exchange.mock.calls[0][1].headers['X-Confirmed']).toBeUndefined();
	});

	it('sends the same exchange again, confirmed, when she presses through', async () => {
		const exchange = await reachWarning();
		exchange.mockResolvedValue(jsonResponse({ ok: true }));
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({ ...session, memberships: [firstMembership] })
		);

		await testPage.getByRole('button', { name: 'Continue and sign out' }).click();

		await vi.waitFor(() =>
			expect(goto).toHaveBeenCalledWith(`/practices/${firstMembership.practiceId}`)
		);
		expect(exchange).toHaveBeenCalledTimes(2);
		expect(exchange.mock.calls[1][1].headers['X-Confirmed']).toBe('true');
	});

	it('keeps her portal session and sends her back to the form when she cancels', async () => {
		const exchange = await reachWarning();

		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(testPage.getByRole('button', { name: 'Log in' })).toBeVisible();
		expect(testPage.getByText(WARNING).elements()).toHaveLength(0);
		// Nothing further asked of the BFF, so the portal session stands.
		expect(exchange).toHaveBeenCalledTimes(1);
		expect(signOut).toHaveBeenCalled();
	});
});
