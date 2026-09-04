import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { Balance } from '#lib/billing.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts. This
// route's ListPage (#491) also needs the primitives registered, not just
// their CSS: <center-l max="none"> only lifts the default var(--measure) cap
// via the custom element's own attribute handling, and an unregistered
// center-l never runs it, leaving every DataTable narrower than its floor.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { data, fixture } from './page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The `page` this route reads comes from its own fixture (#596), so what
 * this spec renders and what the continuum sweep measures are one
 * description. `vi.mock` is hoisted above every import, so `pageState` is
 * declared empty and filled from the fixture once the imports have run --
 * the route reads `page.url` inside a `$derived`, not at module scope, so
 * the later write is seen. Same installation, through the same
 * `toPageState`, as `route-continuum.svelte.spec.ts`.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
// apiErrorMessage is the real one's behavior for a plain-text body, which
// is what this screen's refusals are: the ledger loader reads a failure
// through it rather than calling response.text() itself.
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

// None of this file's tests are about what an owner alone sees, so the
// session read stays this spec's own -- the fixture's own `respond`
// answers `/session` with `roles: ['owner']`, a variant these tests don't
// need and must not invent by widening the fixture (#596).
beforeEach(() => {
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockResolvedValue(jsonResponse({ roles: [] }));
	sessionStorage.clear();
});

const { practiceId } = fixture.params;
// request-1 names no Request this fixture describes -- the approval
// screen's own return path is a plain string this route only echoes back.
const approvalReturnPath = `/practices/${practiceId}/engagement-requests/request-1`;

describe('the way back to an approval an empty balance interrupted (#502)', () => {
	it('offers the remembered approval screen', async () => {
		sessionStorage.setItem('engagement-request-approval-return', approvalReturnPath);

		await render(Page, { params: fixture.params, data });

		await expect
			.element(testPage.getByRole('link', { name: 'Back to the engagement request you were deciding' }))
			.toHaveAttribute('href', approvalReturnPath);
	});

	it('offers nothing when storage itself is unreachable, rather than failing the page', async () => {
		vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
			throw new Error('site data blocked');
		});

		await render(Page, { params: fixture.params, data });

		await expect
			.element(testPage.getByRole('link', { name: 'Back to the engagement request you were deciding' }))
			.not.toBeInTheDocument();
		vi.restoreAllMocks();
	});

	it('offers nothing to somebody who came here on her own', async () => {
		await render(Page, { params: fixture.params, data });

		await expect
			.element(testPage.getByRole('link', { name: 'Back to the engagement request you were deciding' }))
			.not.toBeInTheDocument();
	});
});

describe('billing ledger', () => {
	it('right-aligns the Quantity column, header and body cell alike (#509)', async () => {
		// DataTable's own content floor (#508) stacks it into a <dl> below
		// 46rem, and this checks the <table> cells specifically.
		await testPage.viewport(1440, 900);
		await render(Page, { params: fixture.params, data });

		const header = testPage.getByRole('columnheader', { name: 'Quantity' });
		const cell = testPage.getByRole('cell', { name: '+20' });
		await expect.element(cell).toBeVisible();
		expect(getComputedStyle(header.element()).textAlign).toBe('end');
		expect(getComputedStyle(cell.element()).textAlign).toBe('end');
	});

	// #506: handleLoadMoreLedger used to return silently on a failed
	// response, leaving "Load more" clickable again with no feedback --
	// it must surface the failure instead, next to the existing rows.
	it('surfaces a "Load more" failure instead of swallowing it', async () => {
		const pagedData: Balance = {
			...data,
			ledger: { ...data.ledger, hasMore: true, nextCursor: 'cursor-1' }
		};
		await render(Page, { params: fixture.params, data: pagedData });
		await expect.element(testPage.getByRole('cell', { name: '+20' })).toBeVisible();

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('the practice is gone', 403));
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByText('the practice is gone')).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: '+20' })).toBeVisible();
	});
});
