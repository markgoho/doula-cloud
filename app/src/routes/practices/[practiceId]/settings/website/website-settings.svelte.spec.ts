import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: {
		params: { practiceId: 'practice-1' },
		url: new URL('https://test.local/practices/practice-1/settings/website')
	}
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native Response.ok property this mock stands in for
function jsonResponse(body: unknown, ok = true, status = 200): Response {
	return {
		ok,
		status,
		text: () => Promise.resolve(JSON.stringify(body)),
		json: () => Promise.resolve(body)
	} as Response;
}

const undeclared = {
	mode: 'undeclared',
	ownUrl: '',
	serviceDescription: '',
	cancellationPolicy: '',
	updatedBy: '',
	updatedAt: ''
};

interface SetupOptions {
	website?: Record<string, unknown>;
	roles?: string[];
	websiteOk?: boolean;
	put?: () => Response;
}

/*
 * One setup per test. It returns `puts` -- every body the screen sent --
 * because "what did she publish?" is the output this screen exists to
 * produce, and asserting on it is the behavioural assertion, not a peek
 * at an internal collaborator.
 */
async function setup({ website = undeclared, roles = ['owner'], websiteOk = true, put }: SetupOptions = {}) {
	const puts: Record<string, unknown>[] = [];
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (path.endsWith('/session')) {
			return Promise.resolve(jsonResponse({ practiceName: 'Rochester Doulas', roles }));
		}
		if (init?.method === 'PUT') {
			const body = JSON.parse(String(init.body)) as Record<string, unknown>;
			puts.push(body);
			return Promise.resolve(
				put
					? put()
					: jsonResponse({ ...undeclared, ...body, updatedBy: 'Maya Chen', updatedAt: '2026-08-29T14:30:00Z' })
			);
		}
		return Promise.resolve(jsonResponse(website, websiteOk));
	});
	await render(Page, {});
	return { puts };
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe('website settings screen', () => {
	it('asks a Practice that has not answered which of the two she wants', async () => {
		await setup();

		await expect.element(testPage.getByText('How will Clients and Stripe find you online?')).toBeVisible();
		await expect
			.element(testPage.getByRole('radio', { name: 'I have my own website or social profile' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('radio', { name: 'Publish a page for me' })).toBeVisible();
	});

	it('refuses to go on until she has chosen', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText('Choose how Clients will find you online')).toBeVisible();
	});

	it('asks for a web address when she has her own site, and refuses an empty one', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await expect
			.element(testPage.getByLabelText('The web address of your website or social profile'))
			.toBeVisible();

		await testPage.getByRole('button', { name: 'Save' }).click();
		await expect
			.element(testPage.getByText('Enter the web address of your website or social profile'))
			.toBeVisible();
	});

	it('sends a social profile straight through, and shows who declared it', async () => {
		const { puts } = await setup();

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await testPage
			.getByLabelText('The web address of your website or social profile')
			.fill('facebook.com/RochesterDoulas');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText('Last changed by Maya Chen on August 29, 2026')).toBeVisible();
		expect(puts.at(-1)).toMatchObject({ mode: 'own', ownUrl: 'facebook.com/RochesterDoulas' });
	});

	it('puts a malformed address the server refused beside the box it is about', async () => {
		await setup({
			put: () =>
				jsonResponse(
					{
						code: 'INVALID_ARGUMENT',
						message: 'invalid request body',
						details: { ownUrl: 'Enter a web address in the correct format, like https://example.com/your-practice' }
					},
					false,
					400
				)
		});

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await testPage.getByLabelText('The web address of your website or social profile').fill('coming soon');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect
			.element(
				testPage.getByText('Enter a web address in the correct format, like https://example.com/your-practice')
			)
			.toBeVisible();
	});

	it('asks for exactly two things when she wants a page published', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Publish a page for me' }).click();

		await expect.element(testPage.getByLabelText('What your Practice offers')).toBeVisible();
		await expect.element(testPage.getByLabelText('Your cancellation or refund policy')).toBeVisible();
		await expect
			.element(testPage.getByLabelText('The web address of your website or social profile'))
			.not.toBeInTheDocument();
	});

	it('counts the budget down as she types, and says so when she is over it', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Publish a page for me' }).click();
		await expect
			.element(testPage.getByText('You have 500 characters remaining').first())
			.toBeVisible();

		await testPage.getByLabelText('What your Practice offers').fill('a'.repeat(501));
		await expect.element(testPage.getByText('You have 1 characters too many')).toBeVisible();

		await testPage.getByLabelText('Your cancellation or refund policy').fill('Two weeks.');
		await expect.element(testPage.getByText('You have 490 characters remaining')).toBeVisible();
		await testPage.getByRole('button', { name: 'Continue' }).click();
		await expect.element(testPage.getByText('Shorten this to 500 characters or fewer')).toBeVisible();
	});

	it('shows her the assembled page before it goes live, and publishes only when she says so', async () => {
		const { puts } = await setup();

		await testPage.getByRole('radio', { name: 'Publish a page for me' }).click();
		await testPage.getByLabelText('What your Practice offers').fill('Birth and postpartum support.');
		await testPage.getByLabelText('Your cancellation or refund policy').fill('Two weeks notice.');
		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect.element(testPage.getByText('Check your page before you publish it')).toBeVisible();
		await expect.element(testPage.getByText('Rochester Doulas')).toBeVisible();
		await expect.element(testPage.getByText('Birth and postpartum support.')).toBeVisible();
		expect(puts).toHaveLength(0);

		await testPage.getByRole('button', { name: 'Publish page' }).click();
		expect(puts.at(-1)).toMatchObject({
			mode: 'hosted',
			serviceDescription: 'Birth and postpartum support.',
			cancellationPolicy: 'Two weeks notice.'
		});
	});

	it('lets a half-written page be abandoned for her own site without a dead button', async () => {
		const { puts } = await setup();

		await testPage.getByRole('radio', { name: 'Publish a page for me' }).click();
		await testPage.getByLabelText('What your Practice offers').fill('a'.repeat(501));
		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await testPage
			.getByLabelText('The web address of your website or social profile')
			.fill('rochesterdoulas.com');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect
			.element(testPage.getByText('Last changed by Maya Chen on August 29, 2026'))
			.toBeVisible();
		expect(puts.at(-1)).toEqual({ mode: 'own', ownUrl: 'rochesterdoulas.com' });
	});

	it('lets her go back from the check screen to change an answer', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Publish a page for me' }).click();
		await testPage.getByLabelText('What your Practice offers').fill('Birth support.');
		await testPage.getByLabelText('Your cancellation or refund policy').fill('Two weeks notice.');
		await testPage.getByRole('button', { name: 'Continue' }).click();
		await testPage.getByRole('button', { name: 'Back' }).click();

		await expect.element(testPage.getByLabelText('What your Practice offers')).toBeVisible();
	});

	it('shows a Practice that has already answered what it says, and lets her change her mind', async () => {
		await setup({
			website: {
				mode: 'hosted',
				ownUrl: '',
				serviceDescription: 'Birth support in Monroe County.',
				cancellationPolicy: 'Two weeks notice.',
				updatedBy: 'Maya Chen',
				updatedAt: '2026-08-29T14:30:00Z'
			}
		});

		await expect.element(testPage.getByText('Birth support in Monroe County.')).toBeVisible();
		await expect.element(testPage.getByText('Last changed by Maya Chen on August 29, 2026')).toBeVisible();

		await testPage.getByRole('button', { name: 'Change' }).click();
		await expect.element(testPage.getByLabelText('What your Practice offers')).toBeVisible();
	});

	it('lets a Practice with her own site abandon a change and keep what she had', async () => {
		await setup({
			website: {
				mode: 'own',
				ownUrl: 'https://rochesterdoulas.com',
				serviceDescription: '',
				cancellationPolicy: '',
				updatedBy: 'Maya Chen',
				updatedAt: '2026-08-29T14:30:00Z'
			}
		});

		await expect.element(testPage.getByText('https://rochesterdoulas.com')).toBeVisible();
		await testPage.getByRole('button', { name: 'Change' }).click();
		await testPage.getByRole('button', { name: 'Cancel' }).click();
		await expect.element(testPage.getByText('https://rochesterdoulas.com')).toBeVisible();
	});

	it('tells a Doula she cannot change it and gives her no working button', async () => {
		await setup({ roles: ['doula'] });

		await expect
			.element(
				testPage.getByText('Only a Practice Owner can change this. Ask an Owner if it needs updating.')
			)
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeDisabled();
	});

	it('says so when the answer cannot be loaded at all', async () => {
		await setup({ website: { code: 'INTERNAL_ERROR', message: 'internal error' }, websiteOk: false });

		await expect.element(testPage.getByText(/internal error/)).toBeVisible();
	});

	it('reports a refusal that names no field', async () => {
		await setup({ put: () => jsonResponse('only a Practice Owner can do that', false, 403) });

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await testPage
			.getByLabelText('The web address of your website or social profile')
			.fill('https://rochesterdoulas.com');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText(/only a Practice Owner can do that/)).toBeVisible();
	});
});
