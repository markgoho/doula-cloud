import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { privacyPolicyUrl, termsOfServiceUrl } from '#lib/legal.js';
import { toPageState } from '../../../routeFixture.js';
import { signupDraft } from '../signupJourney.svelte.js';
import Page from './+page.svelte';
import { answers, data, fixture, seedSignup } from './page.fixture.js';

/*
 * The check page that ends setting up a Practice (#2049), and the one
 * button that creates it. Everything the one-page form's spec held about
 * the press itself is here: the half-landed account (#745), the password
 * going to Identity Platform alone (#1539), the address on the token
 * (#614), the agreement (#1547, #1824), the pilot (#1790) and the portal
 * session (#816).
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
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
vi.mock('#lib/api.js', () => ({ apiBaseURL: () => '' }));

const globalFetch = vi.hoisted(() => vi.fn());

const credential = { user: { getIdToken: async () => 'id-token' } };
const password = 'correct horse battery staple';
// What Identity Platform throws for an address that already has an account.
const emailTaken = { code: 'auth/email-already-in-use' };

beforeEach(() => {
	vi.stubGlobal('fetch', globalFetch);
	for (const mock of [goto, createUserWithEmailAndPassword, signInWithEmailAndPassword, signOut, globalFetch])
		mock.mockReset();
	seedSignup();
	Object.assign(pageState, toPageState(fixture));
});

// The form outside a notice period: no version of either document is
// coming, which is every day but the 30 before a material change (#1824).
const outsideANotice = { ...data, coming: [] };

async function setup(pageData = data) {
	await render(Page, { params: {}, data: pageData });
}

const create = () => testPage.getByRole('button', { name: 'Create your Practice' }).click();

/**
A created Practice: the account, then the BFF's 201.
*/
function answerCreated() {
	createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
	globalFetch.mockResolvedValueOnce(jsonResponse({ practiceId: 'practice-1' }, 201));
}

function sentBody(call = 0): Record<string, unknown> {
	const [, init] = globalFetch.mock.calls[call] as [string, RequestInit];
	return JSON.parse(String(init.body)) as Record<string, unknown>;
}

describe('the answers', () => {
	it('lists each answer with a Change link back to the page that asks it', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Check your answers' })).toBeVisible();
		await expect.element(testPage.getByText('Anne-Marie Ochieng-Whitfield')).toBeVisible();
		await expect.element(testPage.getByText(answers.practiceName)).toBeVisible();
		await expect.element(testPage.getByText('Mountain time, no daylight saving (Phoenix)')).toBeVisible();
		const changes = testPage.getByRole('link', { name: /^Change/ }).elements();
		expect(changes.map((link) => link.getAttribute('href'))).toEqual([
			'/signup?from=check',
			'/signup?from=check',
			'/signup/practice?from=check',
			'/signup/where?from=check',
			'/signup/where?from=check'
		]);
		await expect.element(testPage.getByText('3 of 3 steps completed')).toBeVisible();
	});

	it('shows her own name as the Practice name when she left it empty (#1536)', async () => {
		signupDraft.update({ practiceName: '' });
		await setup();

		await expect.element(testPage.getByRole('definition').filter({ hasText: /^Anne-Marie Ochieng-Whitfield$/ }).first()).toBeVisible();
		expect(testPage.getByText(answers.practiceName).elements()).toHaveLength(0);
	});

	it('says what is not answered on a page reached by its address', async () => {
		signupDraft.clear();
		signupDraft.update({ timezone: '' });
		await setup();

		expect(testPage.getByText('Not answered').elements()).toHaveLength(4);
		await expect.element(testPage.getByText('Your own name')).toBeVisible();
	});
});

describe('creating the Practice', () => {
	it('gives the password to Identity Platform alone, and the address to nobody but the token (#1539, #614)', async () => {
		await setup();
		answerCreated();
		await create();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1'));
		expect(createUserWithEmailAndPassword).toHaveBeenCalledWith({}, answers.email, password);
		expect(globalFetch).toHaveBeenCalledTimes(1);
		const [url, init] = globalFetch.mock.calls[0] as [string, RequestInit & { headers: Record<string, string> }];
		expect(url).toBe('/api/staff/signup');
		expect(sentBody()).toEqual({
			practiceName: answers.practiceName,
			firstName: answers.firstName,
			lastName: answers.lastName,
			workState: 'DC',
			timezone: 'America/Phoenix'
		});
		expect(init.headers.Authorization).toBe('Bearer id-token');
		expect(JSON.stringify(init)).not.toContain(password);
	});

	it('drops the draft once the Practice exists', async () => {
		await setup();
		answerCreated();
		await create();

		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		expect(signupDraft.answers.firstName).toBe('');
		expect(sessionStorage.getItem('signup-draft')).toBeNull();
	});

	it('sends an empty Practice name for the BFF to fill with her own name (#1536)', async () => {
		signupDraft.update({ practiceName: '' });
		await setup();
		answerCreated();
		await create();

		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		expect(sentBody()).toMatchObject({ practiceName: '', firstName: answers.firstName });
	});

	it('asks for the password again after a reload, linked to the first page', async () => {
		signupDraft.password = '';
		await setup();
		await create();

		const link = testPage.getByRole('link', { name: 'Enter a password' });
		await expect.element(link).toHaveAttribute('href', '/signup?from=check');
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
		// The first page says it beside the box, once she follows the link.
		expect(signupDraft.refusalsFor('you')).toEqual([{ message: 'Enter a password', targetId: 'signup-password' }]);
	});

	it('links a field the BFF refused to the page that asks it', async () => {
		await setup();
		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(
			jsonResponse(
				{ code: 'VALIDATION', message: 'Invalid signup', details: { workState: 'Choose the state you work from' } },
				400
			)
		);
		await create();

		await expect
			.element(testPage.getByRole('link', { name: 'Choose the state you work from' }))
			.toHaveAttribute('href', '/signup/where?from=check');
		expect(signOut).toHaveBeenCalled();
	});

	it('shows the policy refusal Identity Platform sends in the same words, linked to the first page', async () => {
		await setup();
		createUserWithEmailAndPassword.mockRejectedValueOnce({ code: 'auth/password-does-not-meet-requirements' });
		await create();

		await expect
			.element(testPage.getByRole('link', { name: 'Password must be 15 characters or more' }))
			.toHaveAttribute('href', '/signup?from=check');
	});
});

describe('signing up when the account half-landed (#745)', () => {
	it('finishes the Practice on a second press, against the account the first press created', async () => {
		await setup();

		// First press: the account is created, and the BFF half is refused
		// by the signup rate limiter.
		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse('too many attempts', 403));
		await create();
		await expect.element(testPage.getByText('too many attempts')).toBeVisible();

		// Second press: Identity Platform now refuses the address, so the
		// page signs in with the same password instead of stopping there.
		createUserWithEmailAndPassword.mockRejectedValueOnce(emailTaken);
		signInWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse({ practiceId: 'practice-1' }, 201));
		await create();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1'));
		expect(signInWithEmailAndPassword).toHaveBeenCalledWith({}, answers.email, password);
		expect(createUserWithEmailAndPassword).toHaveBeenCalledTimes(2);
	});

	it('still tells someone whose address belongs to another account to log in instead', async () => {
		await setup();
		createUserWithEmailAndPassword.mockRejectedValueOnce(emailTaken);
		signInWithEmailAndPassword.mockRejectedValueOnce({ code: 'auth/invalid-credential' });
		await create();

		await expect
			.element(testPage.getByRole('link', { name: 'This email address already has an account. Log in instead.' }))
			.toHaveAttribute('href', '/signup?from=check');
		expect(globalFetch).not.toHaveBeenCalled();
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('the agreement sentence (#1547)', () => {
	const sentence =
		'By creating your Practice, you agree to the Terms of Service (opens in new tab) and the Privacy Policy (opens in new tab).';

	it('sits after the answers and above the button, which it describes', async () => {
		await setup(outsideANotice);

		const button = testPage.getByRole('button', { name: 'Create your Practice' });
		// The full stop follows the last link with no space before it (#1808).
		await expect.element(button).toHaveAccessibleDescription(sentence);
		const paragraph = testPage.getByRole('paragraph').filter({ hasText: 'By creating your Practice' });
		await expect.element(paragraph).toBeVisible();
		const lastAnswer = testPage.getByText('Mountain time, no daylight saving (Phoenix)').element();
		expect(lastAnswer.compareDocumentPosition(paragraph.element()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(paragraph.element().compareDocumentPosition(button.element()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('links each document on the marketing site, in a new tab', async () => {
		await setup();

		for (const [name, href] of [
			['Terms of Service (opens in new tab)', termsOfServiceUrl],
			['Privacy Policy (opens in new tab)', privacyPolicyUrl]
		]) {
			const link = testPage.getByRole('link', { name });
			await expect.element(link).toHaveAttribute('href', href);
			await expect.element(link).toHaveAttribute('target', '_blank');
		}
	});
});

describe('the coming versions, during a notice period (#1824)', () => {
	const leadIn = testPage.getByText('You also agree to:');

	it('names each coming version with the date it starts, linked to its own page', async () => {
		await setup();

		await expect.element(leadIn).toBeVisible();
		for (const [name, href] of [
			['Terms of Service from Nov 30, 2026 (opens in new tab)', 'https://doula.cloud/terms/2026-11-30'],
			['Terms of Service from Dec 15, 2026 (opens in new tab)', 'https://doula.cloud/terms/2026-12-15'],
			['Privacy Policy from Dec 1, 2026 (opens in new tab)', 'https://doula.cloud/privacy/2026-12-01']
		]) {
			const link = testPage.getByRole('link', { name });
			await expect.element(link).toHaveAttribute('href', href);
			await expect.element(link).toHaveAttribute('target', '_blank');
		}
	});

	it('is read after the agreement sentence in the description of the button', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('button', { name: 'Create your Practice' }))
			.toHaveAccessibleDescription(
				/^By creating your Practice, you agree to .+ You also agree to: Terms of Service from Nov 30, 2026 .+ Privacy Policy from Dec 1, 2026 \(opens in new tab\)$/
			);
	});

	it('shows nothing more than the agreement sentence outside a notice period', async () => {
		await setup(outsideANotice);

		expect(leadIn.elements()).toHaveLength(0);
	});
});

/*
 * #1790: until January, only an address the founder added can create a
 * Practice. The BFF says so with its own code, and the page answers that
 * code with a screen of its own rather than an error summary.
 */
const pilotNotOpen = {
	code: 'PILOT_NOT_OPEN',
	message: 'DoulaCloud opens in January 2027. Until then, only a Practice in the pilot can sign up.'
};

describe('an address not in the pilot (#1790)', () => {
	it('says DoulaCloud opens in January 2027 and links to the waitlist', async () => {
		await setup();
		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse(pilotNotOpen, 403));
		await create();

		await expect
			.element(testPage.getByRole('heading', { name: 'DoulaCloud opens in January 2027' }))
			.toBeVisible();
		const waitlist = testPage.getByRole('link', { name: 'Join the waitlist' });
		await expect.element(waitlist).toBeVisible();
		expect(waitlist.element().getAttribute('href')).toMatch(/^https:\/\/doula\.cloud\/\?utm_source=/);
		await expect.element(testPage.getByText(/Until then, only doulas in the pilot/)).toHaveFocus();
		// Not an error summary: nothing she typed is wrong.
		expect(testPage.getByRole('alert').elements()).toHaveLength(0);
		expect(signOut).toHaveBeenCalled();
		expect(goto).not.toHaveBeenCalled();
	});

	it('goes back to the first page for a different email address', async () => {
		await setup();
		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse(pilotNotOpen, 403));
		await create();
		await testPage.getByRole('button', { name: 'Use a different email address' }).click();

		expect(goto).toHaveBeenCalledWith('/signup?from=check');
	});
});

/*
 * #816: signing up over a live portal session ends that session, so the
 * BFF refuses the first press and says what continuing costs.
 */
const evictionUnconfirmed = {
	code: 'SESSION_EVICTION_UNCONFIRMED',
	message: 'Continuing signs you out of the client portal in this browser.'
};

async function pressIntoTheWarning() {
	await setup();
	createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
	globalFetch.mockResolvedValueOnce(jsonResponse(evictionUnconfirmed, 409));
	await create();
}

describe('signing up over a live portal session (#816)', () => {
	it('moves focus to the warning, since the button she pressed is gone', async () => {
		await pressIntoTheWarning();

		await expect
			.element(testPage.getByRole('heading', { level: 2, name: 'Before you continue' }))
			.toHaveFocus();
		await expect.element(testPage.getByText(evictionUnconfirmed.message)).toBeVisible();
	});

	it('sends the same signup again with X-Confirmed on Continue and sign out', async () => {
		await pressIntoTheWarning();
		globalFetch.mockResolvedValueOnce(jsonResponse({ practiceId: 'practice-1' }, 201));
		await testPage.getByRole('button', { name: 'Continue and sign out' }).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1'));
		const [, init] = globalFetch.mock.calls[1] as [string, RequestInit & { headers: Record<string, string> }];
		expect(init.headers['X-Confirmed']).toBe('true');
		expect(sentBody(1)).toEqual(sentBody(0));
	});

	it('shows a refusal of the confirmed press on the check page', async () => {
		await pressIntoTheWarning();
		globalFetch.mockResolvedValueOnce(jsonResponse('too many attempts', 403));
		await testPage.getByRole('button', { name: 'Continue and sign out' }).click();

		await expect.element(testPage.getByText('too many attempts')).toBeVisible();
		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Check your answers' })).toBeVisible();
	});

	it('returns to the answers on Cancel, signed out of the attempt', async () => {
		await pressIntoTheWarning();
		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(testPage.getByRole('button', { name: 'Create your Practice' })).toBeVisible();
		expect(signOut).toHaveBeenCalled();
	});
});
