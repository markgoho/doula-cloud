import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, signIn } from './auth';
import { readStaffInviteToken } from './stack';
import { seedFoundingOwner, uniqueEmail, STAFF_PASSWORD } from './staffSignup';

// Every other spec that signs in does so as the Owner signup itself creates,
// who holds owner + admin + doula at once. This is the one spec
// that reaches the app as a Staff member holding a role set that is not
// that bundle -- a Doula invited and accepted through the real invite
// route -- and the one spec that asserts a role refusal (a 403 where a
// role rule says there must be one), rather than only success paths.
test('A Doula invited via the Staff invite route is refused an Owner-only action', async ({
	page,
	request,
	context
}) => {
	const doulaEmail = uniqueEmail('doula');
	const password = STAFF_PASSWORD;

	// Fixture setup, not the seam under test (#207): the Owner side of this
	// spec is provisioned the way every other spec provisions its Practice.
	const { idToken: ownerIdToken, practiceId } = await seedFoundingOwner(request);

	// The invite send is walked through the real screen (#189's "Invite a
	// Staff member" route, otherwise untouched by the suite). The form opens
	// with nothing chosen (#1912), so the Owner ticks Doula alone, the
	// non-Owner bundle this spec needs, and Employee.
	const ownerHeaders = await signIn(request, API_URL, ownerIdToken);
	await enterPractice(context, page, ownerHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Email address').fill(doulaEmail);
	await page.getByRole('checkbox', { name: 'Doula' }).check();
	await page.getByRole('radio', { name: 'Employee' }).check();
	const [inviteResponse] = await Promise.all([
		page.waitForResponse((response) => response.url().endsWith('/staff/invitations') && response.request().method() === 'POST'),
		page.getByRole('button', { name: 'Send invite' }).click()
	]);
	expect(inviteResponse.ok(), `invite send failed: ${inviteResponse.status()}`).toBe(true);
	const { invitationId } = await inviteResponse.json();
	await expect(
		page.getByText(`Invited. An email with a link to join is on its way to ${doulaEmail}.`)
	).toBeVisible();

	// readStaffInviteToken (stack.ts) reads the token off the pending
	// outbox row, or out of the sandbox mailbox if a parallel spec's drain
	// mailed it first (#827). It throws when it is in neither, so there is
	// nothing left here to assert.
	const inviteToken = await readStaffInviteToken(request, invitationId);

	// Accepting is walked through the real screen too (#437's two-step
	// form): a brand-new person, so the signup branch and both questions on
	// step two.
	await page.goto(`/accept-invite?token=${inviteToken}`);
	await page.getByRole('radio', { name: /I'm new here/ }).check();
	await page.getByLabel('Email').fill(doulaEmail);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page.getByRole('heading', { name: 'Tell us about yourself' })).toBeVisible();
	await page.getByLabel('First name').fill('Robin');
	await page.getByLabel('Last name').fill('Doula');
	await page.getByLabel('Which state do you work from?').selectOption('New York');
	await page.getByRole('button', { name: 'Accept invite' }).click();

	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));
	await expect(page.locator('h1')).toHaveText('Overview');

	// Still signed in as the Doula: the same Owner-only screen that just
	// worked for Jamie refuses Robin outright -- the role rule PR-B2 names
	// in employed-doula.md's permission boundary. Since #1028 that refusal
	// comes from the route's own staffauth.OwnerOnly declaration at the
	// mount rather than a RequireOwner call inside InviteHandler, and
	// requireAnyRole names the seat from that declaration (#1031), so the
	// sentence is the one RequireOwner wrote before the move.
	await page.goto(`/practices/${practiceId}/invite`);
	await page.getByLabel('Email address').fill(uniqueEmail('someone-else'));
	await page.getByRole('checkbox', { name: 'Doula' }).check();
	await page.getByRole('radio', { name: 'Employee' }).check();
	await page.getByRole('button', { name: 'Send invite' }).click();
	await expect(page.getByText('only a Practice Owner can do that')).toBeVisible();
});
