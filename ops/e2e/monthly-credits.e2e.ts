import { expect, test } from '../../app/e2e/playwrightTest';
import { seedMonthlyCreditsPractice } from '../../app/e2e/stack';
import { signedInOperator } from './operator';

/*
 * The monthly Credits read against the local stack: it opens on the last
 * closed month with a Practice's purchase in it, offers no month still in
 * progress, and reads an earlier month when one is chosen.
 */

function monthName(month: string): string {
	const [year, number] = month.split('-').map(Number);
	return new Date(Date.UTC(year, number - 1, 1)).toLocaleDateString('en-US', { timeZone: 'UTC', month: 'long', year: 'numeric' });
}

function monthBefore(month: string): string {
	const [year, number] = month.split('-').map(Number);
	const before = new Date(Date.UTC(year, number - 2, 1));
	return `${before.getUTCFullYear()}-${String(before.getUTCMonth() + 1).padStart(2, '0')}`;
}

function monthAfter(month: string): string {
	const [year, number] = month.split('-').map(Number);
	const after = new Date(Date.UTC(year, number, 1));
	return `${after.getUTCFullYear()}-${String(after.getUTCMonth() + 1).padStart(2, '0')}`;
}

test('An operator reads the last closed month and then an earlier one', async ({ page, request }) => {
	const name = `Harbor Doulas ${crypto.randomUUID().slice(0, 8)}`;
	const lastClosed = seedMonthlyCreditsPractice(name);
	const earlier = monthBefore(lastClosed);
	await signedInOperator(page, request, 'operator-monthly-password', 'Monthly Operator');

	await page.getByRole('link', { name: 'Monthly Credits' }).click();

	await expect(page.getByRole('heading', { name: 'Monthly Credits', level: 1 })).toBeVisible();
	const table = page.getByRole('table', { name: `Credits in ${monthName(lastClosed)}` });
	await expect(table.getByRole('row', { name: `${name} 4 0` })).toBeVisible();
	const picker = page.getByRole('combobox', { name: 'Month' });
	await expect(picker).toHaveValue(lastClosed);
	const inProgress = monthName(monthAfter(lastClosed));
	await expect(picker.getByRole('option', { name: inProgress })).toHaveCount(0);

	await picker.selectOption(earlier);
	await page.getByRole('button', { name: 'Show month' }).click();

	await expect(page).toHaveURL((url) => url.pathname === '/credits' && url.searchParams.get('month') === earlier);
	const earlierTable = page.getByRole('table', { name: `Credits in ${monthName(earlier)}` });
	await expect(earlierTable.getByRole('row', { name: `${name} 0 0` })).toBeVisible();
});
