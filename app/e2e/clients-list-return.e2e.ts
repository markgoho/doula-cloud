import { expect, test, type Page } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, signIn } from './auth';
import { retryPastRateLimit } from './rateLimit';
import { seedFoundingOwner } from './staffSignup';

// #2840: Back from a Client returns to the Clients list as it was left. The
// list reads 30 a page, so 31 Clients make a second page.
const BOOK = 31;
const NAME = 'Pagetest';

test('Back from a Client on the second page restores the rows, the scroll and focus', async ({ page, request, context }) => {
	const { idToken, practiceId } = await seedFoundingOwner(request);
	const staffHeaders = await signIn(request, API_URL, idToken);
	for (let number = 1; number <= BOOK; number += 1) {
		const created = await retryPastRateLimit(() =>
			request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
				headers: staffHeaders,
				data: { givenName: NAME, familyName: `Number${String(number).padStart(2, '0')}` }
			})
		);
		expect(created.ok(), `create client ${number}: ${created.status()} ${await created.text()}`).toBe(true);
	}
	await enterPractice(context, page, staffHeaders, practiceId);

	const addresses: string[] = [];
	page.on('request', (sent) => {
		addresses.push(sent.url());
	});
	page.on('framenavigated', (frame) => {
		addresses.push(frame.url());
	});

	const list = `/practices/${practiceId}/clients?all=true`;
	const rows = page.locator('tbody tr');

	async function loadSecondPageAndLeave() {
		await expect(rows).toHaveCount(30);
		await page.getByRole('button', { name: 'Load more' }).click();
		await expect(rows).toHaveCount(BOOK);
		const row = rows.nth(BOOK - 2);
		await row.scrollIntoViewIfNeeded();
		await page.evaluate(() => window.scrollBy(0, 60));
		const link = row.getByRole('link');
		const label = (await link.textContent())!.trim();
		const scrollY = await page.evaluate(() => Math.round(window.scrollY));
		expect(scrollY).toBeGreaterThan(500);
		await link.click();
		await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/clients/[^/?]+$`));
		await page.goBack();
		await expect(rows).toHaveCount(BOOK);
		await expect(page.getByRole('link', { name: label })).toBeFocused();
		await expect.poll(() => page.evaluate(() => Math.round(window.scrollY))).toBeCloseTo(scrollY, -1);
	}

	await test.step('with a filter in the address', async () => {
		await page.goto(list);
		await loadSecondPageAndLeave();
		await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/clients[?]all=true$`));
	});

	await test.step('with a Name search, which is in no address', async () => {
		await page.goto(list);
		await enterName(page);
		await loadSecondPageAndLeave();
		await expect(page.getByRole('searchbox', { name: 'Name' })).toHaveValue(NAME);
		expect(addresses.length).toBeGreaterThan(0);
		for (const address of addresses) expect(decodeURIComponent(address)).not.toContain(NAME);
	});
});

async function enterName(page: Page) {
	await page.getByRole('searchbox', { name: 'Name' }).fill(NAME);
	await page.getByRole('button', { name: 'Apply' }).click();
	await expect(page.getByRole('button', { name: 'Load more' })).toBeVisible();
}
