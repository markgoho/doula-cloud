import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { fixture, session } from './page.fixture.js';

/*
 * #619's request half. The `page` this route reads and the answer its
 * own fetch gets both come from its fixture (#596), so what this spec
 * renders and what the continuum sweep measures are one description.
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

// The address the fixture says signs her in today, so the spec asserts
// on the same screen the sweep measures.
const currentAddress = session.signInAddress;
const happyPath = toApiResponder(fixture);

interface SetupOptions {
	/**
	What GET /api/portal/session answers -- the fixture's own by default.
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
		if (path !== '/api/portal/session') {
			return request instanceof Error ? Promise.reject(request) : Promise.resolve(request);
		}
		if (load === undefined) return happyPath(path);
		return load instanceof Error ? Promise.reject(load) : Promise.resolve(load);
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
		await setup({ load: jsonResponse('', 500) });

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

	// govuk-alignment.md's second recorded departure: the outcome is
	// announced in place and the form stays where it is, rather than a
	// confirmation page replacing it.
	it('announces the sent link in place, leaving the form up', async () => {
		await setup();
		await expect.element(field()).toBeVisible();

		await field().fill('new@example.test');
		await submit().click();

		await expect.element(page.getByText(/Check your email/)).toBeVisible();
		await expect.element(page.getByText(/new@example.test/)).toBeVisible();
		// The reassurance the ADR is built on, said where she can read it.
		await expect
			.element(page.getByText(new RegExp(`${currentAddress} still signs you in`)))
			.toBeVisible();
		await expect.element(field()).toBeVisible();
	});

	// The BFF names the field its refusal is about (api-design.md section
	// 7), which is what puts the message above the input rather than
	// adrift at the top of the page.
	it('puts a field-keyed BFF refusal on the field it names', async () => {
		await setup({
			request: jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'Enter your new sign-in address',
					details: { email: 'Enter your new sign-in address' }
				},
				400
			)
		});
		await expect.element(field()).toBeVisible();

		await field().fill('new@example.test');
		await submit().click();

		await expect.element(page.getByText('Enter your new sign-in address').first()).toBeVisible();
		await expect.element(field()).toHaveAttribute('aria-invalid', 'true');
	});

	it('reports a thrown submit as a service problem', async () => {
		await setup({ request: new Error('offline') });
		await expect.element(field()).toBeVisible();

		await field().fill('new@example.test');
		await submit().click();

		await expect.element(page.getByText(/problem with the service/)).toBeVisible();
	});
});
