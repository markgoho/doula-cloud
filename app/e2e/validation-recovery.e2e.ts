import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

/**
 * GOV.UK's Recover from validation errors pattern, proved on a real
 * refused form (#467).
 *
 * Two of the three things the pattern promises cannot be seen from
 * inside a component spec, and this is the only place they can be:
 *
 * 1. **A refused page is still accessible.** The error summary renders an
 *    `<h2>` *above* the page's `<h1>`, which is GOV.UK's own markup and
 *    looks wrong the first time anybody reads it. `accessibility.e2e.ts`
 *    only ever scans clean forms, so without this the summary is the one
 *    thing in the application axe never sees.
 * 2. **An entry moves focus to its control.** The component writes no
 *    click handler for this: an `<a href="#id">` pointing at a focusable
 *    control is focused by the browser as part of navigating to the
 *    fragment, and the Rule of Least Power says do not write JavaScript
 *    for what the platform already does. That is a claim about browsers,
 *    not about our code, so it is asserted against a real one rather than
 *    against jsdom.
 *
 * `/login` is the subject because it needs no fixture: an empty submit is
 * refused before anything reaches the network.
 */

const WCAG_TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

// #487, and the reason is on `accessibility.e2e.ts`'s own KNOWN list:
// nothing in the application sets a <title> yet.
const KNOWN_RULE_IDS = new Set(['document-title']);

test('a refused form says what is wrong, takes focus, and leads to the field', async ({ page }) => {
	await page.goto('/login');
	await expect(page.getByRole('heading', { level: 1, name: 'Log in' })).toBeVisible();

	// Nothing before the submit: no summary on a clean form, which is what
	// stops it stealing focus from somebody who has not asked for anything
	// yet.
	await expect(page.getByRole('heading', { name: 'There is a problem' })).toBeHidden();

	await page.getByRole('button', { name: 'Log in' }).click();

	// The browser's own validation is off (`novalidate`), so the page is
	// what refuses -- and it refuses with *every* reason at once rather
	// than stopping at the first empty field, which is the whole
	// difference between this and a native bubble.
	const summary = page.getByRole('heading', { name: 'There is a problem' });
	await expect(summary).toBeVisible();
	await expect(page.getByRole('link', { name: 'Enter your email address' })).toBeVisible();
	await expect(page.getByRole('link', { name: 'Enter your password' })).toBeVisible();

	// The summary itself holds focus, so a screen reader announces the
	// refusal and the first fix is one Tab away.
	await expect(page.locator('div.summary')).toBeFocused();

	const { violations } = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();
	expect(
		violations
			.filter((violation) => !KNOWN_RULE_IDS.has(violation.id))
			.map((violation) => `${violation.id} (${violation.impact}) -- ${violation.help}`),
		'a refused form must be no less accessible than a clean one'
	).toEqual([]);

	// The word-for-word rule, seen rather than asserted about one array:
	// the message beside the field is the entry above it.
	await expect(page.getByText('Enter your email address')).toHaveCount(2);

	// The claim the component leaves to the platform.
	await page.getByRole('link', { name: 'Enter your password' }).click();
	await expect(page.getByLabel('Password')).toBeFocused();

	// And recovery: fixing the fields and submitting again clears the
	// summary rather than leaving a stale list of solved problems.
	await page.getByLabel('Email').fill('nobody@example.com');
	await page.getByLabel('Password').fill('a-password');
	await page.getByRole('button', { name: 'Log in' }).click();
	await expect(page.getByRole('link', { name: 'Enter your email address' })).toBeHidden();
});
