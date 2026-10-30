import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { workStateReportedOn } from '#lib/workStates.js';
import { jsonResponse as buildResponse } from '#lib/testResponse.js';
import { expectFieldError } from '#lib/components/molecules/LabeledField.testing.js';
import { gotoWithOutcome } from '#lib/outcome.js';
import { toApiResponder, toPageState, toSweptFixtures } from '../../routeFixture.js';
import Page from './+page.svelte';
import { resetAccountSession } from '../session.svelte.js';
import { fixture, session, soleOwnerSession } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiBaseURL: () => ''
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

function jsonResponse(body: unknown): Response {
	return buildResponse(body);
}

function refusal(status: number, message: string): Response {
	return buildResponse(message, status);
}

interface MockOptions {
	sessionResponse?: Response;
	resendResponse?: Response;
	resendThrows?: boolean;
}

function mockApi({
	sessionResponse = jsonResponse(session),
	resendResponse = new Response(undefined, { status: 202 }),
	resendThrows = false
}: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path === '/api/staff/verify-email/request') {
			if (resendThrows) return Promise.reject(new Error('The network dropped'));
			return Promise.resolve(resendResponse);
		}
		return Promise.resolve(sessionResponse);
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

const resendButton = () => testPage.getByRole('button', { name: 'Send a new verification link' });

async function setup(options: MockOptions = {}) {
	mockApi(options);
	await render(Page, {});
}

/*
 * #2044: the page is the facts about her, and each row says how it
 * changes. It asks no question itself, so there is no form here to
 * submit: the name and the work state each have a question page of
 * their own (name/, work-state/).
 */
describe('the account summary', () => {
	it('shows her name, where she works and since when, her email address and her second factor', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Your account' })).toBeVisible();
		await expect.element(testPage.getByText(`${session.firstName} ${session.lastName}`)).toBeVisible();
		await expect.element(testPage.getByText('New York')).toBeVisible();
		await expect
			.element(testPage.getByText(`Last confirmed ${workStateReportedOn(session.workStateReportedAt)}.`))
			.toBeVisible();
		await expect.element(testPage.getByText(session.email)).toBeVisible();
		await expect.element(testPage.getByText('Off', { exact: true })).toBeVisible();
	});

	it('asks nothing on the page itself: no field and no save button', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1 })).toBeVisible();
		expect(testPage.getByRole('textbox').elements()).toHaveLength(0);
		expect(testPage.getByRole('combobox').elements()).toHaveLength(0);
		expect(testPage.getByRole('button', { name: /^Save/ }).elements()).toHaveLength(0);
	});

	// Each Change names what it changes through SummaryList's hidden text,
	// so a list of the page's links does not read "Change, Change".
	it('opens a question page for the name and for where she works', async () => {
		await setup();

		const changes = testPage.getByRole('link', { name: 'Change' });
		await expect.element(changes.first()).toHaveAttribute('href', '/account/name');
		await expect.element(changes.first()).toHaveAccessibleDescription('name');
		await expect.element(changes.last()).toHaveAttribute('href', '/account/work-state');
		await expect.element(changes.last()).toHaveAccessibleDescription('where you work');
	});

	it('says when no work state has been given yet', async () => {
		await setup({ sessionResponse: jsonResponse({ ...session, workState: '', workStateReportedAt: '' }) });

		await expect.element(testPage.getByText('Not given yet')).toBeVisible();
		expect(testPage.getByText(/Last confirmed/).elements()).toHaveLength(0);
	});

	// The way back is one Back link at the top of the column (#2044), not a
	// heading at the foot of the shell listing every Practice.
	it('goes back to the Practice she was last in, by name', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Back to Riverside Doula Collective' }))
			.toHaveAttribute('href', '/practices/practice-1');
	});

	it('goes back to the Practice she was last in when she works at several', async () => {
		const memberships = [
			{ practiceId: 'practice-2', practiceName: 'Lakeshore Birth Partners', roles: ['doula'] },
			...session.memberships
		];
		await setup({ sessionResponse: jsonResponse({ ...session, memberships }) });

		await expect
			.element(testPage.getByRole('link', { name: 'Back to Riverside Doula Collective' }))
			.toHaveAttribute('href', '/practices/practice-1');
	});

	it('goes back to the chooser when she works at several and none was the last', async () => {
		const memberships = [
			{ practiceId: 'practice-2', practiceName: 'Lakeshore Birth Partners', roles: ['doula'] },
			...session.memberships
		];
		await setup({ sessionResponse: jsonResponse({ ...session, memberships, lastPracticeId: undefined }) });

		await expect
			.element(testPage.getByRole('link', { name: 'Back to your practices' }))
			.toHaveAttribute('href', '/');
	});

	it('goes back to her one Practice when none is recorded as the last', async () => {
		await setup({ sessionResponse: jsonResponse({ ...session, lastPracticeId: undefined }) });

		await expect
			.element(testPage.getByRole('link', { name: 'Back to Riverside Doula Collective' }))
			.toBeVisible();
	});

	// #1526: the founder holds no Membership, and a way back to nowhere is
	// no way back.
	it('offers no way back to a person who belongs to no Practice', async () => {
		await setup({ sessionResponse: jsonResponse({ ...session, memberships: [], lastPracticeId: undefined }) });

		await expect.element(testPage.getByRole('heading', { level: 1 })).toBeVisible();
		expect(testPage.getByRole('link', { name: /^Back/ }).elements()).toHaveLength(0);
	});

	// #613's re-request AC: a signed-in Staff member can ask for a fresh
	// verification link without knowing whether her address is already
	// verified. The answer is under the row it was asked from.
	it('sends a fresh verification link on request', async () => {
		await setup();

		await resendButton().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/verify-email/request', {
			method: 'POST'
		});
		await expect
			.element(testPage.getByRole('status').filter({ hasText: "We've sent a new verification link" }))
			.toHaveTextContent("We've sent a new verification link to your email address.");
	});

	it("shows the server's own words when a resend is refused", async () => {
		await setup({ resendResponse: refusal(429, 'too many requests -- try again later') });

		await resendButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('too many requests -- try again later');
	});

	it('owns a resend that never reached the server at all', async () => {
		await setup({ resendThrows: true });

		await resendButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
	});
});

// #1710: a question page saves and comes back here, and this page says
// what it did.
describe('the status message after a change (#2044)', () => {
	it('says what the save did, as a status before the title, and takes focus once the account loads', async () => {
		goto.mockImplementationOnce(async () => {
			await setup();
		});

		await gotoWithOutcome('/account', 'Your name is Mary Anne Smith.');

		const message = testPage.getByRole('status').filter({ hasText: 'Your name is Mary Anne Smith.' });
		await expect.element(message).toBeVisible();
		await expect.element(message).toHaveFocus();
	});

	it('says nothing on a visit no save sent', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1 })).toBeVisible();
		expect(testPage.getByText(/^Your name is/).elements()).toHaveLength(0);
	});
});

describe('when the account screen cannot do its job', () => {
	// A verified identity with no staff row behind it. Signed in, but
	// nobody here yet -- so there is nothing to change and no row to offer.
	it('says so, and offers nothing to change, when there is no Staff account', async () => {
		await setup({ sessionResponse: refusal(404, 'no matching staff account') });

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('no matching staff account');
		expect(testPage.getByRole('link', { name: 'Change' }).elements()).toHaveLength(0);
	});
});

/*
 * #606: whether a second factor is enrolled, and the two ways off this
 * screen -- voluntary enrollment when it is off, removal when it is on.
 * `secondFactor` is read as the session-carried fact it is (see
 * SessionInfo's own doc comment), so these tests set it on the session
 * response rather than modeling a fresh enrollment check.
 */
describe('two-factor authentication status (#606)', () => {
	it('shows it is on and offers to remove it', async () => {
		await setup({ sessionResponse: jsonResponse({ ...session, secondFactor: true }) });

		await expect.element(testPage.getByText('On', { exact: true })).toBeVisible();
		const remove = testPage.getByRole('button', { name: 'Remove' });
		await expect.element(remove).toHaveAccessibleDescription('two-factor authentication');
		await expect.element(remove).toHaveAttribute('aria-expanded', 'false');
	});

	it('offers to set it up voluntarily when it is off, returning here afterward', async () => {
		await setup({ sessionResponse: jsonResponse({ ...session, secondFactor: false }) });

		await expect.element(testPage.getByText('Off', { exact: true })).toBeVisible();
		const setUp = testPage.getByRole('link', { name: 'Set up' });
		await expect.element(setUp).toHaveAttribute('href', '/mfa/enroll?returnTo=%2Faccount');
		await expect.element(setUp).toHaveAccessibleDescription('two-factor authentication');
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
		await testPage.getByRole('button', { name: 'Remove' }).last().click();
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

		await expectFieldError('account-mfa-password', 'Enter your password');
		expect(signInWithEmailAndPassword).not.toHaveBeenCalled();
	});

	it('shows a wrong password as a step-up refusal', async () => {
		signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/wrong-password' });
		await setup();

		await testPage.getByLabelText('Password').fill('wrong');
		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expectFieldError('account-mfa-password', 'Password is not correct');
	});

	it('lets her cancel out of the step-up back to the row', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(testPage.getByText('On', { exact: true })).toBeVisible();
		expect(testPage.getByLabelText('Password').elements()).toHaveLength(0);
	});

	// The row's own Remove closes the step-up it opened, the way a Change
	// on a record page closes its in-place form.
	it('closes the step-up from the row that opened it', async () => {
		await setup();

		const rowRemove = testPage.getByRole('button', { name: 'Remove' }).first();
		await expect.element(rowRemove).toHaveAttribute('aria-expanded', 'true');
		await rowRemove.click();

		expect(testPage.getByLabelText('Password').elements()).toHaveLength(0);
	});

	it('challenges for a code, since the identity already holds one enrolled', async () => {
		await reachCodeStep();

		expect(testPage.getByLabelText('Password').elements()).toHaveLength(0);
	});

	it('refuses an empty code without resolving sign-in', async () => {
		await reachCodeStep();

		await testPage.getByRole('button', { name: 'Remove' }).last().click();

		await expectFieldError('account-mfa-code', 'Enter the 6-digit code from your authenticator app');
		expect(resolver.resolveSignIn).not.toHaveBeenCalled();
	});

	it('shows a wrong or expired code as a refusal she can retry', async () => {
		resolver.resolveSignIn.mockRejectedValue({ code: 'auth/invalid-verification-code' });
		await reachCodeStep();

		await testPage.getByLabelText('Authenticator app code').fill('000000');
		await testPage.getByRole('button', { name: 'Remove' }).last().click();

		await expectFieldError(
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
		await testPage.getByRole('button', { name: 'Remove' }).last().click();

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

/*
 * #892: deleting her own login. Its own describe rather than an option on
 * mockApi -- every test here needs the DELETE routed on its own, and the
 * refusal case needs the session read to keep working while the delete
 * fails, which one shared `saveResponse` cannot express.
 */
function mockDeleteLogin(deleteResponse: Response | Error) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path === '/api/staff/session') return Promise.resolve(jsonResponse(session));
		if (path === '/api/staff/account') {
			return deleteResponse instanceof Error
				? Promise.reject(deleteResponse)
				: Promise.resolve(deleteResponse);
		}
		return Promise.resolve(jsonResponse({}));
	});
}

// Both the section's own button and the ConfirmDialog's confirm button
// carry this label on purpose -- ConfirmDialog names the act rather than
// saying "OK" -- so `.first()` is the one that opens and `.last()` the
// one that commits.
const deleteButton = () => testPage.getByRole('button', { name: 'Delete your login' });

async function setupDeleteLogin(deleteResponse: Response | Error) {
	mockDeleteLogin(deleteResponse);
	await render(Page, {});
}

describe('deleting your own login', () => {
	it('names what deletion destroys and what it keeps, before she presses anything', async () => {
		await setupDeleteLogin(new Response(undefined, { status: 204 }));

		await expect.element(testPage.getByText(/ends your access to DoulaCloud everywhere/)).toBeVisible();
		await expect.element(testPage.getByText(/membership of every Practice you work at/).first()).toBeVisible();
		await expect.element(testPage.getByText(/Everything you did stays with the Practices/).first()).toBeVisible();
	});

	it('sends the delete only after the confirmation, and lands her on the sign-in screen', async () => {
		await setupDeleteLogin(new Response(undefined, { status: 204 }));

		await deleteButton().first().click();
		// The dialog's own confirm button carries the same label, so the act
		// is the second one -- pressing the first has sent nothing yet.
		expect(apiFetchWithSession).not.toHaveBeenCalledWith('/api/staff/account', expect.anything());

		await deleteButton().last().click();
		await vi.waitFor(() => {
			expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/account', {
				method: 'DELETE',
				headers: { 'X-Confirmed': 'true' }
			});
		});
		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		// #167's shared-device rule: no client-side Identity Platform session
		// is left behind in this tab.
		expect(signOut).toHaveBeenCalled();
	});

	it('shows the last-Owner refusal with the practices in the way named, and stays put', async () => {
		await setupDeleteLogin(
			refusal(
				409,
				'cannot delete your login while you are the only Owner of Riverside Doula Collective: hand ownership to someone else, or delete the practice first'
			)
		);

		await deleteButton().first().click();
		await deleteButton().last().click();

		// #804: the dialog stays open on a rejected onConfirm and renders
		// this inside itself, so the named Practices are in front of her
		// rather than behind its backdrop.
		const dialog = testPage.getByRole('dialog');
		await expect.element(dialog).toBeVisible();
		await expect.element(dialog.getByText(/only Owner of Riverside Doula Collective/)).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('shows a service problem when the request never lands', async () => {
		await setupDeleteLogin(new Error('The network dropped'));

		await deleteButton().first().click();
		await deleteButton().last().click();

		const dialog = testPage.getByRole('dialog');
		await expect.element(dialog).toBeVisible();
		await expect.element(dialog.getByText('The network dropped')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});

/*
 * #615's saved recovery codes, offered here and nowhere else (#694).
 *
 * The thing worth stating once, because the screen's whole shape follows
 * from it: there is no "show me the set I already have". Every set is
 * minted with its plaintext discarded except the one this button asks
 * for, so being shown them and rotating them are one act -- which is why
 * a confirmation stands between the offer and the codes.
 */
const CODES = ['AAAA1111BBBB2222', 'CCCC3333DDDD4444'];

// Press through the offer and its confirmation to the codes themselves.
async function revealSavedCodes() {
	await testPage.getByRole('button', { name: 'Show my recovery codes' }).click();
	await testPage.getByRole('button', { name: 'Show a new set' }).click();
}

describe('recovery codes for a sole Owner (#615)', () => {

	interface CodesSetupOptions {
		soleOwner?: boolean;
		rotateResponse?: Response;
	}

	/*
	 * The session is the fixture's own sole-Owner variant (#596), not a
	 * second one written here: that variant is what the continuum sweep
	 * mounts, so this spec and the sweep describe one screen. `soleOwner:
	 * false` is the departure, written as a spread.
	 */
	async function setup({ soleOwner = true, rotateResponse = jsonResponse({ codes: CODES }) }: CodesSetupOptions = {}) {
		apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
			if (path === '/api/staff/mfa-recovery/saved-codes/rotate' && init?.method === 'POST') {
				return Promise.resolve(rotateResponse);
			}
			return Promise.resolve(jsonResponse({ ...soleOwnerSession, soleOwner }));
		});
		await render(Page, {});
	}

	// The AC's last clause, and the one a screen gets wrong by omission:
	// somebody who holds no saved codes must not be shown a section
	// implying she does. The two-factor line is asserted on its own
	// distinctive clause rather than on "Turned on.", which is a prefix of
	// nothing here but is one keystroke away from matching "Not turned
	// on." if this session's `secondFactor` ever drifts.
	it('offers nothing at all to a Staff member who is not a sole Owner', async () => {
		await setup({ soleOwner: false });

		await expect.element(testPage.getByText('On', { exact: true })).toBeVisible();
		expect(testPage.getByRole('button', { name: 'Show my recovery codes' }).elements()).toHaveLength(0);
		expect(testPage.getByText('Recovery codes').elements()).toHaveLength(0);
	});

	// #1531: a sole Owner with no second factor has nothing for a code to
	// recover, and the BFF mints none for her.
	it('offers nothing to a sole Owner with no second factor', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...soleOwnerSession, secondFactor: false }));
		await render(Page, {});

		await expect.element(testPage.getByRole('heading', { name: 'Your account' })).toBeVisible();
		expect(testPage.getByRole('button', { name: 'Show my recovery codes' }).elements()).toHaveLength(0);
		expect(testPage.getByText('Recovery codes').elements()).toHaveLength(0);
	});

	it('says what pressing it costs before it mints anything', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Show my recovery codes' }).click();

		await expect
			.element(testPage.getByText('Showing a new set replaces the one you have.', { exact: false }))
			.toBeVisible();
		expect(apiFetchWithSession).not.toHaveBeenCalledWith(
			'/api/staff/mfa-recovery/saved-codes/rotate',
			expect.anything()
		);
	});

	it('shows every code once, with the warning that this is the only time', async () => {
		await setup();

		await revealSavedCodes();

		for (const code of CODES) {
			await expect.element(testPage.getByText(code)).toBeVisible();
		}
		await expect
			.element(testPage.getByText('Write these down now.', { exact: false }))
			.toBeVisible();
	});

	// The half that works on a phone, where writing ten opaque strings
	// down by hand is not a real option.
	it('offers them as a file to keep', async () => {
		await setup();

		await revealSavedCodes();

		await expect
			.element(testPage.getByRole('button', { name: 'Download these codes' }))
			.toBeVisible();
	});

	it("shows the BFF's own refusal when she is no longer a sole Owner", async () => {
		await setup({
			rotateResponse: refusal(403, "saved recovery codes are only issued to a practice's sole owner")
		});

		await revealSavedCodes();

		await expect
			.element(testPage.getByText("saved recovery codes are only issued to a practice's sole owner"))
			.toBeVisible();
	});
});

/*
 * #2050: the sweep and the drag surface mount the base screen, then the
 * second-factor variant, in one module, with no spec `beforeEach` between
 * them. The memoized session read kept the first mount's answer, so the
 * variant drew the base screen; the fixture's `beforeMount` is what makes
 * each mount ask its own `respond`.
 */
describe('the account fixture, as the sweep mounts it', () => {
	it('draws the Recovery codes section on the second-factor variant mounted after the base', async () => {
		const [base, withSecondFactor] = toSweptFixtures(fixture);
		for (const subject of [base!, withSecondFactor!]) {
			subject.beforeMount?.();
			apiFetchWithSession.mockImplementation(toApiResponder(subject));
			const { unmount } = await render(Page, {});
			await expect.element(testPage.getByRole('heading', { level: 1, name: subject.readyText })).toBeVisible();
			if (subject === base) unmount();
		}

		await expect.element(testPage.getByText('Recovery codes', { exact: true })).toBeVisible();
	});
});
