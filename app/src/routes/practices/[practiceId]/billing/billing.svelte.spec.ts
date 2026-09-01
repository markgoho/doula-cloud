import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { Balance } from '#lib/billing.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts.
import '#lib/styles/app.css';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1' }, url: new URL('https://example.test/billing') }
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const data: Balance = {
	balance: 19,
	ledger: {
		items: [{ origin: 'purchase', quantity: 20, createdAt: '2026-08-01T00:00:00Z' }],
		hasMore: false
	}
};

beforeEach(() => {
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockResolvedValue(jsonResponse({ roles: [] }));
	sessionStorage.clear();
});

describe('the way back to an approval an empty balance interrupted (#502)', () => {
	it('offers the remembered approval screen', async () => {
		sessionStorage.setItem('engagement-request-approval-return', '/practices/practice-1/engagement-requests/request-1');

		await render(Page, { params: { practiceId: 'practice-1' }, data });

		await expect
			.element(testPage.getByRole('link', { name: 'Back to the engagement request you were deciding' }))
			.toHaveAttribute('href', '/practices/practice-1/engagement-requests/request-1');
	});

	it('offers nothing when storage itself is unreachable, rather than failing the page', async () => {
		vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
			throw new Error('site data blocked');
		});

		await render(Page, { params: { practiceId: 'practice-1' }, data });

		await expect
			.element(testPage.getByRole('link', { name: 'Back to the engagement request you were deciding' }))
			.not.toBeInTheDocument();
		vi.restoreAllMocks();
	});

	it('offers nothing to somebody who came here on her own', async () => {
		await render(Page, { params: { practiceId: 'practice-1' }, data });

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
		await render(Page, { params: { practiceId: 'practice-1' }, data });

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
		await render(Page, { params: { practiceId: 'practice-1' }, data: pagedData });
		await expect.element(testPage.getByRole('cell', { name: '+20' })).toBeVisible();

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('the practice is gone', 403));
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByText('the practice is gone')).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: '+20' })).toBeVisible();
	});
});
