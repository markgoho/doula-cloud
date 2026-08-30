import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { Balance } from '#lib/billing.js';
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
});

describe('billing ledger', () => {
	it('right-aligns the Quantity column, header and body cell alike (#509)', async () => {
		await render(Page, { params: { practiceId: 'practice-1' }, data });

		const header = testPage.getByRole('columnheader', { name: 'Quantity' });
		const cell = testPage.getByRole('cell', { name: '+20' });
		await expect.element(cell).toBeVisible();
		expect(getComputedStyle(header.element()).textAlign).toBe('end');
		expect(getComputedStyle(cell.element()).textAlign).toBe('end');
	});
});
