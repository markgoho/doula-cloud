import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { workStateReportedOn } from '#lib/workStates.js';
import { jsonResponse as buildResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { resetAccountSession } from './session.svelte.js';
import { session } from './page.fixture.js';

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiBaseURL: () => '',
	// The real one reads a plain-text body or a {code, message} JSON body
	// without the caller knowing which; the screen's job is only to show
	// whatever it says, so the mock is the plain-text half.
	apiErrorMessage: (response: Response) => response.text()
}));

/*
 * #606's removal flow, faking the Firebase SDK surface the same way
 * `mfa/enroll`'s and the login screen's own specs do -- it needs a live
 * project or emulator this suite never runs against.
 */
const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

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

const globalFetch = vi.hoisted(() => vi.fn());

const SAVED_AT = '2027-03-14T10:00:00Z';

function jsonResponse(body: unknown): Response {
	return buildResponse(body);
}

function refusal(status: number, message: string): Response {
	return buildResponse(message, status);
}

interface MockOptions {
	sessionResponse?: Response;
	saveResponse?: Response;
	saveThrows?: boolean;
	resendResponse?: Response;
	resendThrows?: boolean;
}

function mockApi({
	sessionResponse = jsonResponse(session),
	saveResponse = jsonResponse({ workState: 'NJ', workStateReportedAt: SAVED_AT }),
	saveThrows = false,
	resendResponse = new Response(undefined, { status: 202 }),
	resendThrows = false
}: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path === '/api/staff/session') return Promise.resolve(sessionResponse);
		if (path === '/api/staff/verify-email/request') {
			if (resendThrows) return Promise.reject(new Error('The network dropped'));
			return Promise.resolve(resendResponse);
		}
		if (saveThrows) return Promise.reject(new Error('The network dropped'));
		return Promise.resolve(saveResponse);
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	// loadAccountSession() memoizes its in-flight request at module scope
	// (#474), so a fresh test needs a clean slate rather than replaying the
	// previous test's fetch.
	resetAccountSession();

	for (const mock of [goto, signInWithEmailAndPassword, signOut, getMultiFactorResolver, assertionForSignIn, globalFetch])
		mock.mockReset();
	vi.stubGlobal('fetch', globalFetch);
	signOut.mockResolvedValue(undefined);
});

afterEach(() => {
	vi.unstubAllGlobals();
});

const saveButton = () => testPage.getByRole('button', { name: 'Save work state' });
const stateSelect = () => testPage.getByRole('combobox', { name: 'Which state do you work from?' });

/*
 * A field-targeted refusal renders beside its own control -- LabeledField's
 * own `<p role="alert">`, `id`d off the field's own id -- the same helper
 * `mfa/enroll`'s and the login screen's own specs use, since `getByText`
 * cannot single it out from an identical error summary entry.
 */
async function fieldError(id: string, message: string) {
	await vi.waitFor(() => {
		expect(document.querySelector(`#${id}-error`)?.textContent).toBe(message);
	});
}

describe('the account screen', () => {
	it('shows the work state she has already asserted, and the day she asserted it', async () => {
		mockApi();
		await render(Page, {});

		await expect.element(testPage.getByRole('heading', { name: 'Your account' })).toBeVisible();
		await expect.element(stateSelect()).toHaveValue('New York');
		await expect
			.element(testPage.getByText(`Last confirmed ${workStateReportedOn(session.workStateReportedAt)}.`))
			.toBeVisible();
	});

	// The consequence, before the choice rather than in a dialog after it
	// -- and the answer to the question it invites, which is whether a
	// correction reaches backwards. It does not (#420).
	it('states the sales tax consequence and that past purchases are not re-priced', async () => {
		mockApi();
		await render(Page, {});

		await expect
			.element(testPage.getByText(/sets how much sales tax your practice pays/))
			.toBeVisible();
		await expect.element(testPage.getByText(/are not re-priced/)).toBeVisible();
	});

	it('sends the USPS code for the state she picked, with no staff id of its own', async () => {
		mockApi();
		await render(Page, {});

		await stateSelect().selectOptions('New Jersey');
		await saveButton().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/work-state', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ workState: 'NJ' })
		});
	});

	it('confirms the save and moves the last-confirmed day to the one the server returned', async () => {
		mockApi();
		await render(Page, {});

		await stateSelect().selectOptions('New Jersey');
		await saveButton().click();

		await expect.element(testPage.getByRole('status')).toHaveTextContent(
			'Saved. You work from New Jersey.'
		);
		await expect
			.element(testPage.getByText(`Last confirmed ${workStateReportedOn(SAVED_AT)}.`))
			.toBeVisible();
	});

	// The re-assertion case, which is the reason the button is never
	// disabled on an unchanged value: "yes, still New York, as of today"
	// is the only staleness signal the design has.
	it('sends the same state again rather than treating it as a no-op', async () => {
		mockApi({ saveResponse: jsonResponse({ workState: 'NY', workStateReportedAt: SAVED_AT }) });
		await render(Page, {});

		await expect.element(saveButton()).toBeEnabled();
		await saveButton().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			'/api/staff/work-state',
			expect.objectContaining({ body: JSON.stringify({ workState: 'NY' }) })
		);
		await expect
			.element(testPage.getByText(`Last confirmed ${workStateReportedOn(SAVED_AT)}.`))
			.toBeVisible();
	});

	// The practices nav is layout chrome, not this page's business -- see
	// account-layout.svelte.spec.ts (#474).

	// #613's re-request AC: a signed-in Staff member can ask for a fresh
	// verification link without knowing whether her address is already
	// verified.
	it('sends a fresh verification link on request', async () => {
		mockApi();
		await render(Page, {});

		await testPage.getByRole('button', { name: 'Send a new verification link' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/verify-email/request', {
			method: 'POST'
		});
		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent("We've sent a new verification link to your email address.");
	});

	it("shows the server's own words when a resend is refused", async () => {
		mockApi({ resendResponse: refusal(429, 'too many requests -- try again later') });
		await render(Page, {});

		await testPage.getByRole('button', { name: 'Send a new verification link' }).click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('too many requests -- try again later');
	});

	it('owns a resend that never reached the server at all', async () => {
		mockApi({ resendThrows: true });
		await render(Page, {});

		await testPage.getByRole('button', { name: 'Send a new verification link' }).click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
	});
});

describe('when the account screen cannot do its job', () => {
	// A verified identity with no staff row behind it. Signed in, but
	// nobody here yet -- so there is nothing to edit and no form to offer.
	it('says so, and offers no form, when there is no Staff account', async () => {
		mockApi({ sessionResponse: refusal(404, 'no matching staff account') });
		await render(Page, {});

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('no matching staff account');
		expect(saveButton().elements()).toHaveLength(0);
	});

	it("shows the server's own words when a save is refused", async () => {
		mockApi({
			saveResponse: refusal(
				400,
				'workState is required, and must be a two-letter US state abbreviation'
			)
		});
		await render(Page, {});

		await saveButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('workState is required, and must be a two-letter US state abbreviation');
	});

	/*
	 * The thrown message is the network's. Since #467 this screen owns the
	 * failure in words she can act on rather than repeating an exception,
	 * and reports it in the error summary above the title.
	 */
	it('owns a save that never reached the server at all', async () => {
		mockApi({ saveThrows: true });
		await render(Page, {});

		await saveButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
		await expect.element(testPage.getByText('The network dropped')).not.toBeInTheDocument();
	});
});

/*
 * #606: whether a second factor is enrolled, and the two ways off this
 * screen -- voluntary enrolment when it is off, removal when it is on.
 * `secondFactor` is read as the session-carried fact it is (see
 * SessionInfo's own doc comment), so these tests set it on the session
 * response rather than modeling a fresh enrolment check.
 */
describe('two-factor authentication status (#606)', () => {
	it('shows it is turned on and offers to remove it', async () => {
		mockApi({ sessionResponse: jsonResponse({ ...session, secondFactor: true }) });
		await render(Page, {});

		await expect.element(testPage.getByText('Turned on.')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Remove' })).toBeVisible();
	});

	it('offers to set it up voluntarily when it is off, returning here afterward', async () => {
		mockApi({ sessionResponse: jsonResponse({ ...session, secondFactor: false }) });
		await render(Page, {});

		await expect.element(testPage.getByText('Not turned on.')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Set up two-factor authentication' }))
			.toHaveAttribute('href', '/mfa/enroll?returnTo=%2Faccount');
	});
});

/*
 * #606's own AC: removing a second factor ends every live session for the
 * identity. `DELETE /api/staff/mfa` reads both a fresh Bearer ID token
 * (api/internal/staffauth/reauth.go's RequireRecentAuth) and the
 * `__session` cookie already on the request (authn.Begin) -- so the step-up
 * always re-authenticates, and Identity Platform itself challenges the
 * second factor on that reauth, since one is already enrolled.
 */
describe('removing a second factor (#606)', () => {
	const resolver = { hints: [{ uid: 'enrollment-1' }], resolveSignIn: vi.fn() };

	interface SetupOptions {
		deleteResponse?: Response;
	}

	async function setup({ deleteResponse = new Response(undefined, { status: 204 }) }: SetupOptions = {}) {
		mockApi({ sessionResponse: jsonResponse({ ...session, secondFactor: true }) });
		globalFetch.mockResolvedValue(deleteResponse);
		await render(Page, {});
		await testPage.getByRole('button', { name: 'Remove' }).click();
		await expect.element(testPage.getByLabelText('Password')).toBeVisible();
	}

	async function reachCodeStep() {
		signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/multi-factor-auth-required' });
		getMultiFactorResolver.mockReturnValue(resolver);
		await setup();
		await testPage.getByLabelText('Password').fill('correct horse');
		await testPage.getByRole('button', { name: 'Continue' }).click();
		await expect.element(testPage.getByLabelText('Authenticator app code')).toBeVisible();
	}

	beforeEach(() => {
		resolver.resolveSignIn.mockReset();
	});

	it('refuses an empty password without calling Identity Platform', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await fieldError('account-mfa-password', 'Enter your password');
		expect(signInWithEmailAndPassword).not.toHaveBeenCalled();
	});

	it('shows a wrong password as a step-up refusal', async () => {
		signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/wrong-password' });
		await setup();

		await testPage.getByLabelText('Password').fill('wrong');
		await testPage.getByRole('button', { name: 'Continue' }).click();

		await fieldError('account-mfa-password', 'Password is not correct');
	});

	it('lets her cancel out of the step-up back to the status line', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(testPage.getByText('Turned on.')).toBeVisible();
		expect(testPage.getByLabelText('Password').elements()).toHaveLength(0);
	});

	it('challenges for a code, since the identity already holds one enrolled', async () => {
		await reachCodeStep();

		expect(testPage.getByLabelText('Password').elements()).toHaveLength(0);
	});

	it('refuses an empty code without resolving sign-in', async () => {
		await reachCodeStep();

		await testPage.getByRole('button', { name: 'Remove' }).click();

		await fieldError('account-mfa-code', 'Enter the 6-digit code from your authenticator app');
		expect(resolver.resolveSignIn).not.toHaveBeenCalled();
	});

	it('shows a wrong or expired code as a refusal she can retry', async () => {
		resolver.resolveSignIn.mockRejectedValue({ code: 'auth/invalid-verification-code' });
		await reachCodeStep();

		await testPage.getByLabelText('Authenticator app code').fill('000000');
		await testPage.getByRole('button', { name: 'Remove' }).click();

		await fieldError(
			'account-mfa-code',
			'The code is not correct. Enter the 6-digit code from your authenticator app.'
		);
	});

	it('sends the fresh ID token as a Bearer credential alongside the session cookie, and ends every session on success', async () => {
		assertionForSignIn.mockReturnValue({ assertion: true });
		resolver.resolveSignIn.mockResolvedValue({
			user: { getIdToken: async () => 'fresh-id-token' }
		});
		await reachCodeStep();

		await testPage.getByLabelText('Authenticator app code').fill('123456');
		await testPage.getByRole('button', { name: 'Remove' }).click();

		expect(assertionForSignIn).toHaveBeenCalledWith('enrollment-1', '123456');
		await vi.waitFor(() =>
			expect(globalFetch).toHaveBeenCalledWith('/api/staff/mfa', {
				method: 'DELETE',
				credentials: 'include',
				headers: { Authorization: 'Bearer fresh-id-token' }
			})
		);
		expect(signOut).toHaveBeenCalled();
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/login?sessionEnded=true'));
	});

	// Identity Platform's reauth resolves directly whenever it does not
	// challenge for the second factor -- the general shape `handleMfaCodeSubmit`
	// also uses, exercised here since it is the branch that can reach
	// `finishMfaRemoval` without a code step first.
	it('shows a DELETE refusal as a service failure and sends her back to the step-up', async () => {
		signInWithEmailAndPassword.mockResolvedValue({
			user: { getIdToken: async () => 'fresh-id-token' }
		});
		await setup({ deleteResponse: refusal(500, '') });

		await testPage.getByLabelText('Password').fill('correct horse');
		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
		await expect.element(testPage.getByLabelText('Password')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});
