import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientMatch } from '#lib/client.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture } from './page.fixture.js';

/*
 * The `page` this screen reads comes from the route's own fixture
 * (#596), the same installation, through the same `toPageState`, as
 * `route-continuum.svelte.spec.ts`. `vi.mock` is hoisted above every
 * import, so `pageState` is declared empty and filled once the imports
 * have run.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const { practiceId } = fixture.params;
// The load's own `data.isContractor` -- what `+page.ts` hands the route
// as a prop, not `page.data` -- so it is not part of `toPageState` and is
// installed by `setup()` below instead, per test.
const fixtureData = fixture.props?.data as { isContractor: boolean };

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const adaMatch: ClientMatch = {
	id: 'client-2',
	givenName: 'Ada',
	familyName: 'Byron',
	preferredName: '',
	email: 'ada.byron@example.com',
	phone: '555-0100',
	addressLine1: '',
	addressLine2: '',
	addressLocality: '',
	addressRegion: '',
	addressPostalCode: '',
	dateOfBirth: '1815-12-10',
	engagements: [{ engagementId: 'engagement-1', kind: 'birth', status: 'active', createdAt: '2024-01-01' }]
};

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

// The generated `data` prop merges practices/[practiceId]/+layout.ts's
// `session` (#835) into +page.ts's own ContractorGate, the way SvelteKit
// really does at runtime -- this route never reads it, but rendering the
// component directly still needs the full merged shape.
async function setup(isContractor = fixtureData.isContractor) {
	await render(Page, {
		data: {
			isContractor,
			isOwner: false,
			session: { practiceId, staffId: 'staff-1', practiceName: 'Riverside Doula Collective', roles: [], isContractor }
		}
	});
}

function requestUrl(callIndex = 0): string {
	return apiFetchWithSession.mock.calls[callIndex][0] as string;
}

describe('the search that fronts intake (#498)', () => {
	it('refuses a search with nothing typed, naming the reason and moving focus to it', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Enter a name, date of birth, email or phone to search' }))
			.toBeVisible();
		expect(document.activeElement?.textContent).toContain('There is a problem');
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('runs on name, date of birth, email and phone -- whatever the staff member has', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [] }));
		await setup();

		await testPage.getByLabelText('Name').fill('Ada');
		await testPage.getByLabelText('Month').fill('12');
		await testPage.getByLabelText('Day').fill('10');
		await testPage.getByLabelText('Year').fill('1815');
		await testPage.getByLabelText('Email').fill('ada@example.com');
		await testPage.getByLabelText('Phone').fill('555-0100');
		await testPage.getByRole('button', { name: 'Search' }).click();

		expect(requestUrl()).toBe(
			`/api/practices/${practiceId}/clients/search?name=Ada&dateOfBirth=1815-12-10&email=ada%40example.com&phone=555-0100`
		);
	});

	it('prints each result\'s name and history unrestricted, linking to the Client detail hub (#494)', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [adaMatch] }));
		await setup();

		await testPage.getByLabelText('Name').fill('Ada');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect.element(testPage.getByRole('heading', { level: 2, name: '1 match' })).toBeVisible();
		await expect.element(testPage.getByRole('heading', { level: 3, name: 'Ada Byron' })).toBeVisible();
		await expect.element(testPage.getByText('1815-12-10')).toBeVisible();
		await expect.element(testPage.getByText('Birth · active')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "Open Ada Byron's record" }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/client-2`);
		// A client-side result set moves no focus on its own -- the results
		// heading takes it, so a screen reader announces the count.
		expect(document.activeElement?.id).toBe('client-search-results');
	});

	it('offers to start intake on an empty result set, carrying the typed name (#497)', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [] }));
		await setup();

		await testPage.getByLabelText('Name').fill('Nadia Haddad');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect.element(testPage.getByRole('heading', { level: 2, name: 'No matches' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Add a new Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/new?name=Nadia%20Haddad`);
	});

	/*
	 * A staff member with nothing but a phone number searches on it and
	 * finds no one. Carrying only the name would lose the one thing she
	 * had -- ADR-0017's "searching costs nothing" holds for every key.
	 */
	it('carries a phone-only search into intake', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [] }));
		await setup();

		await testPage.getByLabelText('Phone').fill('555-0100');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Add a new Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/new?phone=555-0100`);
	});

	it('carries every key that was typed, not only the name', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [] }));
		await setup();

		await testPage.getByLabelText('Name').fill('Nadia');
		await testPage.getByLabelText('Month').fill('2');
		await testPage.getByLabelText('Day').fill('11');
		await testPage.getByLabelText('Year').fill('1994');
		await testPage.getByLabelText('Email').fill('nadia@example.com');
		await testPage.getByLabelText('Phone').fill('555-0100');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Add a new Client' }))
			.toHaveAttribute(
				'href',
				`/practices/${practiceId}/clients/new?name=Nadia&dateOfBirth=1994-02-11&email=nadia%40example.com&phone=555-0100`
			);
	});

	/*
	 * #807: the same Postel's Law tolerance intake accepts -- one or two
	 * digits for the month and the day, two or four for the year -- so a
	 * date typed the way it is remembered still reaches the endpoint as
	 * the "YYYY-MM-DD" `client.SearchHandler` reads.
	 */
	it('accepts a one-digit month and a two-digit year, and composes both ways', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [] }));
		await setup();

		await testPage.getByLabelText('Month').fill('3');
		await testPage.getByLabelText('Day').fill('12');
		await testPage.getByLabelText('Year').fill('88');
		await testPage.getByRole('button', { name: 'Search' }).click();

		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients/search?dateOfBirth=1988-03-12`);
		await expect
			.element(testPage.getByRole('link', { name: 'Add a new Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/new?dateOfBirth=1988-03-12`);
	});

	/*
	 * The boxes feed two readers -- the search, and the link a miss offers
	 * into intake, which is rendered from whatever is typed right now. A
	 * date snapshotted at submit would leave that link carrying the
	 * previous one beside the current name.
	 */
	it('carries the date currently in the boxes into intake, not the one the last search ran on', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [] }));
		await setup();

		await testPage.getByLabelText('Month').fill('2');
		await testPage.getByLabelText('Day').fill('11');
		await testPage.getByLabelText('Year').fill('1994');
		await testPage.getByRole('button', { name: 'Search' }).click();
		await expect
			.element(testPage.getByRole('link', { name: 'Add a new Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/new?dateOfBirth=1994-02-11`);

		await testPage.getByLabelText('Year').fill('1995');

		await expect
			.element(testPage.getByRole('link', { name: 'Add a new Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/new?dateOfBirth=1995-02-11`);
	});

	it('refuses a date that is not one, announcing it once and linking to the box that has to change', async () => {
		await setup();

		await testPage.getByLabelText('Month').fill('13');
		await testPage.getByLabelText('Day').fill('12');
		await testPage.getByLabelText('Year').fill('1988');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Date of birth must be a real date' }))
			.toHaveAttribute('href', '#client-search-date-of-birth-month');
		// Announced once, from the group's own fieldset -- and the month is
		// the box marked, not all three.
		const group = testPage.getByRole('group', { name: 'Date of birth' });
		await expect.element(group.getByRole('alert')).toHaveTextContent('Date of birth must be a real date');
		await expect.element(testPage.getByLabelText('Month')).toHaveAttribute('aria-invalid', 'true');
		await expect.element(testPage.getByLabelText('Day')).not.toHaveAttribute('aria-invalid', 'true');
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('surfaces a refusal as a readable message rather than a raw crash -- e.g. a contractor\'s 403 (#501)', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse('a contractor doula does not search for clients at a practice she contracts for', 403)
		);
		await setup();

		await testPage.getByLabelText('Name').fill('Ada');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect
			.element(testPage.getByText('a contractor doula does not search for clients at a practice she contracts for'))
			.toBeVisible();
	});
});

describe("the contractor's Add a Client door (#501, ADR-0017)", () => {
	it('shows the explainer instead of the search screen, and never calls the search API', async () => {
		await setup(true);

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Add a Client' })).toBeVisible();
		await expect.element(testPage.getByLabelText('Name')).not.toBeInTheDocument();
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('names the actual mechanism -- work arrives as an Offer', async () => {
		await setup(true);

		await expect.element(testPage.getByText('reaches you as an Offer', { exact: false })).toBeVisible();
	});

	it('links to plain /signup, keyboard-reachable as an ordinary link', async () => {
		await setup(true);

		await expect
			.element(testPage.getByRole('link', { name: 'Set up a Practice' }))
			.toHaveAttribute('href', '/signup');
	});

	it('leaves the search screen (#498) unchanged for an Owner, Admin or employee Doula', async () => {
		await setup(false);

		await expect.element(testPage.getByRole('heading', { level: 1, name: fixture.readyText })).toBeVisible();
		await expect.element(testPage.getByLabelText('Name')).toBeVisible();
	});
});
