import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * TOTP enrolment (#606): step one re-authenticates and opens an
 * enrolment session, step two shows the QR code/secret and confirms the
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
vi.mock('#lib/api.js', () => ({
	apiBaseURL: () => '',
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
		user: { getIdToken: vi.fn().mockResolvedValue('id-token') }
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

describe('TOTP enrolment -- step one, re-authenticating', () => {
	it('asks for the password again rather than assuming a live sign-in', async () => {
		await render(Page, {});

		await expect.element(testPage.getByLabelText('Password')).toBeVisible();
	});

	it('sends a visitor with no session at all to the login screen', async () => {
		apiFetch.mockResolvedValue(jsonResponse('no matching staff session', 404));

		await render(Page, {});

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/login'));
	});

	it('refuses an empty submission without calling Identity Platform', async () => {
		await render(Page, {});

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await fieldError(passwordId, 'Enter your password');
		expect(signInWithEmailAndPassword).not.toHaveBeenCalled();
	});

	it('reads the email from the session probe, not from a form field', async () => {
		await render(Page, {});
		await testPage.getByLabelText('Password').fill('correct horse');

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await vi.waitFor(() =>
			expect(signInWithEmailAndPassword).toHaveBeenCalledWith({}, session.email, 'correct horse')
		);
	});

	it('shows a wrong password as a step-up refusal', async () => {
		signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/wrong-password' });
		await render(Page, {});
		await testPage.getByLabelText('Password').fill('wrong');

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await fieldError(passwordId, 'Password is not correct');
	});
});

describe('TOTP enrolment -- step two, the QR code and secret', () => {
	it('opens an enrolment session and shows the QR code and secret as text', async () => {
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

		await fieldError(codeId, 'Enter the 6-digit code from your authenticator app');
		expect(enroll).not.toHaveBeenCalled();
	});

	it('shows a wrong code as a sign-in failure, not an app error', async () => {
		enroll.mockRejectedValue({ code: 'auth/invalid-verification-code' });
		await goToSetupStep();
		await testPage.getByLabelText('Authenticator app code').fill('000000');

		await testPage.getByRole('button', { name: 'Confirm and turn on' }).click();

		await fieldError(
			codeId,
			'The code is not correct. Enter the 6-digit code from your authenticator app.'
		);
	});

	it('force-refreshes the ID token before finishing enrolment', async () => {
		const getIdToken = vi.fn().mockResolvedValue('fresh-id-token');
		signInWithEmailAndPassword.mockResolvedValue({ user: { getIdToken } });
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

	it('lands on a same-origin returnTo once enrolment finishes', async () => {
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

	// Decision 4: the post-enrolment token turned out not to carry the
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

describe('TOTP enrolment -- leaving the JS SDK signed out (#167)', () => {
	it('signs out of the JS SDK when the screen is left mid-flow', async () => {
		const { unmount } = await render(Page, {});

		await unmount();

		expect(signOut).toHaveBeenCalled();
	});
});
