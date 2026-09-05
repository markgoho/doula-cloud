import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';

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
	await testPage.getByLabelText('Practice name').fill('Riverside Doulas');
	await testPage.getByLabelText('Your name').fill('Priya Sharma');
	await testPage
		.getByRole('combobox', { name: 'Which state do you work from?' })
		.selectOptions('New Jersey');
	await testPage.getByLabelText('Email').fill('priya@example.com');
	await testPage.getByLabelText('Password').fill('correct horse');
}

const submit = () => testPage.getByRole('button', { name: 'Create Practice' }).click();

describe('signing up when the account half-landed (#745)', () => {
	it('finishes the Practice on a second attempt, against the account the first attempt created', async () => {
		await render(Page, {});
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
		expect(signInWithEmailAndPassword).toHaveBeenCalledWith({}, 'priya@example.com', 'correct horse');
		// One account, not two: the retry never asked for a second one.
		expect(createUserWithEmailAndPassword).toHaveBeenCalledTimes(2);
	});

	it('still tells someone whose address belongs to another account to log in instead', async () => {
		await render(Page, {});
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
