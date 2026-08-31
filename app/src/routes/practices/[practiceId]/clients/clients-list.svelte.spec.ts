import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts.
import '#lib/styles/app.css';
import Page from './+page.svelte';

// Mirrors new-client.svelte.spec.ts's pattern: a hoisted, mutable
// URLSearchParams stands in for `page.url.searchParams`, set per test
// before render() rather than mutated mid-test -- the mock is a plain
// object, not Svelte-reactive, so a change after mount would never
// re-trigger the component's own `$derived`/`$effect`.
const searchParameters = vi.hoisted(() => new URLSearchParams());
vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1' }, url: { searchParams: searchParameters } }
}));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

function textResponse(body: string): Response {
	return jsonResponse(body, 403);
}

function requestUrl(callIndex = 0): string {
	return apiFetchWithSession.mock.calls[callIndex][0] as string;
}

const clients = [
	{
		clientId: 'client-1',
		name: 'Ada Lovelace',
		email: 'ada@example.com',
		hasWork: true,
		portalInviteStatus: 'sent'
	},
	{
		clientId: 'client-2',
		name: 'Grace Hopper',
		email: 'grace@example.com',
		hasWork: false
	}
];

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	searchParameters.delete('all');
});

async function setup(
	response: Response = jsonResponse({ items: clients, hasMore: false }),
	isContractor = false
) {
	// DataTable's own content floor (#508) stacks it into a <dl> below
	// 44rem, and this file's assertions are about the <table> specifically.
	await testPage.viewport(1440, 900);
	apiFetchWithSession.mockResolvedValue(response);
	await render(Page, { data: { isContractor } });
}

describe('clients list screen', () => {
	it('sends "Find or add a Client" to the search that fronts intake, not straight to intake (#498, #539)', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.toHaveAttribute('href', '/practices/practice-1/clients/search');
	});

	it('renders a Portal invite column showing the label for each status', async () => {
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
		await setup(
			jsonResponse({
				items: [
					{
						clientId: 'client-1',
						name: 'Ada Lovelace',
						email: 'ada@example.com',
						hasWork: true,
						portalInviteStatus
					}
				],
				hasMore: false
			})
		);

		await expect.element(testPage.getByRole('cell', { name: label })).toBeVisible();
	});

	it('distinguishes complained from bounced and dead-lettered as non-actionable', async () => {
		await setup(
			jsonResponse({
				items: [
					{
						clientId: 'client-1',
						name: 'Complained Client',
						email: 'complained@example.com',
						hasWork: true,
						portalInviteStatus: 'complained'
					},
					{
						clientId: 'client-2',
						name: 'Bounced Client',
						email: 'bounced@example.com',
						hasWork: true,
						portalInviteStatus: 'bounced'
					}
				],
				hasMore: false
			})
		);

		await expect
			.element(testPage.getByRole('cell', { name: 'Marked as spam (no action needed)' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Bounced — needs re-invite' })).toBeVisible();
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
		await setup(jsonResponse({ items: clients, hasMore: false }), true);

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
			.toHaveAttribute('href', '/practices/practice-1/clients/search');
	});

	it('does not show the explainer link once she has attached Clients', async () => {
		await setup(jsonResponse({ items: clients, hasMore: false }), true);

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
		expect(requestUrl()).toBe('/api/practices/practice-1/clients');
	});

	it('reflects `?all=true` from the URL: checked, and threaded through to the fetch', async () => {
		searchParameters.set('all', 'true');

		await setup();

		await expect.element(testPage.getByRole('switch', { name: 'See everyone' })).toBeChecked();
		expect(requestUrl()).toBe('/api/practices/practice-1/clients?all=true');
	});

	it('navigates to `?all=true` when switched on from the default view', async () => {
		await setup();

		await testPage.getByRole('switch', { name: 'See everyone' }).click();

		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients?all=true');
	});

	it('navigates back to the default view when switched off', async () => {
		searchParameters.set('all', 'true');
		await setup();

		await testPage.getByRole('switch', { name: 'See everyone' }).click();

		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients');
	});
});

// #499: a pending Engagement Request shows on its Client's row.
describe('a pending Request on the row', () => {
	it('shows the kind and "request pending" for a Client with one pending Request', async () => {
		await setup(
			jsonResponse({
				items: [
					{
						clientId: 'client-1',
						name: 'Pending Client',
						email: 'pending@example.com',
						hasWork: true,
						pendingRequestKinds: ['birth']
					}
				],
				hasMore: false
			})
		);

		await expect
			.element(testPage.getByRole('cell', { name: 'Birth request pending' }))
			.toBeVisible();
	});

	it('joins both kinds when a Client holds a pending Request of each', async () => {
		await setup(
			jsonResponse({
				items: [
					{
						clientId: 'client-1',
						name: 'Both Kinds Client',
						email: 'both@example.com',
						hasWork: true,
						pendingRequestKinds: ['birth', 'postpartum']
					}
				],
				hasMore: false
			})
		);

		await expect
			.element(testPage.getByRole('cell', { name: 'Birth & Postpartum request pending' }))
			.toBeVisible();
	});

	it('leaves the cell blank for a Client with no pending Request', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('columnheader', { name: 'Pending request' }))
			.toBeVisible();
		await expect.element(testPage.getByText('request pending')).not.toBeInTheDocument();
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
			jsonResponse({ items: clients, hasMore: true, nextCursor: 'cursor-1' })
		);
		await render(Page, {});
		await expect.element(testPage.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();

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
			jsonResponse({ items: clients, hasMore: true, nextCursor: 'cursor-1' })
		);
		await render(Page, {});
		await expect.element(testPage.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();

		apiFetchWithSession.mockResolvedValueOnce(textResponse('the practice is gone'));
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByText('the practice is gone')).toBeVisible();
	});

	it('swallows a "Load more" failure whose filter has since moved', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: clients, hasMore: true, nextCursor: 'cursor-1' })
		);
		await render(Page, {});
		await expect.element(testPage.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();

		const pageTwo = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValueOnce(pageTwo.promise);
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await testPage.getByRole('switch', { name: 'See everyone' }).click();
		pageTwo.resolve(textResponse('the practice is gone'));

		await expect.element(testPage.getByText('the practice is gone')).not.toBeInTheDocument();
	});
});
