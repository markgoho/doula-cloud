import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { AwaitingContract } from '#lib/contract.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { data, fixture } from './page.fixture.js';

/*
 * The screen's content and the `page` it reads both come from the route's
 * own fixture (#596), the same installation invoices.svelte.spec.ts uses.
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

const [draftContract, sentContract] = data.items;
const { practiceId } = fixture.params;

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

// `session` merges in from practices/[practiceId]/+layout.ts (#835) --
// this route never reads it, but the generated `data` prop type requires
// it, since SvelteKit really does merge ancestor layout data into it at
// runtime.
const sessionStub = {
	practiceId,
	staffId: 'staff-1',
	practiceName: 'Riverside Doula Collective',
	roles: ['owner'],
	isContractor: false
};

async function setup(page: CursorPage<AwaitingContract> = data) {
	// Wide enough for DataTable's <table> rather than the <dl> record view
	// its content floor stacks into below 46rem (#508) -- the same call the
	// Invoice list's own spec makes for the same reason.
	await testPage.viewport(1440, 900);
	await render(Page, { params: fixture.params, data: { ...page, session: sessionStub } });
}

describe('the Practice-wide Contracts awaiting signature list (#273)', () => {
	it('lists every outstanding Contract without opening a single Engagement', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: fixture.readyText }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: draftContract.clientName }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: sentContract.clientName }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Draft', exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Sent', exact: true })).toBeVisible();
	});

	it('names each Client as the way in to her Engagement', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: draftContract.clientName }))
			.toHaveAttribute('href', `/practices/${practiceId}/engagements/${draftContract.engagementId}`);
	});

	it('says so plainly when nothing is waiting', async () => {
		// Not the happy path, so it is this spec's own to declare -- but it
		// is declared as a departure from the fixture rather than as a
		// second description of the same screen.
		await setup({ items: [], hasMore: false });

		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'Nothing is waiting. Every contract has been signed or voided.'
				})
			)
			.toBeVisible();
	});

	it('appends the next page rather than replacing the one already read', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				items: [
					{
						engagementId: 'eng-3',
						contractId: 'contract-3',
						clientId: 'client-3',
						clientName: 'Cleo',
						status: 'draft',
						createdAt: '2026-06-01T00:00:00Z'
					}
				],
				hasMore: false
			})
		);

		await setup({ ...data, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Cleo' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: draftContract.clientName }))
			.toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/contracts/awaiting-signature?cursor=cursor-1`
		);
	});

	it('reports a failed next page in place rather than losing the list', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('invalid cursor', 400));

		await setup({ ...data, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('invalid cursor');
		await expect
			.element(testPage.getByRole('link', { name: draftContract.clientName }))
			.toBeVisible();
	});
});
