import { expect, test } from '@playwright/test';
import { enterPracticeAsEnrolled, signInEnrolled } from './mfa';
import { portalEngagementSettled } from './mountSettled';
import { seedPortalClient, signInPortalClient } from './portalClient';
import { seedFoundingOwner } from './staffSignup';

/**
 * A piece of Feedback sent end to end from a Staff screen (#1527): the
 * Pilot banner opens the drawer, a send reaches the real BFF
 * (`POST /api/staff/feedback`, #1523), and the drawer's own AC --
 * closes, shows a focused status Notice -- follows. The GitHub-issue
 * outbox (#1524) is not merged yet, so this stops at the 201 and the
 * database row; no GitHub issue is expected to appear.
 */
test('sends a piece of Feedback from a Staff screen', async ({ page, request, context }) => {
	const owner = await seedFoundingOwner(request, { practiceName: 'Riverside Doulas' });
	const staffHeaders = await signInEnrolled(request, owner.idToken, owner.localId);

	await enterPracticeAsEnrolled(context, page, staffHeaders, owner.practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${owner.practiceId}$`));

	await page.getByRole('button', { name: 'Tell us what is not working or what you need.' }).click();
	await expect(page.getByRole('dialog', { name: 'Send feedback to DoulaCloud' })).toBeVisible();

	await page.getByLabel('An idea or a request').check();
	await page.getByLabel('Tell us more').fill('Add a way to export invoices as CSV.');

	const sent = page.waitForResponse(
		(response) => response.url().endsWith('/api/staff/feedback') && response.request().method() === 'POST'
	);
	await page.getByRole('button', { name: 'Send feedback' }).click();
	const response = await sent;
	expect(response.status()).toBe(201);

	await expect(page.getByRole('dialog')).not.toBeVisible();
	await expect(page.getByText('Feedback sent.', { exact: false })).toBeVisible();
	// The focused element is the Notice's own wrapper (StaffFeedback.svelte's
	// `.notice`, tabindex="-1"), not the <p role="status"> text inside it --
	// no accessible role names that wrapper, so this is the CSS-selector
	// exception rather than a shortcut past an accessible query.
	await expect(page.locator('.notice')).toBeFocused();
});

/**
 * The Portal's own instance (#1528): the same walk, from a Client-portal
 * screen, reaching `POST /api/portal/feedback` (#1523) with the
 * Engagement the screen sits inside. Same outbox caveat as the Staff
 * test above.
 */
test('sends a piece of Feedback from a Portal screen', async ({ page, request }) => {
	const { clientEmail, engagementId } = await seedPortalClient(request, 'Meadowbrook Doulas');

	await Promise.all([
		portalEngagementSettled(page, engagementId),
		signInPortalClient(page, request, clientEmail)
	]);
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	await page.getByRole('button', { name: 'Tell us what is not working or what you need.' }).click();
	await expect(page.getByRole('dialog', { name: 'Send feedback about this portal' })).toBeVisible();

	await page.getByLabel('Something is not working').check();
	await page.getByLabel('Tell us more').fill('The contract PDF will not download.');

	const sent = page.waitForResponse(
		(response) => response.url().endsWith('/api/portal/feedback') && response.request().method() === 'POST'
	);
	await page.getByRole('button', { name: 'Send feedback' }).click();
	const response = await sent;
	expect(response.status()).toBe(201);

	await expect(page.getByRole('dialog')).not.toBeVisible();
	await expect(page.getByText('Feedback sent.', { exact: false })).toBeVisible();
	// Same querySelector exception as the Staff test above --
	// PortalFeedback.svelte's own `.notice` wrapper.
	await expect(page.locator('.notice')).toBeFocused();
});
