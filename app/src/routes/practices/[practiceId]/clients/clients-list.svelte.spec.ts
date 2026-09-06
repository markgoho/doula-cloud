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
async function setup(response?: Response, isContractor = false) {
	// DataTable's own content floor (#508) stacks it into a <dl> below
	// 46rem, and this file's assertions are about the <table> specifically.
	await testPage.viewport(1440, 900);
	if (response) {
		apiFetchWithSession.mockResolvedValue(response);
	} else {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
	}
	await render(Page, { data: { isContractor, isOwner: false } });
}

describe('clients list screen', () => {
	it('sends "Find or add a Client" to the search that fronts intake, not straight to intake (#498, #539)', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/search`);
	});

	it('renders a Portal invite column showing the label for each status', async () => {
		// Both rows are the fixture's own: its second Client carries no
		// `portalInviteStatus` at all, which is what "never invited" is.
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Portal invite' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Invite sent' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Never invited' })).toBeVisible();
	});

	it.each([
		['pending', 'Invite pending'],
		['bounced', 'Bounced — needs re-invite'],
		['dead_lettered', 'Dead-lettered — needs re-invite'],
		['complained', 'Marked as spam (no action needed)'],
		['accepted', 'Accepted']
	])('shows %s as %s', async (portalInviteStatus, label) => {
		await setup(jsonResponse({ items: [{ ...clients[0], portalInviteStatus }], hasMore: false }));

		await expect.element(testPage.getByRole('cell', { name: label })).toBeVisible();
	});

	it('distinguishes complained from bounced and dead-lettered as non-actionable', async () => {
		await setup(
			jsonResponse({
				items: [
					{ ...clients[0], portalInviteStatus: 'complained' },
					{ ...clients[0], clientId: 'client-2', portalInviteStatus: 'bounced' }
				],
				hasMore: false
			})
		);

		await expect
			.element(testPage.getByRole('cell', { name: 'Marked as spam (no action needed)' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Bounced — needs re-invite' })).toBeVisible();
	});

	it.each([
		['bounced', 'Bounced — unblock the address to invite again'],
		['dead_lettered', 'Not sent — unblock the address to invite again']
	])(
		'shows %s on a suppressed address as %s, not a re-invite that cannot succeed',
		async (portalInviteStatus, label) => {
			// #785: the send-time guard (#733) refuses a suppressed address
			// before Mailgun is asked, so a re-invite dead-letters on
			// arrival. The row has to name the unblock instead.
			await setup(
				jsonResponse({
					items: [{ ...clients[0], portalInviteStatus, emailSuppressed: true }],
					hasMore: false
				})
			);

			await expect.element(testPage.getByRole('cell', { name: label })).toBeVisible();
		}
	);

	it('offers Blocked email addresses on a suppressed row, and nowhere else', async () => {
		await setup(
			jsonResponse({
				items: [
					{ ...clients[0], portalInviteStatus: 'bounced', emailSuppressed: true },
					{ ...clients[0], clientId: 'client-2', portalInviteStatus: 'bounced' }
				],
				hasMore: false
			})
		);

		const link = testPage.getByRole('link', { name: 'Blocked email addresses' });
		await expect.element(link).toBeVisible();
		await expect
			.element(link)
			.toHaveAttribute('href', '/practices/practice-1/settings/blocked-addresses');
		expect(link.elements()).toHaveLength(1);
		// The second row's suppression was lifted; its own words, and its
		// absence of a link, are what says so.
		await expect.element(testPage.getByRole('cell', { name: 'Bounced — needs re-invite' })).toBeVisible();
	});

	it('keeps a suppressed address on a state the unblock does not unstick reading as it did', async () => {
		// 'complained' is never clearable (ADR-0029) and 'sent' is not a
		// failure, so neither takes the #785 wording even though the
		// address is suppressed.
		await setup(
			jsonResponse({
				items: [
					{ ...clients[0], portalInviteStatus: 'complained', emailSuppressed: true },
					{ ...clients[0], clientId: 'client-2', portalInviteStatus: 'sent', emailSuppressed: true }
				],
				hasMore: false
			})
		);

		await expect
			.element(testPage.getByRole('cell', { name: 'Marked as spam (no action needed)' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Invite sent' })).toBeVisible();
	});

	it('shows the empty message when there are no Clients', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }));

		// getByRole, not getByText: the record view carries the same
		// message in a hidden <p>, and only a role query excludes it.
		await expect.element(testPage.getByRole('cell', { name: 'No Clients yet.' })).toBeVisible();
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
		await setup(undefined, true);

		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.not.toBeInTheDocument();
	});

	it('names why her list is empty rather than reusing "No Clients yet."', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), true);

		// getByRole, not getByText: the record view carries the same
		// message in a hidden <p>, and only a role query excludes it.
		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'Work reaches you as an Offer, so there are no Clients here yet.'
				})
			)
			.toBeVisible();
	});

	it('links her empty list to #501\'s explain-only door', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), true);

		await expect
			.element(testPage.getByRole('link', { name: "How to add Clients of your own" }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/search`);
	});

	it('does not show the explainer link once she has attached Clients', async () => {
		await setup(undefined, true);

		await expect
			.element(testPage.getByRole('link', { name: "How to add Clients of your own" }))
			.not.toBeInTheDocument();
	});
});

describe('a non-contractor with an empty list', () => {
	it('shows the plain empty message with no explainer link', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }));

		// getByRole, not getByText: the record view carries the same
		// message in a hidden <p>, and only a role query excludes it.
		await expect.element(testPage.getByRole('cell', { name: 'No Clients yet.' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "How to add Clients of your own" }))
			.not.toBeInTheDocument();
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

	it('navigates back to the default view when switched off', async () => {
		pageState.url.searchParams.set('all', 'true');
		await setup();

		await testPage.getByRole('switch', { name: 'See everyone' }).click();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients`);
	});
});

// #264 (RA-G6): the open-Engagement rollup column.
describe('the Engagements rollup column (#264)', () => {
	it('shows one line per open Engagement, dropping none', async () => {
		// The fixture's first Client already carries two openEngagements
		// entries (#596: one fully populated, one fully absent).
		await setup();

		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'Contract: Sent · Doula: Yolanda Okonkwo-Fitzgerald · Active · Invoice: Open ($4,500.00)'
				})
			)
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: 'Contract: No contract yet · Doula: No Doula assigned · Intake' }))
			.toBeVisible();
	});

	it('shows no rollup lines for a Client with zero open Engagements', async () => {
		// The fixture's second Client carries no openEngagements at all.
		await setup(jsonResponse({ items: [clients[1]], hasMore: false }));

		await expect
			.element(testPage.getByRole('columnheader', { name: 'Engagements' }))
			.toBeVisible();
		await expect.element(testPage.getByText('Contract:')).not.toBeInTheDocument();
	});

	it('shows her own fee on a line that carries one, never Invoice money', async () => {
		await setup(
			jsonResponse({
				items: [
					{
						...clients[1],
						openEngagements: [
							{ engagementId: 'engagement-fee', engagementStatus: 'active', feeCents: 120_000 }
						]
					}
				],
				hasMore: false
			})
		);

		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'Contract: No contract yet · Doula: No Doula assigned · Active · Your fee: $1,200.00'
				})
			)
			.toBeVisible();
	});
});

// #499: a pending Engagement Request shows on its Client's row.
describe('a pending Request on the row', () => {
	it('shows the kind and "request pending" for a Client with one pending Request', async () => {
		// The fixture's own Client already carries `pendingRequestKinds:
		// ['birth']` (#596), so the default fixture response is the whole
		// case here.
		await setup();

		await expect
			.element(testPage.getByRole('cell', { name: 'Birth request pending' }))
			.toBeVisible();
	});

	it('joins both kinds when a Client holds a pending Request of each', async () => {
		await setup(
			jsonResponse({ items: [{ ...clients[0], pendingRequestKinds: ['birth', 'postpartum'] }], hasMore: false })
		);

		await expect
			.element(testPage.getByRole('cell', { name: 'Birth & Postpartum request pending' }))
			.toBeVisible();
	});

	it('leaves the cell blank for a Client with no pending Request', async () => {
		// The fixture's second Client has no pending Request. Only she is
		// listed, because this asserts the absence of the text anywhere on
		// the page and her neighbour has one.
		await setup(jsonResponse({ items: [clients[1]], hasMore: false }));

		await expect
			.element(testPage.getByRole('columnheader', { name: 'Pending request' }))
			.toBeVisible();
		await expect.element(testPage.getByText('request pending')).not.toBeInTheDocument();
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
		await expect.element(testPage.getByRole('cell', { name: 'Has work' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'No work yet' })).toBeVisible();
	});
});

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
		await render(Page, {});
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
		await render(Page, {});
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
		await render(Page, {});
		await expect.element(testPage.getByRole('cell', { name: clients[0].name })).toBeVisible();

		const pageTwo = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValueOnce(pageTwo.promise);
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await testPage.getByRole('switch', { name: 'See everyone' }).click();
		pageTwo.resolve(textResponse('the practice is gone'));

		await expect.element(testPage.getByText('the practice is gone')).not.toBeInTheDocument();
	});
});
