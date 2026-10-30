import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { formatInstant } from '#lib/dates.js';
import { jsonResponse } from '#lib/testResponse.js';
import { captureLanding } from '#lib/testOutcome.js';
import { expectOneFramedHeading } from '#lib/components/templates/pageFrame.testing.js';
import type { PracticeWebsite } from '#lib/website.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture, undeclaredWebsite, website as fixtureWebsite } from './page.fixture.js';

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
	apiFetchWithSession
}));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const undeclared = undeclaredWebsite;

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
 * produce, and asserting on it is the behavioral assertion, not a peek
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

/*
 * A saved answer's Change (#2047). Every row's action reads "Change",
 * told apart only by the visually-hidden words it is described by -- an
 * accessible description, which no role query filters on, so this reads
 * the `aria-describedby` target the way a screen reader does.
 */
async function changeFor(changes: string) {
	// The answers arrive after mount; every saved state has this row.
	await expect.element(testPage.getByText('How Clients find you', { exact: true })).toBeVisible();
	const control = [...document.querySelectorAll<HTMLElement>('dd [aria-describedby]')].find(
		(candidate) => document.querySelector(`#${CSS.escape(candidate.getAttribute('aria-describedby')!)}`)?.textContent === changes
	);
	if (!control) throw new Error(`no Change is described by "${changes}"`);
	return testPage.elementLocator(control);
}

async function clickChange(changes: string) {
	const control = await changeFor(changes);
	await control.click();
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	pageState.url = new URL(fixture.url);
});

/*
 * MO-G15 (#2196): Getting paid sends an Owner here before it offers
 * Connect Stripe, and the save takes her back there, saying what it did,
 * rather than leaving her to find her own way back.
 */
describe('website settings screen, reached from Getting paid', () => {
	beforeEach(() => {
		pageState.url = new URL(`${fixture.url}?from=getting-paid`);
	});

	it('returns her to Getting paid once her own site is saved', async () => {
		const { puts } = await setup();
		const landing = captureLanding(goto);

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await testPage.getByLabelText('The web address of your website or social profile').fill('instagram.com/okonkwobirth');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.poll(() => landing.href).toBe('/practices/practice-1/settings/payments');
		expect(landing.message).toBe('Your website is saved. You can connect Stripe now.');
		expect(puts.at(-1)).toMatchObject({ mode: 'own', ownUrl: 'instagram.com/okonkwobirth' });
	});

	it('returns her to Getting paid once her page is published', async () => {
		await setup();
		const landing = captureLanding(goto);

		await testPage.getByRole('radio', { name: 'Publish a page for me' }).click();
		await testPage.getByLabelText('What your Practice offers').fill('Birth and postpartum support in Monroe County.');
		await testPage.getByLabelText('Your cancellation or refund policy').fill('The deposit is not refunded.');
		await testPage.getByRole('button', { name: 'Continue' }).click();
		await testPage.getByRole('button', { name: 'Publish page' }).click();

		await expect.poll(() => landing.href).toBe('/practices/practice-1/settings/payments');
		expect(landing.message).toBe('Your page is published. You can connect Stripe now.');
	});

	it('stays on the questions when the save is refused', async () => {
		await setup({ put: () => jsonResponse({ message: 'Something went wrong' }, 500) });

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await testPage.getByLabelText('The web address of your website or social profile').fill('instagram.com/okonkwobirth');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText('Something went wrong').first()).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('website settings screen, reached any other way', () => {
	it('stays on the saved answers, with no navigation', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'I have my own website or social profile' }).click();
		await testPage.getByLabelText('The web address of your website or social profile').fill('instagram.com/okonkwobirth');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText('Last changed by Maya Chen on Aug 29, 2026')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
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

		await expect.element(testPage.getByText('Last changed by Maya Chen on Aug 29, 2026')).toBeVisible();
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

		// #1968: an Owner's Publish page is a button she can press, never a
		// disabled one.
		await expect.element(testPage.getByRole('button', { name: 'Publish page' })).toBeEnabled();
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
			.element(testPage.getByText('Last changed by Maya Chen on Aug 29, 2026'))
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
		// Computed rather than a hardcoded "Aug 1, 2026": the fixture's
		// `updatedAt` is UTC midnight, an instant the screen reads in the
		// runner's local timezone, which can read back a day earlier --
		// this is still a rendered form of the fixture's value (#596),
		// through the same dates.ts formatter the component uses (#1958).
		const updatedOn = formatInstant(fixtureWebsite.updatedAt);
		await expect
			.element(testPage.getByText(`Last changed by ${fixtureWebsite.updatedBy} on ${updatedOn}`))
			.toBeVisible();

		await clickChange('how Clients find you');
		await expect.element(testPage.getByRole('radio', { name: 'Publish a page for me' })).toBeChecked();
		await expect.element(testPage.getByLabelText('What your Practice offers')).toBeVisible();
	});

	// #2047: a Change per row, and each leads to that one question.
	it('changes only the cancellation policy, and publishes the rest as it was', async () => {
		const { puts } = await setup({ website: fixtureWebsite });

		await clickChange('the cancellation policy');
		await expect.element(testPage.getByLabelText('Your cancellation or refund policy')).toBeVisible();
		await expect.element(testPage.getByLabelText('What your Practice offers')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('radio')).not.toBeInTheDocument();

		await testPage.getByLabelText('Your cancellation or refund policy').fill('Two weeks notice.');
		await testPage.getByRole('button', { name: 'Continue' }).click();
		await testPage.getByRole('button', { name: 'Publish page' }).click();
		expect(puts.at(-1)).toEqual({
			mode: 'hosted',
			serviceDescription: fixtureWebsite.serviceDescription,
			cancellationPolicy: 'Two weeks notice.'
		});
	});

	it('changes only what she offers', async () => {
		await setup({ website: fixtureWebsite });

		await clickChange('what you offer');
		await expect.element(testPage.getByLabelText('What your Practice offers')).toBeVisible();
		await expect.element(testPage.getByLabelText('Your cancellation or refund policy')).not.toBeInTheDocument();
	});

	it('sends a change of business name to the Practice name screen', async () => {
		await setup({ website: fixtureWebsite });

		await expect.element(await changeFor('the business name')).toHaveAttribute('href', '/practices/practice-1/settings/name');
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
			.element(testPage.getByText('Being published.', { exact: false }))
			.toBeVisible();
		// Not a link until we have loaded it ourselves.
		await expect
			.element(testPage.getByRole('link', { name: 'https://doula.cloud/p/rochester-doulas' }))
			.not.toBeInTheDocument();
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
			.element(testPage.getByRole('link', { name: 'https://doula.cloud/p/rochester-doulas' }))
			.toHaveAttribute('href', 'https://doula.cloud/p/rochester-doulas');
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
		await clickChange('your page');
		await testPage
			.getByLabelText('The web address of your website or social profile')
			.fill('https://example.com/abandoned');
		await testPage.getByRole('button', { name: 'Cancel' }).click();
		await expect
			.element(testPage.getByRole('link', { name: 'https://rochesterdoulas.com' }))
			.toHaveAttribute('href', 'https://rochesterdoulas.com');
		// Opened again, the question starts from what is saved.
		await clickChange('your page');
		await expect
			.element(testPage.getByLabelText('The web address of your website or social profile'))
			.toHaveValue('https://rochesterdoulas.com');
	});

	it('tells a non-Owner she cannot change it and offers her no button at all (#1968)', async () => {
		await setup({ roles: ['admin'] });

		await expect
			.element(
				testPage.getByText('Only a Practice Owner can change this. Ask an Owner if it needs updating.')
			)
			.toBeVisible();
		await expect
			.element(
				testPage.getByText('Not answered yet. A Practice Owner answers this before Stripe can be connected.')
			)
			.toBeVisible();
		await expect
			.element(testPage.getByRole('radio', { name: 'Publish a page for me' }))
			.not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button')).not.toBeInTheDocument();
	});

	it('keeps the non-Owner Notice inside the page frame, with the heading (#1971)', async () => {
		await setup({ roles: ['admin'] });

		await expectOneFramedHeading('Your website');
		const notice = testPage.getByText('Only a Practice Owner can change this. Ask an Owner if it needs updating.');
		await expect.element(notice).toBeVisible();
		// The frame is a layout primitive with no role, so `closest` is the
		// only way to ask (svelte-tests.md's third exception, the reason
		// `expectOneFramedHeading` gives). Outside `center-l`, the Notice has
		// no gutter and runs to the window's edges at 320px.
		expect(notice.element().closest('center-l')).not.toBeNull();
	});

	it('shows a non-Owner a published page with no Change or Publish page button (#1968)', async () => {
		await setup({ roles: ['doula'], website: fixtureWebsite });

		await expect.element(testPage.getByRole('link', { name: fixtureWebsite.pageUrl })).toBeVisible();
		await expect.element(testPage.getByRole('button')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: 'Change' })).not.toBeInTheDocument();
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
