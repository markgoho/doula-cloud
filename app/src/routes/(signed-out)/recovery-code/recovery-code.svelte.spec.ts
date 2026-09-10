import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';

const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetch }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

interface SetupOptions {
	response?: Response;
}

/*
 * Returns the spend calls, because what this screen produces is one POST
 * and where it sends her afterwards -- nothing on it is read back.
 */
async function setup({ response = jsonResponse(undefined, 204) }: SetupOptions = {}) {
	const spends: { path: string; init?: RequestInit }[] = [];
	apiFetch.mockImplementation((path: string, init?: RequestInit) => {
		spends.push({ path, init });
		return Promise.resolve(response);
	});
	await render(Page, {});
	return { spends };
}

async function fillAndSubmit(email: string, code: string) {
	await testPage.getByLabelText('Email').fill(email);
	await testPage.getByLabelText('Recovery code').fill(code);
	await testPage.getByRole('button', { name: 'Continue' }).click();
}

beforeEach(() => {
	apiFetch.mockReset();
	goto.mockReset();
});

describe('spending a recovery code', () => {
	// #615's AC: the screen is reached signed out and never asks for a
	// password, because the whole reason she is here is that she cannot
	// finish a sign-in.
	it('asks only for an address and a code', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Use a recovery code' })).toBeVisible();
		await expect.element(testPage.getByLabelText('Email')).toBeVisible();
		await expect.element(testPage.getByLabelText('Recovery code')).toBeVisible();
		expect(testPage.getByLabelText('Password').elements()).toHaveLength(0);
	});

	it('refuses an empty form without calling the endpoint, naming both fields', async () => {
		const { spends } = await setup();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Enter your email address' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Enter your recovery code' })).toBeVisible();
		expect(spends).toHaveLength(0);
	});

	it('spends the code and sends her to log in again, since none of this mints a session', async () => {
		const { spends } = await setup();

		await fillAndSubmit('anne-marie@example.test', 'CODE-1');

		await vi.waitFor(() => expect(spends).toHaveLength(1));
		expect(spends[0].path).toBe('/api/staff/mfa-recovery/spend');
		expect(JSON.parse(String(spends[0].init?.body))).toEqual({
			email: 'anne-marie@example.test',
			code: 'CODE-1'
		});
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/login?codeSpent=true'));
	});

	/*
	 * #168: the endpoint answers a wrong code and an address it has never
	 * heard of with one sentence, and this screen's job is to show that
	 * sentence rather than decide which of the two it thinks happened --
	 * which is why the refusal is untargeted rather than pointed at the
	 * code field.
	 */
	it("shows the endpoint's one refusal, and points it at neither field", async () => {
		await setup({ response: jsonResponse({ message: 'this code is invalid or has expired' }, 400) });

		await fillAndSubmit('nobody@example.test', 'nope');

		await expect
			.element(testPage.getByText('this code is invalid or has expired'))
			.toBeVisible();
		expect(testPage.getByRole('link', { name: 'this code is invalid or has expired' }).elements()).toHaveLength(0);
		expect(goto).not.toHaveBeenCalled();
	});

	// #602's per-account throttle. It reads as a plain "wait and try
	// again", beside the form, rather than as a broken page.
	it('shows being throttled as an ordinary refusal on the form', async () => {
		await setup({
			response: jsonResponse({ message: 'too many requests -- try again in 43 seconds' }, 429)
		});

		await fillAndSubmit('anne-marie@example.test', 'CODE-1');

		await expect
			.element(testPage.getByText('too many requests -- try again in 43 seconds'))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('offers the way back to the log-in screen', async () => {
		await setup();

		await expect.element(testPage.getByRole('link', { name: 'Log in' })).toHaveAttribute('href', '/login');
	});
});
