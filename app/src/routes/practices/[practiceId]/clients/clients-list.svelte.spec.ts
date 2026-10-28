import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts. This
// route's ListPage (#491) also needs the primitives registered, not just
// their CSS: <center-l max="none"> only lifts the default var(--measure) cap
// via the custom element's own attribute handling, and an unregistered
// center-l never runs it, leaving every DataTable narrower than its floor.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toApiResponder, toPageState } from '../../../routeFixture.js';
import { clients, fixture } from './page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The Clients list's content and the `page` it reads both come from the
 * route's own fixture (#596), so what this spec asserts on and what the
 * continuum sweep measures are one description. `vi.mock` is hoisted
 * above every import, so the object is declared empty here and filled
 * from the fixture once the imports have run -- the route reads `page`
 * inside its own functions rather than destructuring it at module scope,
 * so the later write is seen. Same installation, through the same
 * `toPageState`, as `route-continuum.svelte.spec.ts`.
 *
 * `pageState.url` ends up a real `URL`, whose `searchParams` getter
 * returns the same `URLSearchParams` instance every read (WHATWG), so a
 * test can set `all` on it directly before `render()` -- mutated before
 * render, not mid-test, since the mock is not Svelte-reactive.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const { practiceId } = fixture.params;

function textResponse(body: string): Response {
	return jsonResponse(body, 403);
}

function requestUrl(callIndex = 0): string {
	return apiFetchWithSession.mock.calls[callIndex][0] as string;
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	pageState.url.searchParams.delete('all');
});

// A `response` left undefined answers every fetch from the fixture's own
// `respond()`, per request rather than once -- the same reason
// `toApiResponder` exists. A test that needs different content passes its
// own `Response` and gets it on every call, which is what every "Load
// more"/failure case below still wants.
interface Caller {
	isContractor?: boolean;
	// What `+page.ts` read with `all=true` (#1609).
	hasAnyClient?: boolean;
}

async function setup(response?: Response, { isContractor = false, hasAnyClient = true }: Caller = {}) {
	// DataTable's own content floor (#508) shows its record cards below
	// 55rem, and this file's assertions are about the <table> specifically.
	// A desktop window is past it (#2022), so the <table> renders.
	await testPage.viewport(1440, 900);
	if (response) {
		apiFetchWithSession.mockResolvedValue(response);
	} else {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
	}
	// The generated `data` prop merges practices/[practiceId]/+layout.ts's
	// `session` (#835) into +page.ts's own ContractorGate, the way
	// SvelteKit really does at runtime -- this route never reads it, but
	// rendering the component directly still needs the full merged shape.
	await render(Page, {
		data: {
			isContractor,
			isOwner: false,
			hasAnyClient: Promise.resolve(hasAnyClient),
			session: { practiceId, staffId: 'staff-1', practiceName: 'Riverside Doula Collective', roles: [], isContractor }
		}
	});
}

describe('clients list screen', () => {
	it('sends "Find or add a Client" to the search that fronts intake, not straight to intake (#498, #539)', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/search`);
	});

	// #1609: at a Practice that holds no Client record the search can find
	// nobody, so the link opens the name question and names only that
	// errand. Which one it is comes from +page.ts's `all=true` count; the
	// rows here are the filtered list and decide nothing.
	it('opens the name question as "Add a Client" at a Practice with no Client record', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: false });

		await expect
			.element(testPage.getByRole('link', { name: 'Add a Client', exact: true }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/new?from=clients`);
		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.not.toBeInTheDocument();
	});

	it('keeps the search when the filtered list is empty but the Practice holds a Client with no work', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: true });

		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/search`);
	});

	// The Portal invite tests moved to the Client hub with the column
	// (#2022, `client-detail.svelte.spec.ts`).

	it('shows the empty message when there are no Clients', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: false });

		await expect.element(testPage.getByText('No Clients yet.')).toBeVisible();
	});

	it('shows an error notice when the Clients list fails to load', async () => {
		await setup(textResponse('Server rejected the Clients list request'));

		await expect.element(testPage.getByText('Server rejected the Clients list request')).toBeVisible();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
	});
});

// #539 (ADR-0017): a contractor Doula originates nothing at a Practice she
// contracts for -- neither errand behind "Find or add a Client" is hers --
// and her narrowed list is already her route to a Client she is attached
// to, so the control is gone rather than merely relabeled for her.
describe('a contractor Doula without the owner or admin role', () => {
	it('does not see "Find or add a Client" at all', async () => {
		await setup(undefined, { isContractor: true });

		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.not.toBeInTheDocument();
	});

	it('names why her list is empty rather than reusing "No Clients yet."', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { isContractor: true });

		await expect
			.element(
				testPage.getByText('Work reaches you as an Offer, so there are no Clients here yet.')
			)
			.toBeVisible();
	});

	it('links her empty list to #501\'s explain-only door', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { isContractor: true });

		await expect
			.element(testPage.getByRole('link', { name: 'How to add Clients of your own' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/search`);
	});

	it('does not show the explainer link once she has attached Clients', async () => {
		await setup(undefined, { isContractor: true });

		await expect
			.element(testPage.getByRole('link', { name: 'How to add Clients of your own' }))
			.not.toBeInTheDocument();
	});
});

describe('a non-contractor with an empty list', () => {
	it('shows the plain empty message with no explainer link', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: false });

		await expect.element(testPage.getByText('No Clients yet.')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'How to add Clients of your own' }))
			.not.toBeInTheDocument();
	});

	// #1706: why, beside `emptyMessage` in +page.svelte.
	it('says no Client has work yet, and how to see everyone, at a Practice whose Clients have no work', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: true });

		await expect
			.element(testPage.getByText('No Client has work yet. Turn on See everyone to list every Client.'))
			.toBeVisible();
		await expect.element(testPage.getByText('No Clients yet.')).not.toBeInTheDocument();
	});

	// With "See everyone" on there is no filter left to point at, so an
	// empty list there reads the plain message whatever the count said.
	it('keeps the plain empty message while "See everyone" is on', async () => {
		pageState.url.searchParams.set('all', 'true');

		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: true });

		await expect.element(testPage.getByText('No Clients yet.')).toBeVisible();
	});
});

// #499: the default view is "Clients with work" -- ListHandler's own
// default -- and a visible toggle switches to everyone, including a
// Client whose only Request was refused.
describe('the "see everyone" toggle', () => {
	it('defaults to the "Clients with work" filter, unchecked and no `all` param on the wire', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('switch', { name: 'See everyone' }))
			.not.toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('reflects `?all=true` from the URL: checked, and threaded through to the fetch', async () => {
		pageState.url.searchParams.set('all', 'true');

		await setup();

		await expect.element(testPage.getByRole('switch', { name: 'See everyone' })).toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?all=true`);
	});

	it('navigates to `?all=true` when switched on from the default view', async () => {
		await setup();

		await testPage.getByRole('switch', { name: 'See everyone' }).click();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients?all=true`);
	});

	// #2068: the toggle is the list's filter bar, with no Apply (a switch
	// acts at once), and the bar says what the list now holds.
	it('sits in the list’s filter bar, which says what the list holds', async () => {
		await setup();

		const bar = testPage.getByRole('group', { name: 'Choose who to show' });
		await expect.element(bar.getByRole('switch', { name: 'See everyone' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing \d+ Clients? with work/);
	});

	it('navigates back to the default view when switched off', async () => {
		pageState.url.searchParams.set('all', 'true');
		await setup();

		await testPage.getByRole('switch', { name: 'See everyone' }).click();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients`);
	});
});

// #264 (RA-G6), #2022: the Status column -- each open Engagement as its
// kind's link and a badge per fact, then a pending Request. Scoped to the
// table: the record view draws the same cell, hidden at this width.
function table() {
	return testPage.getByRole('table');
}

describe('the Status column (#264, #2022)', () => {
	it('lists Name and Status, and no longer Email, Portal invite or Pending request', async () => {
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Status' })).toBeVisible();
		for (const gone of ['Email', 'Engagements', 'Portal invite', 'Pending request']) {
			await expect.element(testPage.getByRole('columnheader', { name: gone })).not.toBeInTheDocument();
		}
	});

	it('shows each open Engagement as its own line of badges, dropping none', async () => {
		// The fixture's first Client carries three open Engagements (#596:
		// one fully populated, one fully absent; #741 a settled Contract).
		await setup();

		await expect.element(table().getByText('Contract sent')).toBeVisible();
		await expect.element(table().getByText('Overdue $4,500.00')).toBeVisible();
		await expect.element(table().getByText('Intake', { exact: true })).toBeVisible();
		await expect.element(table().getByText('No contract yet').first()).toBeVisible();
	});

	// #2130: the BFF's one count (#2122) decides it, so a late Invoice
	// reads Overdue here as on its Engagement and on Invoices.
	it('reads "Overdue" once the BFF counts the open Invoice late', async () => {
		await setup();

		await expect.element(table().getByText('Overdue $4,500.00')).toBeVisible();
		await expect.element(table().getByText('Owes $4,500.00')).not.toBeInTheDocument();
	});

	it('still reads "Owes" for an open Invoice that is not late', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[0], openEngagements: [{ ...line, daysOverdue: 0 }] }],
				hasMore: false
			})
		);

		await expect.element(table().getByText('Owes $4,500.00')).toBeVisible();
		await expect.element(table().getByText(/^Overdue/)).not.toBeInTheDocument();
	});

	// #741: `open` (billed, unpaid) is what "is anything outstanding"
	// means; a settled Contract's `paid` reads as paid, with nothing to do.
	it('reads "Paid" rather than "Owes" once nothing is outstanding', async () => {
		await setup();

		await expect.element(table().getByText('Paid $1,000.00')).toBeVisible();
	});

	it('names any other Invoice status in its own word', async () => {
		await setup(
			jsonResponse({
				items: [
					{
						...clients[1],
						openEngagements: [
							{
								engagementId: 'engagement-void',
								kind: 'birth',
								engagementStatus: 'active',
								contractStatus: 'voided',
								invoiceStatus: 'void'
							}
						]
					}
				],
				hasMore: false
			})
		);

		await expect.element(table().getByText('Contract voided')).toBeVisible();
		await expect.element(table().getByText('Invoice void')).toBeVisible();
	});

	// #1511: a new Engagement has no Visit, Contract or Invoice, so its
	// line here is one of the two ways to its page.
	it("links each line's kind to its Engagement, naming the Client", async () => {
		await setup();

		const link = testPage.getByRole('link', { name: 'Postpartum', exact: true });
		await expect.element(link).toBeVisible();
		await expect.element(link).toHaveAttribute('href', '/practices/practice-1/engagements/engagement-2');
		await expect.element(link).toHaveAccessibleDescription(clients[0]!.name);
	});

	it('draws nothing for a Client with no open Engagement and no pending Request', async () => {
		// The fixture's second Client carries neither.
		await setup(jsonResponse({ items: [clients[1]], hasMore: false }));

		await expect.element(testPage.getByRole('columnheader', { name: 'Status' })).toBeVisible();
		await expect.element(table().getByRole('listitem')).not.toBeInTheDocument();
	});

	it('shows her own fee on a line that carries one, never Invoice money', async () => {
		await setup(
			jsonResponse({
				items: [
					{
						...clients[1],
						openEngagements: [
							{ engagementId: 'engagement-fee', kind: 'birth', engagementStatus: 'active', feeCents: 120_000 }
						]
					}
				],
				hasMore: false
			})
		);

		await expect.element(table().getByText('Your fee $1,200.00')).toBeVisible();
		await expect.element(table().getByText(/Owes|Paid/)).not.toBeInTheDocument();
	});

	// #499: a pending Engagement Request shows on its Client's row, folded
	// into Status (#2022).
	it('shows a pending Request, joining both kinds when she holds one of each', async () => {
		// The fixture's third Client holds a Request of each kind.
		await setup();

		await expect.element(table().getByText('Birth request pending')).toBeVisible();
		await expect.element(table().getByText('Birth & Postpartum request pending')).toBeVisible();
	});
});

// #458: "See everyone" mixes Clients who have work with Clients who
// don't, and nothing on the row said which was which. The default view
// never mixes the two (ADR-0017: every row has work already), so the
// column would add nothing there -- it only earns its place once
// "See everyone" is on.
describe('the Work column (#458)', () => {
	it('is absent from the default "Clients with work" view', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('columnheader', { name: 'Work' }))
			.not.toBeInTheDocument();
	});

	it('shows which Clients have work and which do not once "See everyone" is on', async () => {
		pageState.url.searchParams.set('all', 'true');

		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Work' })).toBeVisible();
		// Two of the fixture's three Clients have work, so `first()`.
		await expect.element(testPage.getByRole('cell', { name: 'Has work' }).first()).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'No work yet' })).toBeVisible();
	});
});

// Bare `render(Page, {})`, not the module's own `setup()`: these two tests
// drive the exact sequence of `apiFetchWithSession` responses across two
// fetches (initial load, then "Load more"), which `setup()`'s single
// `response` parameter answers every call with alike -- there is nothing
// this block could hand it that would still let the second call differ
// from the first.
async function setupLoadMoreRace() {
	await render(Page, {});
}

describe('a "Load more" already in flight when the filter changes', () => {
	/*
	 * "Load more" is the only fetch that merges into what is on screen, so
	 * it is the only one a stale response can corrupt -- the old filter's
	 * Clients appended onto the new filter's list, with nothing afterwards
	 * to take them back out. Flipping the toggle has to abandon it.
	 */
	it('drops its response rather than appending the old filter\'s Clients', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: [clients[0]], hasMore: true, nextCursor: 'cursor-1' })
		);
		await setupLoadMoreRace();
		await expect.element(testPage.getByRole('cell', { name: clients[0].name })).toBeVisible();

		// Page two never settles until the test says so.
		const pageTwo = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValueOnce(pageTwo.promise);
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await testPage.getByRole('switch', { name: 'See everyone' }).click();
		pageTwo.resolve(
			jsonResponse({
				items: [
					{
						clientId: 'client-9',
						name: 'Stale Filter Client',
						email: 'stale@example.com',
						hasWork: true
					}
				],
				hasMore: false
			})
		);

		await expect
			.element(testPage.getByRole('cell', { name: 'Stale Filter Client' }))
			.not.toBeInTheDocument();
	});

	it('surfaces a "Load more" failure only while the filter has not moved', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: [clients[0]], hasMore: true, nextCursor: 'cursor-1' })
		);
		await setupLoadMoreRace();
		await expect.element(testPage.getByRole('cell', { name: clients[0].name })).toBeVisible();

		apiFetchWithSession.mockResolvedValueOnce(textResponse('the practice is gone'));
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByText('the practice is gone')).toBeVisible();
		// #506: a failed "Load more" used to route through the same `error`
		// state as the initial load, replacing the whole table with a bare
		// notice -- it must leave the rows already on screen in place.
		await expect.element(testPage.getByRole('cell', { name: clients[0].name })).toBeVisible();
	});

	it('swallows a "Load more" failure whose filter has since moved', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: [clients[0]], hasMore: true, nextCursor: 'cursor-1' })
		);
		await setupLoadMoreRace();
		await expect.element(testPage.getByRole('cell', { name: clients[0].name })).toBeVisible();

		const pageTwo = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValueOnce(pageTwo.promise);
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await testPage.getByRole('switch', { name: 'See everyone' }).click();
		pageTwo.resolve(textResponse('the practice is gone'));

		await expect.element(testPage.getByText('the practice is gone')).not.toBeInTheDocument();
	});
});
