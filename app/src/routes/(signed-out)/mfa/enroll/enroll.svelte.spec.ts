import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { expectFieldError } from '#lib/components/molecules/LabeledField.testing.js';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * TOTP enrollment (#606): step one re-authenticates and opens an
 * enrollment session, step two shows the QR code/secret and confirms the
 * code it produces. These specs fake the Firebase SDK surface entirely
 * -- `#lib/firebase.js`'s own doc comment is why: it needs a live
 * project or emulator this suite never runs against.
 */

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

function urlWith(returnTo?: string): URL {
	const url = new URL(fixture.url);
	if (returnTo) url.searchParams.set('returnTo', returnTo);
	return url;
}

const passwordId = 'mfa-enroll-password';
const codeId = 'mfa-enroll-code';

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const signInWithEmailAndPassword = vi.hoisted(() => vi.fn());
const signOut = vi.hoisted(() => vi.fn());
const multiFactorFunction = vi.hoisted(() => vi.fn());
const getSession = vi.hoisted(() => vi.fn());
const enroll = vi.hoisted(() => vi.fn());
const generateSecret = vi.hoisted(() => vi.fn());
const assertionForEnrollment = vi.hoisted(() => vi.fn());
vi.mock('firebase/auth', () => ({
	signInWithEmailAndPassword,
	signOut,
	multiFactor: multiFactorFunction,
	TotpMultiFactorGenerator: { generateSecret, assertionForEnrollment }
}));
vi.mock('#lib/firebase.js', () => ({ getFirebaseAuth: () => ({}) }));

const toDataURL = vi.hoisted(() => vi.fn());
vi.mock('qrcode', () => ({ default: { toDataURL }, toDataURL }));

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

const globalFetch = vi.hoisted(() => vi.fn());
vi.stubGlobal('fetch', globalFetch);

const session = {
	memberships: [{ practiceId: 'practice-1', practiceName: 'Riverside Doulas', roles: ['owner'] }],
	lastPracticeId: undefined,
	staffId: 'staff-1',
	name: 'Anne-Marie Ochieng-Whitfield',
	email: 'anne-marie@example.test',
	workState: 'NY',
	workStateReportedAt: '2026-01-01T00:00:00Z',
	secondFactor: false
};

const totpSecret = {
	secretKey: 'JBSWY3DPEHPK3PXP',
	generateQrCodeUrl: vi.fn(() => 'otpauth://totp/Doula%20Cloud:anne-marie@example.test')
};

beforeEach(() => {
	for (const mock of [
		goto,
		apiFetch,
		apiFetchWithSession,
		globalFetch,
		signInWithEmailAndPassword,
		signOut,
		multiFactorFunction,
		getSession,
		enroll,
		generateSecret,
		assertionForEnrollment,
		toDataURL
	])
		mock.mockReset();

	pageState.url = urlWith();
	apiFetch.mockResolvedValue(jsonResponse(session));
	multiFactorFunction.mockReturnValue({ getSession, enroll });
	getSession.mockResolvedValue({});
	generateSecret.mockResolvedValue(totpSecret);
	assertionForEnrollment.mockReturnValue({ assertion: true });
	toDataURL.mockResolvedValue('data:image/png;base64,fake');
	signInWithEmailAndPassword.mockResolvedValue({
		user: { emailVerified: true, getIdToken: vi.fn().mockResolvedValue('id-token') }
	});
	signOut.mockResolvedValue(undefined);
	enroll.mockResolvedValue(undefined);
});

afterEach(() => {
	vi.unstubAllGlobals();
	vi.stubGlobal('fetch', globalFetch);
});

async function goToSetupStep() {
	await render(Page, {});
	await testPage.getByLabelText('Password').fill('correct horse');
	await testPage.getByRole('button', { name: 'Continue' }).click();
	await expect.element(testPage.getByLabelText('Authenticator app code')).toBeVisible();
}

async function confirmCode() {
	await testPage.getByLabelText('Authenticator app code').fill('123456');
	await testPage.getByRole('button', { name: 'Confirm and turn on' }).click();
}

async function setup() {
	await render(Page, {});
}

describe('TOTP enrollment -- step one, re-authenticating', () => {
	it('asks for the password again rather than assuming a live sign-in', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Password')).toBeVisible();
	});

	it('sends a visitor with no session at all to the login screen', async () => {
		apiFetch.mockResolvedValue(jsonResponse('no matching staff session', 404));

		await setup();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/login'));
	});

	it('refuses an empty submission without calling Identity Platform', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expectFieldError(passwordId, 'Enter your password');
		expect(signInWithEmailAndPassword).not.toHaveBeenCalled();
	});

	it('reads the email from the session probe, not from a form field', async () => {
		await setup();
		await testPage.getByLabelText('Password').fill('correct horse');

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await vi.waitFor(() =>
			expect(signInWithEmailAndPassword).toHaveBeenCalledWith({}, session.email, 'correct horse')
		);
	});

	it('shows a wrong password as a step-up refusal', async () => {
		signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/wrong-password' });
		await setup();
		await testPage.getByLabelText('Password').fill('wrong');

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expectFieldError(passwordId, 'Password is not correct');
	});
});

/*
 * #1504: a new Owner arrives here before the verification message signup
 * queued has reached her, and Identity Platform refuses a second factor for
 * an unverified address. Wherever she arrives from, she is told to verify
 * first and never shown a QR code step that cannot succeed.
 */
function signInUnverified() {
	const user = {
		emailVerified: false,
		reload: vi.fn().mockResolvedValue(undefined),
		getIdToken: vi.fn().mockResolvedValue('id-token')
	};
	signInWithEmailAndPassword.mockResolvedValue({ user });
	return user;
}

async function goToVerifyStep() {
	await render(Page, {});
	await testPage.getByLabelText('Password').fill('correct horse');
	await testPage.getByRole('button', { name: 'Continue' }).click();
	await expect.element(testPage.getByRole('heading', { name: 'Verify your email address first' })).toBeVisible();
}

describe('TOTP enrollment -- an email address not yet verified (#1504)', () => {
	it('says to verify first, to which address, and how to get a new link, before any enrollment call', async () => {
		signInUnverified();

		await goToVerifyStep();

		await expect.element(testPage.getByText(session.email, { exact: false })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Send a new verification link' })).toBeVisible();
		expect(getSession).not.toHaveBeenCalled();
		expect(generateSecret).not.toHaveBeenCalled();
		expect(testPage.getByAltText(/QR code/).elements()).toHaveLength(0);
	});

	it('names the cause when Identity Platform refuses an address the sign-in showed as verified', async () => {
		generateSecret.mockRejectedValue({ code: 'auth/unverified-email' });

		await goToVerifyStep();
	});

	it('still reports any other refusal while opening the enrollment session', async () => {
		generateSecret.mockRejectedValue({ code: 'auth/network-request-failed' });
		await render(Page, {});
		await testPage.getByLabelText('Password').fill('correct horse');

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('We could not reach the service');
	});

	it('stays on the step, with the reason, while the address is still not verified', async () => {
		signInUnverified();
		await goToVerifyStep();

		await testPage.getByRole('button', { name: 'I have verified my email address' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('Your email address is not verified yet.');
		expect(generateSecret).not.toHaveBeenCalled();
	});

	it('moves on to the QR code once the address is verified', async () => {
		const user = signInUnverified();
		await goToVerifyStep();
		user.reload.mockImplementation(async () => {
			user.emailVerified = true;
		});

		await testPage.getByRole('button', { name: 'I have verified my email address' }).click();

		await expect.element(testPage.getByLabelText('Authenticator app code')).toBeVisible();
		expect(user.getIdToken).toHaveBeenCalledWith(true);
	});

	it('sends a new verification link on request and says so', async () => {
		signInUnverified();
		apiFetchWithSession.mockResolvedValue(new Response(undefined, { status: 202 }));
		await goToVerifyStep();

		await testPage.getByRole('button', { name: 'Send a new verification link' }).click();

		await expect.element(testPage.getByText(`We've sent a new verification link to ${session.email}.`)).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/verify-email/request', { method: 'POST' });
	});

	it('reports a refused request for a new link in the server’s words', async () => {
		signInUnverified();
		apiFetchWithSession.mockResolvedValue(jsonResponse('Too many requests. Try again later.', 429));
		await goToVerifyStep();

		await testPage.getByRole('button', { name: 'Send a new verification link' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('Too many requests');
	});
});

describe('TOTP enrollment -- step two, the QR code and secret', () => {
	it('opens an enrollment session and shows the QR code and secret as text', async () => {
		await goToSetupStep();

		await expect
			.element(testPage.getByAltText('QR code for setting up two-factor authentication in an authenticator app'))
			.toBeVisible();
		await expect.element(testPage.getByText(totpSecret.secretKey)).toBeVisible();
		expect(getSession).toHaveBeenCalled();
		expect(generateSecret).toHaveBeenCalled();
	});

	it('refuses an empty code without calling Identity Platform', async () => {
		await goToSetupStep();

		await testPage.getByRole('button', { name: 'Confirm and turn on' }).click();

		await expectFieldError(codeId, 'Enter the 6-digit code from your authenticator app');
		expect(enroll).not.toHaveBeenCalled();
	});

	it('shows a wrong code as a sign-in failure, not an app error', async () => {
		enroll.mockRejectedValue({ code: 'auth/invalid-verification-code' });
		await goToSetupStep();
		await testPage.getByLabelText('Authenticator app code').fill('000000');

		await testPage.getByRole('button', { name: 'Confirm and turn on' }).click();

		await expectFieldError(
			codeId,
			'The code is not correct. Enter the 6-digit code from your authenticator app.'
		);
	});

	it('force-refreshes the ID token before finishing enrollment', async () => {
		const getIdToken = vi.fn().mockResolvedValue('fresh-id-token');
		signInWithEmailAndPassword.mockResolvedValue({ user: { emailVerified: true, getIdToken } });
		globalFetch.mockResolvedValue(jsonResponse({ ok: true }));
		await goToSetupStep();

		await confirmCode();

		await vi.waitFor(() => expect(getIdToken).toHaveBeenCalledWith(true));
		expect(globalFetch).toHaveBeenCalledWith(
			'/api/staff/mfa',
			expect.objectContaining({
				method: 'POST',
				headers: { Authorization: 'Bearer fresh-id-token' }
			})
		);
	});

	it('signs out of the JS SDK and lands on / with no returnTo', async () => {
		globalFetch.mockResolvedValue(jsonResponse({ ok: true }));
		await goToSetupStep();

		await confirmCode();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/'));
		expect(signOut).toHaveBeenCalled();
	});

	it('lands on a same-origin returnTo once enrollment finishes', async () => {
		pageState.url = urlWith('/practices/practice-1');
		globalFetch.mockResolvedValue(jsonResponse({ ok: true }));
		await goToSetupStep();

		await confirmCode();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1'));
	});

	// An open-redirect guard: a protocol-relative target is not a path on
	// this app, so it is treated as absent rather than followed.
	it('ignores a returnTo that is not a same-origin path', async () => {
		pageState.url = urlWith('//evil.example.com');
		globalFetch.mockResolvedValue(jsonResponse({ ok: true }));
		await goToSetupStep();

		await confirmCode();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/'));
	});

	// SvelteKit 3's `goto` rejects a path no route answers (#1657). A stale
	// `returnTo` then lands on the root, not on a failure.
	it('lands on / when no route answers the returnTo', async () => {
		pageState.url = urlWith('/no-such-screen');
		goto.mockRejectedValueOnce(new Error('no route'));
		globalFetch.mockResolvedValue(jsonResponse({ ok: true }));
		await goToSetupStep();

		await confirmCode();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/'));
		expect(goto).toHaveBeenCalledWith('/no-such-screen');
	});

	// Decision 4: the post-enrollment token turned out not to carry the
	// claim yet. Fallback plumbing, not a form refusal.
	it('routes to the ordinary sign-in flow when the fresh token still shows no second factor', async () => {
		globalFetch.mockResolvedValue(jsonResponse('that sign-in does not show a second factor', 400));
		await goToSetupStep();

		await confirmCode();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/login'));
		expect(signOut).toHaveBeenCalled();
		expect(testPage.getByText('that sign-in does not show a second factor').elements()).toHaveLength(0);
	});

	it('shows an unexpected refusal as a service problem, and lets her retry in place', async () => {
		globalFetch.mockResolvedValue(jsonResponse('', 500));
		await goToSetupStep();

		await confirmCode();

		await expect
			.element(testPage.getByText('There is a problem with the service. Try again in a few minutes.'))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalledWith('/login');
	});
});

describe('TOTP enrollment -- leaving the JS SDK signed out (#167)', () => {
	it('signs out of the JS SDK when the screen is left mid-flow', async () => {
		const { unmount } = await render(Page, {});

		await unmount();

		expect(signOut).toHaveBeenCalled();
	});
});
