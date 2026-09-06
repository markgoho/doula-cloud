import { expect, test } from '@playwright/test';
import { signInEnrolled, enterPracticeAsEnrolled } from './mfa';
import { seedFoundingOwner } from './staffSignup';

// Exercises the Billing screen from app/src/routes/practices/[practiceId]/billing end-to-end --
// billing.ts has its own Vitest coverage, but this is the only test that renders the actual route
// and hits the real API, proving the signup-bonus grant (#74) surfaces through GetBalanceHandler
// (#75) and the page.
test('Staff member can view the signup-bonus balance and ledger history', async ({ page, request, context }) => {
	const { idToken, localId, practiceId } = await seedFoundingOwner(request);

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route (see mfa.ts's signInEnrolled doc comment).
	const staffHeaders = await signInEnrolled(request, idToken, localId);
	await enterPracticeAsEnrolled(context, page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.getByRole('link', { name: 'Billing' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}/billing$`));

	// A brand-new Practice's balance is the +3 signup_bonus grant from
	// staffauth.signup, and nothing else. The cell reads "Welcome
	// credits", not the enum value -- #449 gave the ledger a second kind
	// of grant, and the screen names both in words.
	await expect(page.getByText('Credit balance: 3')).toBeVisible();
	await expect(page.getByRole('cell', { name: 'Welcome credits' })).toBeVisible();
	await expect(page.getByRole('cell', { name: '+3' })).toBeVisible();
});
