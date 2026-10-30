import { expect, test, type Page } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, signIn } from './auth';
import { staffLandingSettled } from './mountSettled';
import { seedFoundingOwner } from './staffSignup';

/*
 * #2246: on a client-side move, SvelteKit's announcer read `document.title`
 * the instant the new page mounted. On a page titled from its own read
 * that was a placeholder or nothing, so a screen reader heard "Change
 * roles", or just "— DoulaCloud", for a page whose heading was a name.
 * `RouteAnnouncer` reads the title once the page has settled, and
 * SvelteKit's own region is hidden, so it is never read.
 *
 * Only the real router calls `afterNavigate`, so only a browser shows what
 * is announced. Every text the app's live region ever holds is recorded
 * from the first frame, so a placeholder that is replaced a frame later
 * still fails the test. SvelteKit's region still fills with the
 * placeholder; it is left out of the record because it is never rendered,
 * which the test asserts on its own.
 */

const RECORD_ANNOUNCEMENTS = `
	window.__announcements = [];
	new MutationObserver(() => {
		for (const region of document.querySelectorAll('[aria-live="assertive"]:not(#svelte-announcer)')) {
			const text = region.textContent.trim();
			const seen = window.__announcements;
			if (text && seen[seen.length - 1] !== text) seen.push(text);
		}
	}).observe(document, { subtree: true, childList: true, characterData: true });
`;

async function announcements(page: Page): Promise<string[]> {
	return page.evaluate(() => (globalThis as unknown as { __announcements: string[] }).__announcements);
}

test('a move to a page titled from its own read announces its settled title, and nothing before it', async ({
	page,
	request,
	context,
}) => {
	const { idToken, practiceId } = await seedFoundingOwner(request, {
		practiceName: 'Lakeside Doulas',
		staffName: 'Robin Owner',
	});
	const headers = await signIn(request, API_URL, idToken);
	await page.addInitScript(RECORD_ANNOUNCEMENTS);
	await Promise.all([
		staffLandingSettled(page, practiceId),
		enterPractice(context, page, headers, practiceId),
	]);

	// SvelteKit's own region is there and never rendered, so never read.
	await expect(page.locator('#svelte-announcer')).toBeAttached();
	await expect(page.locator('#svelte-announcer')).toBeHidden();

	await page
		.getByRole('navigation', { name: 'Practice' })
		.getByRole('link', { name: 'Staff' })
		.click();
	await page.getByRole('link', { name: 'Robin Owner' }).click();
	await expect(
		page.getByRole('heading', { level: 1, name: 'Robin Owner' })
	).toBeVisible();
	await expect(page).toHaveTitle('Robin Owner — DoulaCloud');
	await expect
		.poll(() => announcements(page))
		.toEqual(['Staff — DoulaCloud', 'Robin Owner — DoulaCloud']);

	// The roles page used to announce "Change roles" while it loaded.
	// The first Change is the Roles row's.
	await page.getByRole('link', { name: 'Change' }).first().click();
	await expect(page).toHaveTitle(
		'Which roles does Robin Owner have? — DoulaCloud'
	);
	await expect
		.poll(() => announcements(page))
		.toEqual([
			'Staff — DoulaCloud',
			'Robin Owner — DoulaCloud',
			'Which roles does Robin Owner have? — DoulaCloud',
		]);

	// And the save back to her page announced "— DoulaCloud".
	await page.getByRole('button', { name: 'Save' }).click();
	await expect(
		page.getByRole('heading', { level: 1, name: 'Robin Owner' })
	).toBeVisible();
	await expect
		.poll(() => announcements(page))
		.toEqual([
			'Staff — DoulaCloud',
			'Robin Owner — DoulaCloud',
			'Which roles does Robin Owner have? — DoulaCloud',
			'Robin Owner — DoulaCloud',
		]);
});

test('Save and continue announces the Start work form by the Client it is for, not "Start new work"', async ({
	page,
	request,
	context,
}) => {
	const { idToken, practiceId } = await seedFoundingOwner(request);
	const headers = await signIn(request, API_URL, idToken);
	await page.addInitScript(RECORD_ANNOUNCEMENTS);
	await Promise.all([
		staffLandingSettled(page, practiceId),
		enterPractice(context, page, headers, practiceId),
	]);

	await page.getByRole('link', { name: 'Add your first Client' }).click();
	await page.getByLabel('Given name').fill('Ines');
	await page.getByLabel('Family name (optional)').fill('Calloway');
	await page.getByRole('button', { name: 'Save and continue' }).click();

	await expect(
		page.getByRole('button', { name: 'Start work with Ines' })
	).toBeVisible();
	const title = await page.title();
	expect(title).toContain('Ines');
	await expect.poll(() => announcements(page)).toContain(title);
	const heard = await announcements(page);
	expect(heard.at(-1)).toBe(title);
	expect(heard).not.toContain('Start new work — DoulaCloud');
});
