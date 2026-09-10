import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../../routeFixture.js';
import { fixture, roster, session } from './page.fixture.js';

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params this spec installs and the params the continuum sweep installs
 * are one description.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetch = vi.hoisted(() => vi.fn());
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetch,
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

/*
 * The Identity Platform SDK, faked the same way the account screen's and
 * the login screen's own specs fake it -- it needs a live project or
 * emulator this suite never runs against.
 */
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

const member = roster.members[0];
const resolver = { hints: [{ uid: 'enrollment-1' }], resolveSignIn: vi.fn() };

interface SetupOptions {
	roles?: string[];
	rosterResponse?: Response;
	sessionResponse?: Response;
	vouchResponse?: Response;
}

/*
 * Returns the vouch POSTs the screen sent, because "what did it ask the
 * endpoint for, and with which credentials?" is the whole behavior this
 * screen produces -- the fresh Bearer token and the confirmation header
 * are the two things the endpoint refuses without.
 */
async function setup({
	roles = ['owner'],
	rosterResponse = jsonResponse(roster),
	sessionResponse = jsonResponse(session),
	vouchResponse = jsonResponse(undefined, 204)
}: SetupOptions = {}) {
	pageState.data = {
		session: {
			practiceId: 'practice-1',
			staffId: 'staff-1',
			practiceName: 'Riverside Doula Collective',
			roles,
			isContractor: false
		}
	};
	apiFetchWithSession.mockImplementation((path: string) =>
		Promise.resolve(path.startsWith('/api/staff/session') ? sessionResponse : rosterResponse)
	);
	const vouches: { path: string; init?: RequestInit }[] = [];
	apiFetch.mockImplementation((path: string, init?: RequestInit) => {
		vouches.push({ path, init });
		return Promise.resolve(vouchResponse);
	});
	await render(Page, {});
	return { vouches };
}

/*
 * The path every test that actually sends a code walks: read the
 * consequence, press through it, then satisfy the two-step re-auth
 * Identity Platform raises for an Owner (who always holds a second
 * factor, #606).
 */
async function reachCodeStep() {
	signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/multi-factor-auth-required' });
	getMultiFactorResolver.mockReturnValue(resolver);
	await testPage.getByRole('button', { name: 'Send a recovery code' }).click();
	await testPage.getByLabelText('Password').fill('correct horse');
	await testPage.getByRole('button', { name: 'Continue' }).click();
	await expect.element(testPage.getByLabelText('Authenticator app code')).toBeVisible();
}

beforeEach(() => {
	for (const mock of [
		apiFetch,
		apiFetchWithSession,
		signInWithEmailAndPassword,
		signOut,
		getMultiFactorResolver,
		assertionForSignIn,
		resolver.resolveSignIn
	])
		mock.mockReset();
	signOut.mockResolvedValue(undefined);
});

describe('vouching for a locked-out Staff member', () => {
	it("names the Staff member and says the code arrives at the Owner's own address, before anything is sent", async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: `Help ${member.name} sign in again` }))
			.toBeVisible();
		await expect.element(testPage.getByText(session.email, { exact: false })).toBeVisible();
		await expect
			.element(testPage.getByText(`It does not go to ${member.name}`, { exact: false }))
			.toBeVisible();
	});

	// #615's AC, and the misreading the whole screen exists to prevent: an
	// Owner who thinks the mail went to the doula concludes it was lost.
	it('repeats where the code lands on the step that actually sends it', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Send a recovery code' }).click();

		await expect
			.element(testPage.getByText(`The code comes to you, at ${session.email}`, { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByText('will not receive it', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByLabelText('Password')).toBeVisible();
	});

	it('offers an Admin nothing to press, and never asks the Owner-only roster for the member', async () => {
		await setup({ roles: ['admin'] });

		await expect
			.element(testPage.getByText('Only a practice owner can send a recovery code.', { exact: false }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Send a recovery code' }))
			.not.toBeInTheDocument();
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('says so when the staff id names nobody on this roster', async () => {
		await setup({ rosterResponse: jsonResponse({ members: [], invitations: { items: [], hasMore: false } }) });

		await expect
			.element(testPage.getByText('That Staff member is not on this practice roster.'))
			.toBeVisible();
	});

	it('sends the fresh step-up token and the confirmation header, then says what to do with the code', async () => {
		assertionForSignIn.mockReturnValue({ assertion: true });
		resolver.resolveSignIn.mockResolvedValue({ user: { getIdToken: async () => 'fresh-id-token' } });
		const { vouches } = await setup();
		await reachCodeStep();

		await testPage.getByLabelText('Authenticator app code').fill('123456');
		await testPage.getByRole('button', { name: 'Send the code' }).click();

		await vi.waitFor(() => expect(vouches).toHaveLength(1));
		expect(vouches[0].path).toBe(
			'/api/practices/practice-1/staff/staff-2/mfa-recovery/vouch'
		);
		expect(vouches[0].init).toEqual({
			method: 'POST',
			headers: { Authorization: 'Bearer fresh-id-token', 'X-Confirmed': 'true' }
		});
		await expect
			.element(testPage.getByText(`We've sent a recovery code to ${session.email}.`))
			.toBeVisible();
	});

	/*
	 * The refusal that matters most here. A step-up token past its
	 * five-minute window comes back 401, and the fix is to start the
	 * step-up over -- not to be signed out of a session that is perfectly
	 * alive, which is what `apiFetchWithSession` would have done with it.
	 */
	it('shows a stale step-up as its own refusal and puts her back on the password step', async () => {
		assertionForSignIn.mockReturnValue({ assertion: true });
		resolver.resolveSignIn.mockResolvedValue({ user: { getIdToken: async () => 'stale' } });
		await setup({
			vouchResponse: jsonResponse({ message: 'this action requires a fresh sign-in' }, 401)
		});
		await reachCodeStep();

		await testPage.getByLabelText('Authenticator app code').fill('123456');
		await testPage.getByRole('button', { name: 'Send the code' }).click();

		await expect
			.element(testPage.getByText('this action requires a fresh sign-in'))
			.toBeVisible();
		await expect.element(testPage.getByLabelText('Password')).toBeVisible();
	});

	it('lets her back out of the step-up to the explanation she started from', async () => {
		await setup();
		await testPage.getByRole('button', { name: 'Send a recovery code' }).click();

		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect
			.element(testPage.getByText(`It does not go to ${member.name}`, { exact: false }))
			.toBeVisible();
		expect(testPage.getByLabelText('Password').elements()).toHaveLength(0);
	});
});
