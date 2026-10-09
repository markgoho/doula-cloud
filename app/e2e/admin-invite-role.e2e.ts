import { expect, test } from '@playwright/test';
import { signInEnrolled, enterPracticeAsEnrolled } from './mfa';
import { readStaffInviteToken } from './stack';
import { seedFoundingOwner, uniqueEmail, STAFF_PASSWORD } from './staffSignup';

// The Admin bundle -- `admin` without `owner` and without `doula` -- is
// the one role set no other spec reaches the app as (#965). Every spec that
// signs in does so as the Owner signup creates, who holds all three, and
// staff-invite-role.e2e.ts invites a Doula. Since #267 and #269 the Admin
// sits on one side of the OwnerAndAdmin/OwnerOnly line in staffauth's role
// declarations, so this walks her through the real invite and accept
// screens and asserts one screen ADR-0008 grants her and one it refuses her.
// The server-side table already carries the role rules; this is the browser
// path nothing else guards.
test('An Admin invited via the Staff invite route reaches the Credits and Staff screens and is refused an Owner-only action', async ({
	page,
	request,
	context
}) => {
	const adminEmail = uniqueEmail('admin');
	const password = STAFF_PASSWORD;

	// Fixture setup, not the seam under test (#207): the Owner side is
	// provisioned the way every other spec provisions its Practice.
	const { idToken: ownerIdToken, localId: ownerUID, practiceId } = await seedFoundingOwner(request);

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route (see mfa.ts's signInEnrolled doc comment).
	const ownerHeaders = await signInEnrolled(request, ownerIdToken, ownerUID);
	await enterPracticeAsEnrolled(context, page, ownerHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	// The invite form's Roles default is 'doula' alone: select Admin and
	// clear Doula, so the role plumbing is what decides the membership and
	// a regression that ignored the checkboxes would grant the default.
	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Their email').fill(adminEmail);
	await page.getByRole('checkbox', { name: 'Admin' }).check();
	await page.getByRole('checkbox', { name: 'Doula' }).uncheck();
	const [inviteResponse] = await Promise.all([
		page.waitForResponse((response) => response.url().endsWith('/staff/invitations') && response.request().method() === 'POST'),
		page.getByRole('button', { name: 'Send invite' }).click()
	]);
	expect(inviteResponse.ok(), `invite send failed: ${inviteResponse.status()}`).toBe(true);
	expect(inviteResponse.request().postDataJSON().roles).toEqual(['admin']);
	const { invitationId } = await inviteResponse.json();
	await expect(
		page.getByText(`Invited. An email with a link to join is on its way to ${adminEmail}.`)
	).toBeVisible();

	// readStaffInviteToken (stack.ts) reads the token off the pending outbox
	// row, or the sandbox mailbox if a parallel spec's drain mailed it first
	// (#827).
	const inviteToken = await readStaffInviteToken(request, invitationId);

	// Accepting is walked through the real two-step screen (#437): a
	// brand-new person, so the signup branch and both questions on step two.
	await page.goto(`/accept-invite?token=${inviteToken}`);
	await page.getByLabel('Email').fill(adminEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page.getByRole('heading', { name: 'Tell us about yourself' })).toBeVisible();
	await page.getByLabel('First name').fill('Avery');
	await page.getByLabel('Last name').fill('Admin');
	await page.getByLabel('Which state do you work from?').selectOption('New York');
	await page.getByRole('button', { name: 'Accept invite' }).click();

	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));
	await expect(page.locator('h1')).toHaveText('Welcome to Riverside Doulas');

	// Still signed in as the Admin. The Staff roster is OwnerAndAdmin, and
	// her own row carries exactly the role that was selected: Admin, and
	// neither Owner nor Doula.
	await page.goto(`/practices/${practiceId}/staff`);
	const ownRow = page.getByRole('row', { name: /Avery Admin/ });
	await expect(ownRow).toBeVisible();
	await expect(ownRow.getByRole('cell', { name: 'Admin', exact: true })).toBeVisible();

	// The Credits screen is OwnerAndAdmin too (#272): a Doula meets the
	// refusal screen there, the Admin reads the Practice's balance.
	await page.goto(`/practices/${practiceId}/billing`);
	await expect(page.getByText('Credit balance: 3')).toBeVisible();
	await expect(page.getByRole('cell', { name: 'Welcome credits' })).toBeVisible();

	// The invite send is Owner-only: the same screen that just worked for
	// Jamie refuses Avery, with the sentence the route's own OwnerOnly
	// declaration names (#1031).
	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Their email').fill(uniqueEmail('someone-else'));
	await page.getByRole('button', { name: 'Send invite' }).click();
	await expect(page.getByText('only a Practice Owner can do that')).toBeVisible();
});
