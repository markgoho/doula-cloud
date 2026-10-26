import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { fixture } from './page.fixture.js';

/*
 * #619's spend half. This route reads the token off `page.url` through
 * `#lib/appState.svelte.js`, which reads `$app/state` -- mocked here at
 * that source, so the shim itself is exercised rather than replaced. The
 * URL is what each case varies, starting from the fixture's own, so this
 * spec and the continuum sweep describe one screen.
 */
const realPage = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: realPage }));
vi.mock('$app/paths', () => ({ resolve: (route: string) => route }));
vi.mock('#lib/api.js', () => ({ apiBaseURL: () => 'https://api.example.test' }));

interface SetupOptions {
	/**
	The page's own URL -- the fixture's, which carries a token, by default.
	*/
	url?: string;
	/**
	What POST /api/portal/sign-in-address answers.
	*/
	spend?: Partial<Response> | Error;
}

const confirmed = {
	ok: true,
	status: 200,
	json: () => Promise.resolve({ signInAddress: 'new@example.test' })
} as Response;

async function setup({ url = fixture.url, spend = confirmed }: SetupOptions = {}) {
	realPage.url = new URL(url);
	const fetchSpy = vi.fn(() =>
		spend instanceof Error ? Promise.reject(spend) : Promise.resolve(spend as Response)
	);
	vi.stubGlobal('fetch', fetchSpy);
	await render(Page);
	return { fetchSpy };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

const continueButton = () => page.getByRole('button', { name: 'Continue' });

describe('confirming a changed Client sign-in address', () => {
	// ADR-0026's scanner rule: rendering the page spends nothing.
	it('spends the token on the Continue click, never on the render', async () => {
		const { fetchSpy } = await setup();

		await expect.element(continueButton()).toBeVisible();
		expect(fetchSpy).not.toHaveBeenCalled();

		await continueButton().click();

		await expect.element(page.getByText('Your sign-in address has changed')).toBeVisible();
		await expect.element(page.getByText(/new@example.test/)).toBeVisible();
		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it('says so when the link carries no token at all', async () => {
		await setup({ url: 'https://example.test/portal/confirm-sign-in-address' });

		await expect.element(page.getByRole('alert')).toHaveTextContent('This link is not complete.');
		await expect
			.element(page.getByRole('link', { name: 'Sign in to ask for a new link' }))
			.toHaveAttribute('href', '/portal/(signed-out)/login');
		expect(continueButton().elements()).toHaveLength(0);
	});

	// #2025: the address the link was mailed to, from the fragment, so the
	// screen says which address is being confirmed.
	it('names the new address in the line under the heading', async () => {
		await setup();

		await expect
			.element(
				page.getByText(
					'Confirm that you want to sign in with anne-marie.ochieng-whitfield+new-address@highland-midwifery-group.example.org from now on.'
				)
			)
			.toBeVisible();
	});

	it('falls back to a plain line when the link carries no address', async () => {
		await setup({ url: 'https://example.test/portal/confirm-sign-in-address?token=address-change-token-1' });

		await expect
			.element(page.getByText('Confirm that you want to use this address to sign in to DoulaCloud.'))
			.toBeVisible();
	});

	// #2025: a used or expired link cannot be pressed through, so Continue
	// gives way to the next step.
	it('offers the way to a new link when the link has expired or was used', async () => {
		await setup({ spend: jsonResponse({ code: 'INVALID_ARGUMENT', message: 'this link is invalid or has expired' }, 400) });

		await continueButton().click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('This link has expired or was already used.');
		await expect.element(page.getByRole('link', { name: 'Sign in to ask for a new link' })).toBeVisible();
		expect(continueButton().elements()).toHaveLength(0);
	});

	// The one collision the request endpoint deliberately does not
	// answer, surfacing where it is safe to say plainly.
	it('reports an address claimed since the link was sent', async () => {
		await setup({
			spend: {
				ok: false,
				status: 409,
				text: () => Promise.resolve(JSON.stringify({ error: 'that address already signs in' }))
			}
		});

		await continueButton().click();

		await expect.element(page.getByText('that address already signs in')).toBeVisible();
	});

	it('reports a thrown submit as a service problem', async () => {
		await setup({ spend: new Error('offline') });

		await continueButton().click();

		await expect.element(page.getByText(/problem with the service/)).toBeVisible();
	});
});
