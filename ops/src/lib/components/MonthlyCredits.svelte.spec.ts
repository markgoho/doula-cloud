import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import MonthlyCredits from './MonthlyCredits.svelte';

function answering(status: number, body: unknown) {
	return vi.fn<typeof fetch>(async () => Response.json(body, { status }));
}

const september = {
	month: '2026-09',
	months: ['2026-09', '2026-08', '2026-07'],
	practices: [
		{ id: 'p-2', name: 'Atlas Doulas', creditsBought: 3, creditsSpent: 0, boughtBefore: false },
		{ id: 'p-1', name: 'Rooted Birth Collective', creditsBought: 10, creditsSpent: 2, boughtBefore: true }
	],
	totals: { creditsBought: 13, creditsSpent: 2, practicesBoughtAgain: 1 }
};

describe('MonthlyCredits', () => {
	it('shows each Practice’s Credits bought and spent in the month, with totals', async () => {
		const fetcher = answering(200, september);

		await render(MonthlyCredits, { month: undefined, fetcher, onsignedout: vi.fn() });

		const table = page.getByRole('table', { name: 'Credits in September 2026' });
		await expect.element(table).toBeVisible();
		await expect
			.element(table.getByRole('row', { name: 'Rooted Birth Collective 10 2' }))
			.toBeVisible();
		await expect.element(table.getByRole('row', { name: 'Atlas Doulas 3 0' })).toBeVisible();
		await expect.element(table.getByRole('row', { name: 'Total 13 2' })).toBeVisible();
		await expect.element(page.getByText('Practices who bought again: 1')).toBeVisible();
		expect(fetcher).toHaveBeenCalledWith('/api/credits/monthly', { credentials: 'include' });
	});

	it('offers only the closed months, the one shown chosen', async () => {
		await render(MonthlyCredits, { month: '2026-09', fetcher: answering(200, september), onsignedout: vi.fn() });

		const picker = page.getByRole('combobox', { name: 'Month' });
		await expect.element(picker).toHaveValue('2026-09');
		await expect.element(picker.getByRole('option')).toHaveLength(3);
		await expect.element(picker.getByRole('option', { name: 'July 2026' })).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Show month' })).toBeVisible();
	});

	it('says when no Practice existed in the month', async () => {
		const empty = { ...september, practices: [], totals: { creditsBought: 0, creditsSpent: 0, practicesBoughtAgain: 0 } };

		await render(MonthlyCredits, { month: undefined, fetcher: answering(200, empty), onsignedout: vi.fn() });

		await expect.element(page.getByText('No Practice had signed up by the end of September 2026.')).toBeVisible();
		await expect.element(page.getByRole('table')).not.toBeInTheDocument();
	});

	it('shows a refused month and the way back to the last closed one', async () => {
		const message = 'Choose a month that has ended, written as YYYY-MM.';
		const fetcher = answering(400, { code: 'INVALID_ARGUMENT', message, details: { month: message } });

		await render(MonthlyCredits, { month: '2026-11', fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByRole('alert')).toHaveTextContent(message);
		await expect
			.element(page.getByRole('link', { name: 'Show the last month that has ended' }))
			.toHaveAttribute('href', '/credits');
		await expect.element(page.getByRole('table')).not.toBeInTheDocument();
	});

	it('hands back to sign-in when the session has ended', async () => {
		const onsignedout = vi.fn();

		await render(MonthlyCredits, { month: undefined, fetcher: answering(401, {}), onsignedout });

		await expect.poll(() => onsignedout).toHaveBeenCalled();
	});
});
