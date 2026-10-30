import { expect, test, type Page } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, signIn } from './auth';
import { staffLandingSettled } from './mountSettled';
import { seedFoundingOwner } from './staffSignup';

/*
 * #2235: following a section in the narrow Staff bar's sheet left the
 * sheet open over the page it chose. StaffTopBar's own spec shows a link
 * press shuts it; what only the real router can show is where focus goes
 * after the move. The sheet shuts before the router runs, so the router's
 * reset puts focus where a press on the wide bar's nav puts it, and not
 * back on the Menu button that opened the sheet.
 */

async function focusedElement(page: Page): Promise<string> {
	return page.evaluate(() => {
		const element = document.activeElement;
		if (!element || element === document.body) {
			return 'body';
		}
		return `<${element.tagName.toLowerCase()}> ${element.getAttribute('aria-label') ?? element.textContent?.trim() ?? ''}`;
	});
}

test('a section followed from the narrow sheet shuts it, and focus lands where the wide nav puts it', async ({
	page,
	request,
	context
}) => {
	const { idToken, practiceId } = await seedFoundingOwner(request, {
		practiceName: 'Lakeside Doulas',
		staffName: 'Robin Owner'
	});
	const headers = await signIn(request, API_URL, idToken);

	await page.setViewportSize({ width: 1440, height: 900 });
	await Promise.all([
		staffLandingSettled(page, practiceId),
		enterPractice(context, page, headers, practiceId)
	]);
	await page
		.getByRole('navigation', { name: 'Practice' })
		.getByRole('link', { name: 'Staff' })
		.press('Enter');
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/staff$`));
	// The router's reset: the body, the sequential focus starting point
	// for the new page, so the next Tab enters it from the top.
	await expect.poll(() => focusedElement(page)).toBe('body');

	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto(`/practices/${practiceId}`);
	await page.getByRole('button', { name: 'Menu' }).click();
	const sheet = page.getByRole('dialog', { name: 'Practice menu' });
	await sheet.getByRole('link', { name: 'Staff' }).press('Enter');

	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/staff$`));
	await expect(sheet).toBeHidden();
	await expect(page.getByRole('button', { name: 'Close menu' })).toBeHidden();
	await expect.poll(() => focusedElement(page)).toBe('body');
});
