import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toPageState } from '../../../../../routeFixture.js';
import { fixture, session } from './page.fixture.js';

/*
 * #619's request half. The `page` this route reads comes from its own
 * fixture (#596), so what this spec renders and what the continuum sweep
 * measures are one description.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

function jsonResponse(body: unknown, status = 200): Response {
	return {
		ok: status < 400,
		status,
		json: () => Promise.resolve(body),
		text: () => Promise.resolve(JSON.stringify(body))
	} as Response;
}

function refusal(status: number, message: string): Response {
	return { ok: false, status, text: () => Promise.resolve(message) } as Response;
}

// The address the fixture says signs her in today, so the spec asserts
// on the same screen the sweep measures.
const currentAddress = session.signInAddress;
const sessionBody = jsonResponse(session);

interface SetupOptions {
	/**
	What GET /api/portal/session answers -- the current address, or a failure.
	*/
	load?: Response | Error;
	/**
	What POST /api/portal/sign-in-address/request answers.
	*/
	request?: Response | Error;
}

async function setup({ load, request = jsonResponse({}, 202) }: SetupOptions = {}) {
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation((path: string) => {
		const answer = path === '/api/portal/session' ? (load ?? sessionBody) : request;
		return answer instanceof Error ? Promise.reject(answer) : Promise.resolve(answer);
	});
	await render(Page);
}

const field = () => page.getByLabelText('New sign-in address');
const submit = () => page.getByRole('button', { name: 'Send the link' });

describe('the Client-portal sign-in address screen', () => {
	it('shows the address that signs her in today', async () => {
		await setup();

		await expect.element(page.getByText(new RegExp(currentAddress))).toBeVisible();
		await expect.element(field()).toBeVisible();
	});

	it('reports the page failing to load without pretending there is a form', async () => {
		await setup({ load: refusal(500, '') });

		await expect.element(page.getByText(/problem with the service/)).toBeVisible();
	});

	it('reports a thrown load the same way', async () => {
		await setup({ load: new Error('offline') });

		await expect.element(page.getByText(/problem with the service/)).toBeVisible();
	});

	it('refuses an empty address before it costs a round trip', async () => {
		await setup();
		await expect.element(field()).toBeVisible();

		await submit().click();

		await expect.element(page.getByText('Enter your new sign-in address').first()).toBeVisible();
		expect(apiFetchWithSession).not.toHaveBeenCalledWith(
			'/api/portal/sign-in-address/request',
			expect.anything()
		);
	});

	it('refuses an address with no at sign', async () => {
		await setup();
		await expect.element(field()).toBeVisible();

		await field().fill('not-an-address');
		await submit().click();

		await expect.element(page.getByText(/in the correct format/).first()).toBeVisible();
	});

	// The confirmation pattern: the form goes away, because the next act
	// is in her inbox and a second submit would supersede the link she is
	// about to open.
	it('replaces the form with a check-your-email confirmation', async () => {
		await setup();
		await expect.element(field()).toBeVisible();

		await field().fill('new@example.test');
		await submit().click();

		await expect.element(page.getByText('Check your email')).toBeVisible();
		await expect.element(page.getByText(/new@example.test/)).toBeVisible();
		// The reassurance the ADR is built on, said where she can read it.
		await expect
			.element(page.getByText(new RegExp(`${currentAddress} still signs you in`)))
			.toBeVisible();
	});

	it('shows the BFF refusal', async () => {
		await setup({ request: refusal(400, JSON.stringify({ error: 'email is required' })) });
		await expect.element(field()).toBeVisible();

		await field().fill('new@example.test');
		await submit().click();

		await expect.element(page.getByText('email is required').first()).toBeVisible();
	});

	it('reports a thrown submit as a service problem', async () => {
		await setup({ request: new Error('offline') });
		await expect.element(field()).toBeVisible();

		await field().fill('new@example.test');
		await submit().click();

		await expect.element(page.getByText(/problem with the service/)).toBeVisible();
	});
});
