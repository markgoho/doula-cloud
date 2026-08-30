import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { workStateReportedOn } from '#lib/workStates.js';
import { jsonResponse as buildResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';

// Mutable rather than a fixed literal: the screen refuses outright when
// the URL carries no invite token, and that branch needs a test that can
// take the token away.
const pageState = vi.hoisted(() => ({
	url: new URL('https://test.local/accept-invite?token=invite-1')
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const createUserWithEmailAndPassword = vi.hoisted(() => vi.fn());
const signInWithEmailAndPassword = vi.hoisted(() => vi.fn());
const signOut = vi.hoisted(() => vi.fn());
vi.mock('firebase/auth', () => ({
	createUserWithEmailAndPassword,
	signInWithEmailAndPassword,
	signOut
}));
vi.mock('#lib/firebase.js', () => ({ getFirebaseAuth: () => ({}) }));

const apiFetch = vi.hoisted(() => vi.fn());
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetch,
	apiFetchWithSession,
	apiBaseURL: () => '',
	apiErrorMessage: (response: Response) => response.text()
}));

const REPORTED_AT = '2026-08-28T14:02:11Z';

function jsonResponse(body: unknown): Response {
	return buildResponse(body);
}

function refusal(status: number, message: string): Response {
	return buildResponse(message, status);
}

const existingStaff = {
	staffId: 'staff-1',
	name: 'Priya Sharma',
	workState: 'NY',
	workStateReportedAt: REPORTED_AT,
	lastPracticeId: undefined,
	memberships: [{ practiceId: 'practice-1', practiceName: 'Rochester Doulas', roles: ['doula'] }]
};

const landing = {
	...existingStaff,
	memberships: [{ practiceId: 'practice-9', practiceName: 'Rochester Doulas', roles: ['doula'] }]
};

interface SetupOptions {
	/**
	The token in the URL; '' stands for a link that carries none.
	*/
	token?: string;
	/**
	What GET /api/staff/session answers in step one.
	*/
	probe?: Response;
	/**
	Whether POST /api/session mints the cookie.
	*/
	exchangeOk?: boolean;
	/**
	What POST /api/staff/accept-invite answers.
	*/
	accept?: Response;
	/**
	What the session read after acceptance answers.
	*/
	afterAccept?: Response;
	signInThrows?: boolean;
}

const globalFetch = vi.fn();
vi.stubGlobal('fetch', globalFetch);

async function setup({
	token = 'invite-1',
	probe = refusal(404, 'no matching staff account'),
	exchangeOk = true,
	accept = jsonResponse({}),
	afterAccept = jsonResponse(landing),
	signInThrows = false
}: SetupOptions = {}) {
	pageState.url = new URL(
		token === '' ? 'https://test.local/accept-invite' : `https://test.local/accept-invite?token=${token}`
	);

	const credential = { user: { getIdToken: () => Promise.resolve('id-token') } };
	if (signInThrows) {
		createUserWithEmailAndPassword.mockRejectedValue(new Error('auth/invalid-credential'));
		signInWithEmailAndPassword.mockRejectedValue(new Error('auth/invalid-credential'));
	} else {
		createUserWithEmailAndPassword.mockResolvedValue(credential);
		signInWithEmailAndPassword.mockResolvedValue(credential);
	}
	signOut.mockResolvedValue(undefined);

	globalFetch.mockImplementation((url: string) =>
		Promise.resolve(
			url.endsWith('/api/session') ? (exchangeOk ? jsonResponse({}) : refusal(401, 'no')) : accept
		)
	);
	apiFetch.mockResolvedValue(probe);
	apiFetchWithSession.mockResolvedValue(afterAccept);

	await render(Page, {});
}

beforeEach(() => {
	for (const mock of [
		createUserWithEmailAndPassword,
		signInWithEmailAndPassword,
		signOut,
		goto,
		globalFetch,
		apiFetch,
		apiFetchWithSession
	])
		mock.mockReset();
});

const emailField = () => testPage.getByLabelText('Email');
const passwordField = () => testPage.getByLabelText('Password');
const continueButton = () => testPage.getByRole('button', { name: 'Continue' });
const acceptButton = () => testPage.getByRole('button', { name: 'Accept invite' });

const nameField = () => testPage.getByLabelText('Your name');
const stateField = () => testPage.getByRole('combobox', { name: 'Which state do you work from?' });

async function identify(mode: 'signup' | 'login' = 'signup') {
	await emailField().fill('priya@example.com');
	await passwordField().fill('correct horse');
	if (mode === 'login')
		await testPage.getByRole('radio', { name: /I already have an account/ }).click();
	await continueButton().click();
}

// Step two's two questions are `required`, so the browser refuses the
// submit until both are answered -- which is the point of marking them
// required, and means a test of what happens after acceptance has to
// answer them the way a person would.
async function identifyAsNewPerson() {
	await identify('signup');
	await nameField().fill('Priya Sharma');
	await stateField().selectOptions('New Jersey');
}

describe('step one -- identifying yourself', () => {
	// The name and the work state are deliberately absent here: which of
	// them she still has to answer depends on whether she is Staff
	// already, and signing in is what settles that (#437).
	it('asks only for credentials, not for a name or a work state', async () => {
		await setup();

		await expect.element(emailField()).toBeVisible();
		await expect.element(passwordField()).toBeVisible();
		expect(testPage.getByLabelText('Your name').elements()).toHaveLength(0);
		expect(testPage.getByLabelText('Which state do you work from?').elements()).toHaveLength(0);
	});

	it('creates an account and mints the session cookie before reading the staff record', async () => {
		await setup();

		await identify('signup');

		expect(createUserWithEmailAndPassword).toHaveBeenCalled();
		expect(globalFetch).toHaveBeenCalledWith('/api/session', {
			method: 'POST',
			credentials: 'include',
			headers: { Authorization: 'Bearer id-token' }
		});
		expect(apiFetch).toHaveBeenCalledWith('/api/staff/session');
	});

	it('logs in instead when she says she already has an account', async () => {
		await setup();

		await identify('login');

		expect(signInWithEmailAndPassword).toHaveBeenCalled();
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
	});

	// Refused before the first field rather than at the submit: there is
	// nothing this screen can do with credentials when it has no invite to
	// attach them to, so offering the form would be inviting wasted typing.
	it('refuses outright when the URL carries no invite token', async () => {
		await setup({ token: '' });

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('Missing invite token');
		expect(emailField().elements()).toHaveLength(0);
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
	});

	// #467: the SDK's own string names a product and carries a banned
	// adjective, and the flat "Accept invite failed" that replaced it said
	// nothing she could act on either.
	it('reports a non-technical failure rather than whatever the auth SDK threw', async () => {
		await setup({ signInThrows: true });

		await identify();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
		await expect.element(continueButton()).toBeVisible();
	});

	it('stays on step one when the cookie exchange fails', async () => {
		await setup({ exchangeOk: false });

		await identify();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('no');
		expect(signOut).toHaveBeenCalled();
		await expect.element(continueButton()).toBeVisible();
	});

	/*
	 * Not a 404, so not "she is new here" -- and not a reason to navigate
	 * away either, which would throw the invite token out of the URL.
	 *
	 * A 5xx is ours, and since #467 it is reported as ours: "the database
	 * is having a moment" is a sentence about our infrastructure, and there
	 * is nothing she can do with it.
	 */
	it('stays put and owns the failure when the staff read fails some other way', async () => {
		await setup({ probe: refusal(500, 'the database is having a moment') });

		await identify();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
		await expect.element(testPage.getByText('the database is having a moment')).not.toBeInTheDocument();
		await expect.element(continueButton()).toBeVisible();
	});
});

describe('step two -- a person who is new here', () => {
	it('asks for her name and her work state, because nobody has them yet', async () => {
		await setup();

		await identify();

		await expect
			.element(testPage.getByRole('heading', { name: 'Tell us about yourself' }))
			.toBeVisible();
		await expect.element(nameField()).toBeVisible();
		await expect.element(stateField()).toBeVisible();
	});

	// A step that replaces the form without moving focus leaves a keyboard
	// or screen reader user tabbing through a page she was never told had
	// changed.
	it('moves focus to the new step when it appears', async () => {
		await setup();

		await identify();

		await expect
			.element(testPage.getByRole('heading', { name: 'Tell us about yourself' }))
			.toHaveFocus();
	});

	it('sends the name and the USPS code she chose, with the Bearer token', async () => {
		await setup();

		await identifyAsNewPerson();
		await acceptButton().click();

		expect(globalFetch).toHaveBeenCalledWith('/api/staff/accept-invite', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer id-token' },
			body: JSON.stringify({ inviteToken: 'invite-1', name: 'Priya Sharma', workState: 'NJ' })
		});
	});
});

describe('step two -- a person who is Staff somewhere already', () => {
	// Shown, not asked. The server discards a name and a work state from
	// someone who already has a Staff account (#316, #415), so asking for
	// them was the form telling a lie about what it would do with them.
	it('shows the name and work state she already asserted, as plain text', async () => {
		await setup({ probe: jsonResponse(existingStaff) });

		await identify('login');

		await expect
			.element(testPage.getByRole('heading', { name: 'Check your details' }))
			.toBeVisible();
		await expect.element(testPage.getByText('Priya Sharma')).toBeVisible();
		await expect
			.element(
				testPage.getByText(
					`You work from New York, self-reported ${workStateReportedOn(REPORTED_AT)}.`
				)
			)
			.toBeVisible();
		await expect
			.element(testPage.getByText(/come from the Staff account you already have/))
			.toBeVisible();
	});

	// Not disabled form controls -- a disabled input still reads as a
	// question, and these are settled facts, not questions.
	it('offers no name or work state control at all on this branch', async () => {
		await setup({ probe: jsonResponse(existingStaff) });

		await identify('login');

		expect(nameField().elements()).toHaveLength(0);
		expect(stateField().elements()).toHaveLength(0);
	});

	it('points at the one screen that can correct the work state', async () => {
		await setup({ probe: jsonResponse(existingStaff) });

		await identify('login');

		const link = testPage.getByRole('link', { name: 'Change where you work' });
		await expect.element(link).toBeVisible();
		await expect.element(link).toHaveAttribute('href', '/account');
	});

	it('sends empty strings the server already ignores rather than a stale answer', async () => {
		await setup({ probe: jsonResponse(existingStaff) });

		await identify('login');
		await acceptButton().click();

		expect(globalFetch).toHaveBeenCalledWith(
			'/api/staff/accept-invite',
			expect.objectContaining({
				body: JSON.stringify({ inviteToken: 'invite-1', name: '', workState: '' })
			})
		);
	});
});

describe('what happens after the invite is accepted', () => {
	it('lands her straight on the Practice she just joined', async () => {
		await setup();

		await identifyAsNewPerson();
		await acceptButton().click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-9'));
	});

	it('offers a picker when she now belongs to more than one Practice', async () => {
		await setup({
			afterAccept: jsonResponse({
				...landing,
				memberships: [
					{ practiceId: 'practice-9', practiceName: 'Rochester Doulas', roles: ['doula'] },
					{ practiceId: 'practice-8', practiceName: 'Finger Lakes Birth', roles: ['doula'] }
				]
			})
		});

		await identifyAsNewPerson();
		await acceptButton().click();

		await expect
			.element(testPage.getByRole('link', { name: 'Finger Lakes Birth' }))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it("shows the server's own words when the invite is refused", async () => {
		await setup({ accept: refusal(410, 'this invitation has expired') });

		await identifyAsNewPerson();
		await acceptButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('this invitation has expired');
		expect(signOut).toHaveBeenCalled();
	});

	it('owns a session read that fails after an accepted invite', async () => {
		await setup({ afterAccept: refusal(500, 'no session for you') });

		await identifyAsNewPerson();
		await acceptButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
	});

	it('owns the failure when the accept never reaches the server', async () => {
		await setup();

		await identifyAsNewPerson();
		globalFetch.mockRejectedValue(new Error('the network dropped'));
		await acceptButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
	});
});
