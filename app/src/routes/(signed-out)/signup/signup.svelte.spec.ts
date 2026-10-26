import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { privacyPolicyUrl, termsOfServiceUrl } from '#lib/legal.js';
import Page from './+page.svelte';
import { data } from './page.fixture.js';

/*
 * #745: creating the Identity Platform account and creating the Practice
 * are two steps, and the first can land while the second fails. These
 * tests are about the second attempt -- what a person's own retry, with
 * the address Identity Platform now refuses, has to do.
 */

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

// What Identity Platform throws for an address that already has an
// account -- the whole reason a half-landed signup could not be retried.
const emailTaken = { code: 'auth/email-already-in-use' };

beforeEach(() => {
	vi.stubGlobal('fetch', globalFetch);
	for (const mock of [goto, createUserWithEmailAndPassword, signInWithEmailAndPassword, signOut, globalFetch])
		mock.mockReset();
});

async function fillForm() {
	await testPage.getByLabelText('First name').fill('Priya');
	await testPage.getByLabelText('Last name').fill('Sharma');
	await testPage.getByLabelText('Practice name (optional)').fill('Riverside Doulas');
	await testPage
		.getByRole('combobox', { name: 'Which state do you work from?' })
		.selectOptions('New Jersey');
	// Chosen rather than left on whatever zone the test browser reports,
	// so what goes up is the same on every machine (#1166).
	await testPage
		.getByRole('combobox', { name: 'What timezone does this Practice work in?' })
		.selectOptions('Central time (Chicago)');
	await testPage.getByLabelText('Email').fill('priya@example.com');
	await testPage.getByLabelText('Password').fill('correct horse battery');
}

const submit = () => testPage.getByRole('button', { name: 'Create Practice' }).click();

async function setup() {
	await render(Page, { params: {}, data });
}

describe('signing up when the account half-landed (#745)', () => {
	it('finishes the Practice on a second attempt, against the account the first attempt created', async () => {
		await setup();
		await fillForm();

		// First attempt: the account is created, and the BFF half is refused
		// by the signup rate limiter -- the failure the production log shows.
		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse('too many attempts', 403));
		await submit();
		await expect.element(testPage.getByText('too many attempts')).toBeVisible();

		// Second attempt: Identity Platform now refuses the address, so the
		// screen signs in with the same password instead of stopping there.
		createUserWithEmailAndPassword.mockRejectedValueOnce(emailTaken);
		signInWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse({ practiceId: 'practice-1' }, 201));
		await submit();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1'));
		expect(signInWithEmailAndPassword).toHaveBeenCalledWith({}, 'priya@example.com', 'correct horse battery');
		// One account, not two: the retry never asked for a second one.
		expect(createUserWithEmailAndPassword).toHaveBeenCalledTimes(2);
	});

	it('still tells someone whose address belongs to another account to log in instead', async () => {
		await setup();
		await fillForm();

		createUserWithEmailAndPassword.mockRejectedValueOnce(emailTaken);
		signInWithEmailAndPassword.mockRejectedValueOnce({ code: 'auth/invalid-credential' });
		await submit();

		// The address being taken is the true story here, not the password
		// -- the sign-in was this screen's own attempt to resume, and its
		// failure is not something to report as hers.
		// Matched in the summary at the top of the page, which is the entry
		// the same string also renders beside the Email field.
		await expect
			.element(
				testPage.getByRole('link', { name: 'This email address already has an account. Log in instead.' })
			)
			.toBeVisible();
		expect(globalFetch).not.toHaveBeenCalled();
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('the password rule (#1538)', () => {
	it('states the rule before she types', async () => {
		await setup();

		await expect
			.element(testPage.getByText('Must be 15 characters or more, and not a commonly used password'))
			.toBeVisible();
	});

	it('tells Safari the rule, so a generated password passes it (#1813)', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Password')).toHaveAttribute('passwordrules', 'minlength: 15;');
	});

	it('refuses a commonly used password before the account is created, and sends it nowhere (#1539)', async () => {
		await setup();
		await fillForm();
		await testPage.getByLabelText('Password').fill('PasswordPassword');
		await submit();

		await expect
			.element(testPage.getByRole('link', { name: 'Password is too common. Enter a different password' }))
			.toBeVisible();
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
		// The list is in the browser: checking a password against it is no request at all.
		expect(globalFetch).not.toHaveBeenCalled();
	});

	it('gives the password to Identity Platform alone: the BFF request carries none of it (#1539)', async () => {
		const password = 'correct horse battery staple';
		await setup();
		await fillForm();
		await testPage.getByLabelText('Password').fill(password);
		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse({ practiceId: 'practice-1' }, 201));
		await submit();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1'));
		expect(createUserWithEmailAndPassword).toHaveBeenCalledWith({}, 'priya@example.com', password);
		// One request leaves for the BFF, and nothing in it is the password or
		// any part of it the check could have derived.
		expect(globalFetch).toHaveBeenCalledTimes(1);
		const [url, init] = globalFetch.mock.calls[0] as [string, RequestInit];
		expect(url).toBe('/api/staff/signup');
		expect(JSON.parse(String(init.body))).toEqual({
			practiceName: 'Riverside Doulas',
			firstName: 'Priya',
			lastName: 'Sharma',
			workState: 'NJ',
			timezone: 'America/Chicago'
		});
		expect(JSON.stringify(init)).not.toContain(password);
	});

	it('refuses a password of 14 characters before anything is sent', async () => {
		await setup();
		await fillForm();
		await testPage.getByLabelText('Password').fill('a'.repeat(14));
		await submit();

		await expect
			.element(testPage.getByRole('link', { name: 'Password must be 15 characters or more' }))
			.toBeVisible();
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
	});

	it('accepts 64 characters with spaces in them', async () => {
		const long = 'correct horse battery staple '.repeat(3).slice(0, 64);
		await setup();
		await fillForm();
		await testPage.getByLabelText('Password').fill(long);
		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse({ practiceId: 'practice-1' }, 201));
		await submit();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1'));
		expect(createUserWithEmailAndPassword).toHaveBeenCalledWith({}, 'priya@example.com', long);
	});

	it('shows the policy refusal Identity Platform sends in the same words', async () => {
		await setup();
		await fillForm();
		createUserWithEmailAndPassword.mockRejectedValueOnce({
			code: 'auth/password-does-not-meet-requirements'
		});
		await submit();

		await expect
			.element(testPage.getByRole('link', { name: 'Password must be 15 characters or more' }))
			.toBeVisible();
	});
});

describe('the roles statement (#290)', () => {
	it('names Owner, Admin and Doula before any submit, on first paint', async () => {
		await render(Page, { params: {}, data });

		await expect
			.element(
				testPage.getByText(
					'You hold every role — Owner, Admin and Doula. Anyone you invite holds only the roles you choose.'
				)
			)
			.toBeVisible();
		// First paint, not a response to submitting -- nothing has fetched yet.
		expect(globalFetch).not.toHaveBeenCalled();
	});
});

// The form outside a notice period: no version of either document is
// coming, which is every day but the 30 before a material change (#1824).
const outsideANotice = { ...data, coming: [] };

describe('the agreement sentence (#1547)', () => {
	const sentence =
		'By creating your Practice, you agree to the Terms of Service (opens in new tab) and the Privacy Policy (opens in new tab).';

	it('sits after the role notice and directly above the button, which it describes', async () => {
		await render(Page, { params: {}, data: outsideANotice });

		const button = testPage.getByRole('button', { name: 'Create Practice' });
		// The full stop follows the last link with no space before it (#1808).
		await expect.element(button).toHaveAccessibleDescription(sentence);
		const paragraph = testPage.getByRole('paragraph').filter({ hasText: 'By creating your Practice' });
		await expect.element(paragraph).toBeVisible();

		const notice = testPage.getByText(/You hold every role/).element();
		expect(notice.compareDocumentPosition(paragraph.element()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(paragraph.element().nextElementSibling).toBe(button.element());
	});

	it('links each document on the marketing site, in a new tab', async () => {
		await render(Page, { params: {}, data });

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
	const comingList = testPage.getByRole('list');

	it('names each coming version with the date it starts, between the sentence and the button', async () => {
		await render(Page, { params: {}, data });

		await expect.element(leadIn).toBeVisible();
		const items = comingList.getByRole('listitem').elements();
		expect(items.map((item) => item.textContent?.trim())).toEqual([
			'Terms of Service from Nov 30, 2026 (opens in new tab)',
			'Terms of Service from Dec 15, 2026 (opens in new tab)',
			'Privacy Policy from Dec 1, 2026 (opens in new tab)'
		]);
		const sentence = testPage.getByRole('paragraph').filter({ hasText: 'By creating your Practice' }).element();
		const button = testPage.getByRole('button', { name: 'Create Practice' }).element();
		// The lead-in and the list are one block, directly after the
		// sentence and directly before the button, so they are read in
		// that order.
		const block = sentence.nextElementSibling;
		expect(block?.contains(leadIn.element())).toBe(true);
		expect(block?.contains(comingList.element())).toBe(true);
		expect(block?.nextElementSibling).toBe(button);
	});

	it('links each coming version to its own page, in a new tab', async () => {
		await render(Page, { params: {}, data });

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
		await render(Page, { params: {}, data });

		await expect
			.element(testPage.getByRole('button', { name: 'Create Practice' }))
			.toHaveAccessibleDescription(
				/^By creating your Practice, you agree to .+ You also agree to: Terms of Service from Nov 30, 2026 .+ Privacy Policy from Dec 1, 2026 \(opens in new tab\)$/
			);
	});

	it('shows nothing more than the agreement sentence outside a notice period', async () => {
		await render(Page, { params: {}, data: outsideANotice });

		expect(leadIn.elements()).toHaveLength(0);
		expect(comingList.elements()).toHaveLength(0);
	});
});

describe('the address the Practice is created with (#614)', () => {
	it('sends no address in the body, leaving the ID token as the only copy', async () => {
		await render(Page, { params: {}, data });
		await fillForm();

		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse({ practiceId: 'practice-1' }, 201));
		await submit();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1'));
		const [, init] = globalFetch.mock.calls[0];
		// The Email field still exists -- it is what the Identity Platform
		// account was created with -- but it goes up as the token's `email`
		// claim, not as a field the BFF could take at its word.
		expect(JSON.parse(init.body)).toEqual({
			practiceName: 'Riverside Doulas',
			firstName: 'Priya',
			lastName: 'Sharma',
			workState: 'NJ',
			timezone: 'America/Chicago'
		});
		expect(init.headers.Authorization).toBe('Bearer id-token');
	});
});

describe('the two name fields (#1537)', () => {
	it('asks for a first name and a last name with GOV.UK autocomplete tokens', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('First name')).toHaveAttribute('autocomplete', 'given-name');
		await expect.element(testPage.getByLabelText('Last name')).toHaveAttribute('autocomplete', 'family-name');
	});

	it('refuses each empty name on its own, before anything is sent', async () => {
		await setup();
		await fillForm();
		await testPage.getByLabelText('Last name').fill('');

		await submit();

		await expect
			.element(testPage.getByRole('link', { name: 'Enter your last name' }))
			.toBeVisible();
		expect(testPage.getByRole('link', { name: 'Enter your first name' }).elements()).toHaveLength(0);
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
		expect(globalFetch).not.toHaveBeenCalled();
	});
});

describe('the optional Practice name (#1536)', () => {
	it('says it is optional, and what an empty one does, before she types', async () => {
		await setup();

		const field = testPage.getByLabelText('Practice name (optional)');
		await expect.element(field).toHaveAttribute('autocomplete', 'organization');
		await expect.element(field).not.toBeRequired();
		await expect
			.element(field)
			.toHaveAccessibleDescription(
				'Clients see it on Contracts and card statements. Leave it empty to use your own name.'
			);
	});

	it('sends an empty Practice name for the BFF to fill with her own name', async () => {
		await setup();
		await fillForm();
		await testPage.getByLabelText('Practice name (optional)').fill('');

		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse({ practiceId: 'practice-1' }, 201));
		await submit();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/practices/practice-1'));
		const [, init] = globalFetch.mock.calls[0];
		expect(JSON.parse(init.body)).toMatchObject({ practiceName: '', firstName: 'Priya', lastName: 'Sharma' });
	});

	it('lists the refusals of an empty form in the order of its fields, her name first', async () => {
		await setup();

		await submit();

		await expect.element(testPage.getByRole('link', { name: 'Enter your first name' })).toBeVisible();
		const summary = testPage.getByRole('alert');
		expect(summary.getByRole('link').elements().map((link) => link.textContent?.trim())).toEqual([
			'Enter your first name',
			'Enter your last name',
			'Choose the state you work from',
			'Enter your email address',
			'Enter a password'
		]);
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
	});
});

/*
 * #1790: until January, only an address the founder added can create a
 * Practice. The BFF says so with its own code, and the screen answers
 * that code with a screen of its own rather than an error summary.
 */
const pilotNotOpen = {
	code: 'PILOT_NOT_OPEN',
	message: 'DoulaCloud opens in January 2027. Until then, only a Practice in the pilot can sign up.'
};

describe('an address not in the pilot (#1790)', () => {
	it('says DoulaCloud opens in January 2027 and links to the waitlist', async () => {
		await setup();
		await fillForm();

		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse(pilotNotOpen, 403));
		await submit();

		await expect
			.element(testPage.getByRole('heading', { name: 'DoulaCloud opens in January 2027' }))
			.toBeVisible();
		const waitlist = testPage.getByRole('link', { name: 'Join the waitlist' });
		await expect.element(waitlist).toBeVisible();
		expect(waitlist.element().getAttribute('href')).toMatch(/^https:\/\/doula\.cloud\/\?utm_source=/);
		// Not an error summary: nothing she typed is wrong.
		expect(testPage.getByRole('alert').elements()).toHaveLength(0);
		// Signed out of the JS SDK, like every other refusal: a retry picks
		// the same account back up (#745).
		expect(signOut).toHaveBeenCalled();
		expect(goto).not.toHaveBeenCalled();
	});

	it('goes back to the form for a different email address', async () => {
		await setup();
		await fillForm();

		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(jsonResponse(pilotNotOpen, 403));
		await submit();
		await testPage.getByRole('button', { name: 'Use a different email address' }).click();

		await expect.element(testPage.getByLabelText('Email')).toHaveValue('priya@example.com');
	});
});

/*
 * #816: signing up over a live portal session ends that session, so the
 * BFF refuses the first submit and says what continuing costs. The form
 * unmounts with the button she pressed, so the warning's heading takes
 * focus (#2018 moved it onto the `Heading` atom).
 */
describe('signing up over a live portal session (#816)', () => {
	it('moves focus to the warning, since the button she pressed unmounted with the form', async () => {
		await setup();
		await fillForm();

		createUserWithEmailAndPassword.mockResolvedValueOnce(credential);
		globalFetch.mockResolvedValueOnce(
			jsonResponse(
				{
					code: 'SESSION_EVICTION_UNCONFIRMED',
					message: 'Continuing signs you out of the client portal in this browser.'
				},
				409
			)
		);
		await submit();

		await expect
			.element(testPage.getByRole('heading', { level: 2, name: 'Before you continue' }))
			.toHaveFocus();
	});
});
