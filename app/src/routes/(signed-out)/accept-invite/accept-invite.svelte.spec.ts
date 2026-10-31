import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../routeFixture.js';
import { fixture, invitation, returningInvitation } from './page.fixture.js';

/*
 * The `page` this screen reads comes from the route's own fixture (#596),
 * through the same `toPageState` the continuum sweep installs. `vi.mock`
 * is hoisted above every import, so the object is declared empty here and
 * filled once the imports have run.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const inviteToken = new URL(fixture.url).searchParams.get('token') ?? '';

function urlWithToken(token: string): URL {
	const url = new URL(fixture.url);
	if (token === '') url.search = '';
	else url.searchParams.set('token', token);
	return url;
}

const gotoWithOutcome = vi.hoisted(() => vi.fn());
vi.mock('#lib/outcome.js', () => ({ gotoWithOutcome }));

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
	apiBaseURL: () => ''
}));

const PASSWORD = 'correct horse battery';

function refusal(status: number, message: string): Response {
	return jsonResponse(message, status);
}

const accepted = {
	staffId: 'staff-1',
	practiceId: 'practice-9',
	practiceName: invitation.practiceName,
	roles: ['admin'],
	employmentType: 'employee'
};

const session = {
	staffId: 'staff-1',
	name: 'Priya Sharma',
	workState: 'NJ',
	workStateReportedAt: '2026-08-28T14:02:11Z',
	lastPracticeId: undefined,
	memberships: [{ practiceId: 'practice-9', practiceName: invitation.practiceName, roles: ['admin'] }]
};

interface SetupOptions {
	/**
	The token in the URL; '' stands for a link that carries none.
	*/
	token?: string;
	/**
	What the Invitation read answers.
	*/
	lookup?: () => Response;
	/**
	What each POST /api/staff/accept-invite answers, in order.
	*/
	accepts?: (() => Response)[];
	/**
	What the session read after acceptance answers.
	*/
	afterAccept?: Response;
	signInThrows?: boolean;
}

const globalFetch = vi.fn();
vi.stubGlobal('fetch', globalFetch);

async function setup({
	token = inviteToken,
	lookup = () => jsonResponse(invitation),
	accepts = [() => jsonResponse(accepted)],
	afterAccept = jsonResponse(session),
	signInThrows = false
}: SetupOptions = {}) {
	pageState.url = urlWithToken(token);

	const credential = { user: { getIdToken: () => Promise.resolve('id-token') } };
	for (const mock of [createUserWithEmailAndPassword, signInWithEmailAndPassword]) {
		if (signInThrows) mock.mockRejectedValue(new Error('auth/invalid-credential'));
		else mock.mockResolvedValue(credential);
	}
	signOut.mockResolvedValue(undefined);

	const queue = [...accepts];
	globalFetch.mockImplementation(() => Promise.resolve((queue.length > 1 ? queue.shift()! : queue[0]!)()));
	apiFetch.mockImplementation(() => Promise.resolve(lookup()));
	apiFetchWithSession.mockResolvedValue(afterAccept);

	await render(Page, {});
}

beforeEach(() => {
	for (const mock of [
		createUserWithEmailAndPassword,
		signInWithEmailAndPassword,
		signOut,
		gotoWithOutcome,
		globalFetch,
		apiFetch,
		apiFetchWithSession
	])
		mock.mockReset();
});

const passwordField = (label = 'Create a password') => testPage.getByLabelText(label, { exact: true });
const acceptButton = () => testPage.getByRole('button', { name: 'Accept invite' });
const firstNameField = () => testPage.getByLabelText('First name');
const lastNameField = () => testPage.getByLabelText('Last name');
const stateField = () => testPage.getByRole('combobox', { name: 'Which state do you work from?' });
const heading = () => testPage.getByRole('heading', { level: 1 });

function acceptBody(call = 0): Record<string, string> {
	return JSON.parse(globalFetch.mock.calls[call]![1].body as string) as Record<string, string>;
}

async function answerAsNewPerson() {
	await expect.element(firstNameField()).toBeVisible();
	await firstNameField().fill('Priya');
	await lastNameField().fill('Sharma');
	await stateField().selectOptions('New Jersey');
	await passwordField().fill(PASSWORD);
}

describe('before she types -- the Invitation read (#2238)', () => {
	it('reads the Invitation by the token in the link, in the body', async () => {
		await setup();

		await expect.element(heading()).toBeVisible();
		expect(apiFetch).toHaveBeenCalledWith(
			'/api/staff/accept-invite/lookup',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ inviteToken }) })
		);
	});

	it('names the Practice in the heading', async () => {
		await setup();

		await expect.element(heading()).toHaveTextContent(`Join ${invitation.practiceName}`);
	});

	it('says what she joins as and the address the invite went to', async () => {
		await setup();

		await expect
			.element(
				testPage.getByText(`Your invite to join as an Owner, Admin and Doula was sent to ${invitation.address}.`)
			)
			.toBeVisible();
	});

	it('asks no account question and no email address', async () => {
		await setup();

		await expect.element(acceptButton()).toBeVisible();
		await expect.element(testPage.getByRole('radio')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('textbox', { name: 'Email address' })).not.toBeInTheDocument();
	});

	it('refuses outright when the URL carries no invite token', async () => {
		await setup({ token: '' });

		await expect.element(testPage.getByText('This link is not complete.')).toBeVisible();
		await expect.element(heading()).toHaveTextContent('Accept your invite');
		expect(apiFetch).not.toHaveBeenCalled();
	});

	it.each([
		[404, 'invitation not found, already accepted, or revoked'],
		[410, 'this invitation has expired -- ask for a new one']
	])('says the link cannot be used when the read answers %i', async (status, message) => {
		await setup({ lookup: () => refusal(status, message) });

		await expect.element(testPage.getByText('This link has expired or was already used.')).toBeVisible();
		await expect.element(testPage.getByText('Ask the Practice to send the invite again.')).toBeVisible();
		await expect.element(acceptButton()).not.toBeInTheDocument();
	});

	it('owns a read that fails some other way, and reads again on Try again', async () => {
		await setup({ lookup: () => refusal(500, 'boom') });

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
		apiFetch.mockImplementation(() => Promise.resolve(jsonResponse(invitation)));
		await testPage.getByRole('button', { name: 'Try again' }).click();

		await expect.element(acceptButton()).toBeVisible();
		expect(apiFetch).toHaveBeenCalledTimes(2);
	});

	it('owns a read that never reaches the server', async () => {
		await setup({
			lookup: () => {
				throw new Error('the network dropped');
			}
		});

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
	});
});

describe('a person who is new here', () => {
	it('asks for her name, her work state and a new password, on the one screen', async () => {
		await setup();

		await expect.element(firstNameField()).toBeVisible();
		await expect.element(lastNameField()).toBeVisible();
		await expect.element(stateField()).toBeVisible();
		await expect.element(passwordField()).toHaveAttribute('autocomplete', 'new-password');
		await expect.element(passwordField()).toHaveAttribute('passwordrules');
		await expect
			.element(testPage.getByText('Must be 15 characters or more, and not a commonly used password'))
			.toBeVisible();
	});

	it('refuses each empty answer, in field order, before anything is sent', async () => {
		await setup();

		await acceptButton().click();

		const summary = testPage.getByRole('alert').first();
		await expect.element(summary.getByRole('link').first()).toBeVisible();
		expect(
			summary
				.getByRole('link')
				.elements()
				.map((link) => link.textContent.trim())
		).toEqual(['Enter your first name', 'Enter your last name', 'Choose the state you work from', 'Enter your password']);
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
		expect(globalFetch).not.toHaveBeenCalled();
	});

	it('refuses a commonly used password before the account is created (#1539)', async () => {
		await setup();
		await answerAsNewPerson();
		await passwordField().fill('passwordpassword');

		await acceptButton().click();

		await expect
			.element(testPage.getByRole('link', { name: 'Password is too common. Enter a different password' }))
			.toBeVisible();
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
	});

	it('creates the account on the invited address and sends her answers with the Bearer token', async () => {
		await setup();
		await answerAsNewPerson();

		await acceptButton().click();

		await vi.waitFor(() => expect(gotoWithOutcome).toHaveBeenCalled());
		expect(createUserWithEmailAndPassword).toHaveBeenCalledWith({}, invitation.address, PASSWORD);
		expect(globalFetch.mock.calls[0]![0]).toBe('/api/staff/accept-invite');
		expect(globalFetch.mock.calls[0]![1].headers).toMatchObject({ Authorization: 'Bearer id-token' });
		expect(acceptBody()).toEqual({ inviteToken, firstName: 'Priya', lastName: 'Sharma', workState: 'NJ' });
	});

	it('gives the password to Identity Platform alone: no BFF request carries it (#1539)', async () => {
		await setup();
		await answerAsNewPerson();

		await acceptButton().click();

		await vi.waitFor(() => expect(gotoWithOutcome).toHaveBeenCalled());
		const sent = JSON.stringify([globalFetch.mock.calls, apiFetch.mock.calls, apiFetchWithSession.mock.calls]);
		expect(sent).not.toContain(PASSWORD);
	});

	it('reports a non-technical failure rather than whatever the auth SDK threw', async () => {
		await setup({ signInThrows: true });
		await answerAsNewPerson();

		await acceptButton().click();

		await expect.element(testPage.getByRole('alert')).toBeVisible();
		await expect.element(testPage.getByRole('alert')).not.toHaveTextContent('auth/invalid-credential');
		expect(globalFetch).not.toHaveBeenCalled();
	});
});

describe('a person who already has an account', () => {
	it('asks for her password only', async () => {
		await setup({ lookup: () => jsonResponse(returningInvitation) });

		await expect.element(passwordField('Password')).toHaveAttribute('autocomplete', 'current-password');
		await expect.element(passwordField('Password')).not.toHaveAttribute('passwordrules');
		await expect.element(firstNameField()).not.toBeInTheDocument();
		await expect.element(stateField()).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: /where you work/ })).not.toBeInTheDocument();
	});

	it('refuses an empty password before anything is sent', async () => {
		await setup({ lookup: () => jsonResponse(returningInvitation) });
		await expect.element(passwordField('Password')).toBeVisible();

		await acceptButton().click();

		await expect.element(testPage.getByRole('link', { name: 'Enter your password' })).toBeVisible();
		expect(signInWithEmailAndPassword).not.toHaveBeenCalled();
	});

	it('signs in on the invited address and sends no name or work state', async () => {
		await setup({ lookup: () => jsonResponse(returningInvitation) });
		await passwordField('Password').fill('an older short one');

		await acceptButton().click();

		await vi.waitFor(() => expect(gotoWithOutcome).toHaveBeenCalled());
		expect(signInWithEmailAndPassword).toHaveBeenCalledWith({}, returningInvitation.address, 'an older short one');
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
		expect(acceptBody()).toEqual({ inviteToken, firstName: '', lastName: '', workState: '' });
	});

	it('asks for her name and work state when her account has no Staff record behind it', async () => {
		await setup({
			lookup: () => jsonResponse(returningInvitation),
			accepts: [
				() =>
					jsonResponse(
						{
							code: 'INVALID_ARGUMENT',
							message: 'enter your first and last name',
							details: { firstName: 'Enter your first name', lastName: 'Enter your last name' }
						},
						400
					),
				() => jsonResponse(accepted)
			]
		});
		await passwordField('Password').fill(PASSWORD);
		await acceptButton().click();

		await expect.element(firstNameField()).toBeVisible();
		expect(signOut).not.toHaveBeenCalled();
		await firstNameField().fill('Priya');
		await lastNameField().fill('Sharma');
		await stateField().selectOptions('New Jersey');
		await acceptButton().click();

		await vi.waitFor(() => expect(gotoWithOutcome).toHaveBeenCalled());
		expect(signInWithEmailAndPassword).toHaveBeenCalledTimes(1);
		expect(acceptBody(1)).toEqual({ inviteToken, firstName: 'Priya', lastName: 'Sharma', workState: 'NJ' });
		const [, message] = gotoWithOutcome.mock.calls[0]!;
		expect(message).not.toContain('still work from');
	});
});

describe('the Privacy Policy notice (#1557)', () => {
	it('links /privacy in a new tab and asks for no agreement', async () => {
		await setup();

		const notice = testPage.getByText(/tells you what DoulaCloud holds\s+about you/);
		await expect.element(notice).toBeVisible();
		await expect.element(notice).not.toHaveTextContent(/agree|consent|accept/i);
		const link = testPage.getByRole('link', { name: 'Privacy Policy' });
		await expect.element(link).toHaveAttribute('href', 'https://doula.cloud/privacy');
		await expect.element(link).toHaveAttribute('target', '_blank');
		await expect.element(testPage.getByRole('checkbox')).not.toBeInTheDocument();
	});
});

describe('what happens after the invite is accepted (#2239)', () => {
	it('lands on the Practice she joined and says she joined it, and as what', async () => {
		await setup();
		await answerAsNewPerson();

		await acceptButton().click();

		await vi.waitFor(() =>
			expect(gotoWithOutcome).toHaveBeenCalledWith(
				'/practices/practice-9',
				`You've joined ${invitation.practiceName}. You're an Admin, as an employee.`
			)
		);
		expect(signOut).toHaveBeenCalled();
	});

	it('lands on the joined Practice, not the one she used last, and says the rest is unchanged', async () => {
		await setup({
			lookup: () => jsonResponse(returningInvitation),
			accepts: [() => jsonResponse({ ...accepted, roles: ['doula'], employmentType: 'contractor' })],
			afterAccept: jsonResponse({
				...session,
				lastPracticeId: 'practice-1',
				memberships: [...session.memberships, { practiceId: 'practice-1', practiceName: 'Rochester Doulas', roles: ['doula'] }]
			})
		});
		await passwordField('Password').fill(PASSWORD);

		await acceptButton().click();

		await vi.waitFor(() =>
			expect(gotoWithOutcome).toHaveBeenCalledWith(
				'/practices/practice-9',
				`You've joined ${invitation.practiceName}. You're a Doula, as a contractor. Your other Practices are unchanged. You still work from New Jersey.`
			)
		);
	});

	it("shows the server's own words when the invite is refused, and signs out", async () => {
		await setup({ accepts: [() => refusal(410, 'this invitation has expired')] });
		await answerAsNewPerson();

		await acceptButton().click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('this invitation has expired');
		expect(signOut).toHaveBeenCalled();
		expect(gotoWithOutcome).not.toHaveBeenCalled();
	});

	it('owns a session read that fails after an accepted invite', async () => {
		await setup({ afterAccept: refusal(500, 'no session for you') });
		await answerAsNewPerson();

		await acceptButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
	});

	it('owns the failure when the accept never reaches the server', async () => {
		await setup();
		await answerAsNewPerson();
		globalFetch.mockRejectedValue(new Error('the network dropped'));

		await acceptButton().click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('There is a problem with the service. Try again in a few minutes.');
	});
});

function evictionWarning(): Response {
	return jsonResponse(
		{
			code: 'SESSION_EVICTION_UNCONFIRMED',
			message: 'Continuing signs you out of the client portal in this browser.'
		},
		409
	);
}

/*
 * #816: accepting over a live portal session ends that session, so the BFF
 * refuses the first accept and says what continuing costs.
 */
describe('accepting over a live portal session (#816)', () => {
	it('moves focus to the warning, since the button she pressed unmounted with the form', async () => {
		await setup({ accepts: [evictionWarning, () => jsonResponse(accepted)] });
		await answerAsNewPerson();

		await acceptButton().click();

		await expect
			.element(testPage.getByRole('heading', { level: 2, name: 'Before you continue' }))
			.toHaveFocus();
	});

	it('sends the same accept again, confirmed, on Continue and sign out', async () => {
		await setup({ accepts: [evictionWarning, () => jsonResponse(accepted)] });
		await answerAsNewPerson();
		await acceptButton().click();

		await testPage.getByRole('button', { name: 'Continue and sign out' }).click();

		await vi.waitFor(() => expect(gotoWithOutcome).toHaveBeenCalled());
		expect(globalFetch.mock.calls[1]![1].headers).toMatchObject({ 'X-Confirmed': 'true' });
		expect(createUserWithEmailAndPassword).toHaveBeenCalledTimes(1);
	});

	it('goes back to the form on Cancel, with what she typed', async () => {
		await setup({ accepts: [evictionWarning] });
		await answerAsNewPerson();
		await acceptButton().click();

		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(firstNameField()).toHaveValue('Priya');
		expect(signOut).not.toHaveBeenCalled();
	});

	it('returns to the form with the refusal when the confirmed accept is refused', async () => {
		await setup({ accepts: [evictionWarning, () => refusal(410, 'this invitation has expired')] });
		await answerAsNewPerson();
		await acceptButton().click();

		await testPage.getByRole('button', { name: 'Continue and sign out' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('this invitation has expired');
		await expect.element(acceptButton()).toBeVisible();
		expect(signOut).toHaveBeenCalled();
	});
});
