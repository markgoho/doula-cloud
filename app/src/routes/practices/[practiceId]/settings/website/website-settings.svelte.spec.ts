import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeWebsite } from '#lib/website.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture, website as fixtureWebsite } from './page.fixture.js';

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params this spec installs and the params the continuum sweep installs
 * are one description. `vi.mock` is hoisted above every import, so the
 * object is declared empty here and filled from the fixture once the
 * imports have run.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

const undeclared = {
	mode: 'undeclared',
	ownUrl: '',
	serviceDescription: '',
	cancellationPolicy: '',
	updatedBy: '',
	updatedAt: '',
	pageState: '',
	pageCheckedAt: '',
	pageCheckDetail: '',
	pageUrl: ''
};

interface SetupOptions {
	// `PracticeWebsite` alongside the bare `Record` so the fixture's own
	// export (#596) can be handed in directly -- most tests still pass a
	// partial/malformed body to exercise a degraded response, which is
	// exactly what the `Record` arm is for.
	website?: PracticeWebsite | Record<string, unknown>;
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
	// The Practice's name and the caller's roles come off page.data.session
	// (#835), not a fetch this mock has to answer.
	pageState.data = {
		session: { practiceId: 'practice-1', practiceName: 'Rochester Doulas', roles, isContractor: false }
	};
	const puts: Record<string, unknown>[] = [];
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'PUT') {
			const body = JSON.parse(String(init.body)) as Record<string, unknown>;
			puts.push(body);
			return Promise.resolve(
				put
					? put()
					: jsonResponse({ ...undeclared, ...body, updatedBy: 'Maya Chen', updatedAt: '2026-08-29T14:30:00Z' })
			);
		}
		return Promise.resolve(jsonResponse(website, websiteOk ? 200 : 403));
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

		// Twice, and that is the pattern (#467): once in the error summary at
		// the top as a link to the group, and once beside the group itself.
		// The two are one string rendered twice, so they cannot drift.
		await expect
			.element(testPage.getByRole('heading', { name: 'There is a problem' }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Choose how Clients will find you online' }))
			.toHaveAttribute('href', '#website-mode-own');
		await expect
			.element(testPage.getByText('Choose how Clients will find you online').last())
			.toBeVisible();
	});

	it('asks for a web address when she has her own site, and refuses an empty one', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await expect
			.element(testPage.getByLabelText('The web address of your website or social profile'))
			.toBeVisible();

		await testPage.getByRole('button', { name: 'Save' }).click();
		await expect
			.element(
				testPage.getByRole('link', { name: 'Enter the web address of your website or social profile' })
			)
			.toHaveAttribute('href', '#website-own-url');
		await expect
			.element(testPage.getByText('Enter the web address of your website or social profile').last())
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
					400
				)
		});

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await testPage.getByLabelText('The web address of your website or social profile').fill('coming soon');
		await testPage.getByRole('button', { name: 'Save' }).click();

		// A field error the *server* found, now carried into the summary as
		// well as beside the box -- the server's own words, in both places,
		// because it is the only thing that knows what was wrong with it.
		await expect
			.element(
				testPage.getByRole('link', {
					name: 'Enter a web address in the correct format, like https://example.com/your-practice'
				})
			)
			.toHaveAttribute('href', '#website-own-url');
		await expect
			.element(
				testPage
					.getByText('Enter a web address in the correct format, like https://example.com/your-practice')
					.last()
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
		await expect.element(testPage.getByText('You have 1 character too many')).toBeVisible();

		await testPage.getByLabelText('Your cancellation or refund policy').fill('Two weeks.');
		await expect.element(testPage.getByText('You have 490 characters remaining')).toBeVisible();
		await testPage.getByRole('button', { name: 'Continue' }).click();
		await expect
			.element(testPage.getByRole('link', { name: 'Shorten this to 500 characters or fewer' }))
			.toHaveAttribute('href', '#serviceDescription-input');
		await expect
			.element(testPage.getByText('Shorten this to 500 characters or fewer').last())
			.toBeVisible();
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

	// The already-answered/hosted/live state this test needs is exactly
	// what the fixture describes (#596), so the "saved" screen it renders
	// on is the fixture's own -- including the referral-link content #530
	// put there, not a friendlier restatement of the same fact.
	it('shows a Practice that has already answered what it says, and lets her change her mind', async () => {
		await setup({ website: fixtureWebsite });

		// The fixture's Practice pasted the same referral URL into both
		// free-text fields (#530), so the description and the
		// cancellation policy read identically -- DOM order is the only
		// thing that says which `<dd>` is which, service description
		// first, same as the screen's own field order.
		await expect.element(testPage.getByText(fixtureWebsite.serviceDescription).first()).toBeVisible();
		// Computed rather than a hardcoded "August 1, 2026": the fixture's
		// `updatedAt` is UTC midnight, and the screen's own
		// `toLocaleDateString` renders it in the runner's local timezone,
		// which can read back a day earlier -- this is still a rendered
		// form of the fixture's value (#596), just derived the same way
		// the component derives it rather than guessed.
		const updatedOn = new Date(fixtureWebsite.updatedAt).toLocaleDateString('en-US', {
			day: 'numeric',
			month: 'long',
			year: 'numeric'
		});
		await expect
			.element(testPage.getByText(`Last changed by ${fixtureWebsite.updatedBy} on ${updatedOn}`))
			.toBeVisible();

		await testPage.getByRole('button', { name: 'Change' }).click();
		await expect.element(testPage.getByLabelText('What your Practice offers')).toBeVisible();
	});

	/* #443. Three states, and the middle one is the point: a page nothing
	   has confirmed reads as not-yet, never as a success, because the
	   build can fail and report nothing at all. */
	it('says her page is not confirmed while it waits for its deploy', async () => {
		await setup({
			website: {
				...undeclared,
				mode: 'hosted',
				serviceDescription: 'Birth support in Monroe County.',
				cancellationPolicy: 'Two weeks notice.',
				pageState: 'pending',
				pageUrl: 'https://doula.cloud/p/rochester-doulas'
			}
		});

		await expect
			.element(testPage.getByText('Your page is being published', { exact: false }))
			.toBeVisible();
	});

	it('shows her the address once we have loaded the page ourselves', async () => {
		await setup({
			website: {
				...undeclared,
				mode: 'hosted',
				serviceDescription: 'Birth support in Monroe County.',
				cancellationPolicy: 'Two weeks notice.',
				pageState: 'live',
				pageUrl: 'https://doula.cloud/p/rochester-doulas'
			}
		});

		await expect
			.element(testPage.getByText('Your page is live at https://doula.cloud/p/rochester-doulas'))
			.toBeVisible();
	});

	it('tells her why her page failed, in words she can act on', async () => {
		await setup({
			website: {
				...undeclared,
				mode: 'hosted',
				serviceDescription: 'Birth support in Monroe County.',
				cancellationPolicy: 'Two weeks notice.',
				pageState: 'failed',
				pageCheckDetail: 'the site answered 404 for this page',
				pageUrl: 'https://doula.cloud/p/rochester-doulas'
			}
		});

		await expect
			.element(testPage.getByText('the site answered 404 for this page', { exact: false }))
			.toBeVisible();
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
		await setup({ put: () => jsonResponse('only a Practice Owner can do that', 403) });

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await testPage
			.getByLabelText('The web address of your website or social profile')
			.fill('https://rochesterdoulas.com');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText(/only a Practice Owner can do that/)).toBeVisible();
	});
});
