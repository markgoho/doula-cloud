import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { RefusalError } from '#lib/formErrors.js';
import { FormSubmission } from '#lib/formSubmission.svelte.js';
import ReauthPrompt from './ReauthPrompt.svelte';

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

const freshUser = { uid: 'fresh-user' };
const resolver = { hints: [{ uid: 'enrollment-1' }], resolveSignIn: vi.fn() };

interface SetupOptions {
	onAuthenticated?: (user: unknown) => Promise<void> | void;
	withCancel?: boolean;
}

async function setup({
	onAuthenticated = vi.fn().mockResolvedValue(undefined),
	withCancel = true
}: SetupOptions = {}) {
	const submission = new FormSubmission();
	const onCancel = vi.fn();
	const { unmount } = await render(ReauthPrompt, {
		idPrefix: 'reauth',
		email: 'maya@example.test',
		prompt: 'Confirm your password to send the code.',
		confirmLabel: 'Send the code',
		submission,
		onAuthenticated,
		onCancel: withCancel ? onCancel : undefined
	});
	return { submission, onAuthenticated, onCancel, unmount };
}

async function reachCodeStep() {
	signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/multi-factor-auth-required' });
	getMultiFactorResolver.mockReturnValue(resolver);
	await page.getByLabelText('Password').fill('correct horse');
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect.element(page.getByLabelText('Authenticator app code')).toBeVisible();
}

beforeEach(() => {
	for (const mock of [
		signInWithEmailAndPassword,
		signOut,
		getMultiFactorResolver,
		assertionForSignIn,
		resolver.resolveSignIn
	]) {
		mock.mockReset();
	}
	signOut.mockResolvedValue(undefined);
});

describe('ReauthPrompt', () => {
	it('opens on the password step, saying why and offering Continue', async () => {
		await setup();

		await expect.element(page.getByText('Confirm your password to send the code.')).toBeVisible();
		await expect.element(page.getByLabelText('Password')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Continue' })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Send the code' })).not.toBeInTheDocument();
	});

	it('refuses an empty password without calling Identity Platform', async () => {
		await setup();

		await page.getByRole('button', { name: 'Continue' }).click();

		await expect
			.element(page.getByLabelText('Password'))
			.toHaveAccessibleDescription('Enter your password');
		expect(signInWithEmailAndPassword).not.toHaveBeenCalled();
	});

	it('signs in again as the given address', async () => {
		signInWithEmailAndPassword.mockResolvedValue({ user: freshUser });
		await setup();

		await page.getByLabelText('Password').fill('correct horse');
		await page.getByRole('button', { name: 'Continue' }).click();

		await vi.waitFor(() =>
			expect(signInWithEmailAndPassword).toHaveBeenCalledWith({}, 'maya@example.test', 'correct horse')
		);
	});

	it('hands the fresh user to the act, then signs the SDK back out', async () => {
		signInWithEmailAndPassword.mockResolvedValue({ user: freshUser });
		const { onAuthenticated } = await setup();

		await page.getByLabelText('Password').fill('correct horse');
		await page.getByRole('button', { name: 'Continue' }).click();

		await vi.waitFor(() => expect(onAuthenticated).toHaveBeenCalledWith(freshUser));
		await vi.waitFor(() => expect(signOut).toHaveBeenCalled());
	});

	it('says the password is wrong beside the password box', async () => {
		signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/invalid-credential' });
		await setup();

		await page.getByLabelText('Password').fill('wrong');
		await page.getByRole('button', { name: 'Continue' }).click();

		await expect
			.element(page.getByLabelText('Password'))
			.toHaveAccessibleDescription('Password is not correct');
	});

	it('moves to the code step when a second factor is required, dropping the password field and naming the act', async () => {
		await setup();

		await reachCodeStep();

		await expect.element(page.getByLabelText('Password')).not.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Send the code' })).toBeVisible();
	});

	it('refuses an empty code', async () => {
		await setup();
		await reachCodeStep();

		await page.getByRole('button', { name: 'Send the code' }).click();

		await expect
			.element(page.getByLabelText('Authenticator app code'))
			.toHaveAccessibleDescription('Enter the 6-digit code from your authenticator app');
	});

	it('resolves the challenge with the code and hands the user to the act', async () => {
		const assertion = { kind: 'assertion' };
		assertionForSignIn.mockReturnValue(assertion);
		resolver.resolveSignIn.mockResolvedValue({ user: freshUser });
		const { onAuthenticated } = await setup();
		await reachCodeStep();

		await page.getByLabelText('Authenticator app code').fill('123456');
		await page.getByRole('button', { name: 'Send the code' }).click();

		await vi.waitFor(() => expect(onAuthenticated).toHaveBeenCalledWith(freshUser));
		expect(assertionForSignIn).toHaveBeenCalledWith('enrollment-1', '123456');
		expect(resolver.resolveSignIn).toHaveBeenCalledWith(assertion);
	});

	it('says the code is wrong beside the code box', async () => {
		resolver.resolveSignIn.mockRejectedValue({ code: 'auth/invalid-verification-code' });
		await setup();
		await reachCodeStep();

		await page.getByLabelText('Authenticator app code').fill('000000');
		await page.getByRole('button', { name: 'Send the code' }).click();

		await expect
			.element(page.getByLabelText('Authenticator app code'))
			.toHaveAccessibleDescription(/The code is not correct/);
	});

	it('returns to the password step with the act\'s refusal when the act refuses', async () => {
		resolver.resolveSignIn.mockResolvedValue({ user: freshUser });
		const { submission } = await setup({
			onAuthenticated: vi.fn().mockRejectedValue(new RefusalError('You cannot vouch for yourself'))
		});
		await reachCodeStep();

		await page.getByLabelText('Authenticator app code').fill('123456');
		await page.getByRole('button', { name: 'Send the code' }).click();

		await expect.element(page.getByLabelText('Password')).toBeVisible();
		await expect.element(page.getByLabelText('Password')).toHaveValue('');
		expect(submission.errors.map((error) => error.message)).toEqual(['You cannot vouch for yourself']);
		expect(signOut).toHaveBeenCalled();
	});

	it('backs out through Cancel, clearing errors and signing out', async () => {
		signInWithEmailAndPassword.mockRejectedValue({ code: 'auth/invalid-credential' });
		const { submission, onCancel } = await setup();
		await page.getByLabelText('Password').fill('wrong');
		await page.getByRole('button', { name: 'Continue' }).click();
		await expect.element(page.getByText('Password is not correct').first()).toBeVisible();

		await page.getByRole('button', { name: 'Cancel' }).click();

		expect(submission.errors).toEqual([]);
		expect(onCancel).toHaveBeenCalledOnce();
		expect(signOut).toHaveBeenCalled();
		await expect.element(page.getByLabelText('Password')).toHaveValue('');
	});

	it('offers no Cancel where there is nothing to back out to', async () => {
		await setup({ withCancel: false });

		await expect.element(page.getByRole('button', { name: 'Cancel' })).not.toBeInTheDocument();
	});

	it('signs out of the SDK when the prompt is left mid-flow', async () => {
		const { unmount } = await setup();
		expect(signOut).not.toHaveBeenCalled();

		await unmount();

		expect(signOut).toHaveBeenCalled();
	});
});
