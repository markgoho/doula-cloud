import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { toPageState } from '../../../../../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * #619's request half, on its own question page since #2045: the Change
 * on the sign-in address row. The `page` this route reads comes from its
 * fixture (#596).
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

const gotoWithOutcome = vi.hoisted(() => vi.fn());
vi.mock('#lib/outcome.js', () => ({ gotoWithOutcome, takeOutcome: vi.fn() }));

const ADDRESS_HREF = '/portal/engagements/engagement-1/sign-in-address';

async function setup(request: Response | Error = jsonResponse({}, 202)) {
	apiFetchWithSession.mockReset();
	gotoWithOutcome.mockReset();
	apiFetchWithSession.mockImplementation(() =>
		request instanceof Error ? Promise.reject(request) : Promise.resolve(request)
	);
	await render(Page);
}

const field = () => page.getByRole('textbox', { name: 'What is your new sign-in address?' });
const submit = () => page.getByRole('button', { name: 'Send the link' });

describe('the Client-portal sign-in address question', () => {
	it('asks the question as the page heading, with the send button under its field', async () => {
		await setup();

		await expect
			.element(page.getByRole('heading', { level: 1, name: 'What is your new sign-in address?' }))
			.toBeVisible();
		await expect.element(field()).toHaveAccessibleDescription(/We will send a link to it/);
		await expect.element(submit()).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Back' })).toHaveAttribute('href', ADDRESS_HREF);
		// Sign out everywhere is the summary's own section, never here.
		expect(page.getByRole('button', { name: 'Sign out of every device' }).elements()).toHaveLength(0);
	});

	it('refuses an empty address against the field, before it costs a round trip', async () => {
		await setup();

		await submit().click();

		await expect.element(field()).toHaveAccessibleDescription(/Enter your new sign-in address/);
		await expect.element(field()).toHaveAttribute('aria-invalid', 'true');
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('refuses an address with no at sign', async () => {
		await setup();

		await field().fill('not-an-address');
		await submit().click();

		await expect.element(page.getByText(/in the correct format/).first()).toBeVisible();
	});

	// The outcome is said on the summary it goes back to (#1710), with the
	// reassurance the ADR is built on.
	it('sends the link, and goes back to the address saying where to look', async () => {
		await setup();

		await field().fill(' new@example.test ');
		await submit().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/portal/sign-in-address/request', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email: 'new@example.test' })
		});
		await vi.waitFor(() =>
			expect(gotoWithOutcome).toHaveBeenCalledWith(
				ADDRESS_HREF,
				'Check your email. Open the link we sent to new@example.test to finish the change. Until you do, your current address still signs you in.'
			)
		);
	});

	// The BFF names the field its refusal is about (api-design.md section
	// 7), which is what puts the message above the input rather than
	// adrift at the top of the page.
	it('puts a field-keyed BFF refusal on the field it names, and stays', async () => {
		await setup(
			jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'Enter your new sign-in address',
					details: { email: 'Enter your new sign-in address' }
				},
				400
			)
		);

		await field().fill('new@example.test');
		await submit().click();

		await expect.element(field()).toHaveAttribute('aria-invalid', 'true');
		await expect.element(field()).toHaveAccessibleDescription(/Enter your new sign-in address/);
		expect(gotoWithOutcome).not.toHaveBeenCalled();
	});

	it('reports a thrown submit as a service problem', async () => {
		await setup(new Error('offline'));

		await field().fill('new@example.test');
		await submit().click();

		await expect.element(page.getByText(/problem with the service/)).toBeVisible();
	});
});
