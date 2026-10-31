import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, sessionCookieFrom, signIn } from './auth';
import { readStaffInviteToken } from './stack';
import { staffLandingSettled } from './mountSettled';
import { acceptStaffInvite, seedFoundingOwner, uniqueEmail } from './staffSignup';

// #2272: one person holding Memberships at two Practices switches from
// one to the other in the header. Both Overviews are the same route, so
// SvelteKit keeps the page mounted across the move; before the fix its
// reads ran once, on mount, and the first Practice's Clients and
// activity stayed on screen under the second Practice's name until a
// reload.
test('switching Practice from the header draws the second Practice, with nothing of the first', async ({
	page,
	request,
	context
}) => {
	// Practice A: she is its Owner, and it holds a Client.
	const sasha = await seedFoundingOwner(request, { practiceName: 'Rooted Test Collective', staffName: 'Sasha Two' });
	const sashaAtA = await signIn(request, API_URL, sasha.idToken);
	const createClient = await request.post(`${API_URL}/api/practices/${sasha.practiceId}/clients`, {
		headers: sashaAtA,
		data: { givenName: 'Reese', familyName: 'Bishop', email: uniqueEmail('client') }
	});
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${await createClient.text()}`).toBe(true);

	// Practice B: another Owner's, with no Client, where Sasha joins as a
	// contractor Doula.
	const other = await seedFoundingOwner(request, { practiceName: 'Ridgeline Test Doulas', staffName: 'Morgan Owner' });
	const otherHeaders = await signIn(request, API_URL, other.idToken);
	const invite = await request.post(`${API_URL}/api/practices/${other.practiceId}/staff/invitations`, {
		headers: otherHeaders,
		data: { email: sasha.email, roles: ['doula'], employmentType: 'contractor' }
	});
	const inviteBody = await invite.text();
	expect(invite.ok(), `invite failed: ${invite.status()} ${inviteBody}`).toBe(true);
	const inviteToken = await readStaffInviteToken(request, JSON.parse(inviteBody).invitationId);
	const accept = await acceptStaffInvite(request, sasha.idToken, { inviteToken, name: 'Sasha Two' });
	expect(accept.ok(), `accept failed: ${accept.status()} ${await accept.text()}`).toBe(true);
	const sashaHeaders = sessionCookieFrom(accept, 'accept-invite');

	const firstSettled = staffLandingSettled(page, sasha.practiceId);
	await enterPractice(context, page, sashaHeaders, sasha.practiceId);
	await firstSettled;
	// DataTable draws two trees and shows one.
	await expect(page.getByText('Client record created for Reese Bishop').filter({ visible: true })).toBeVisible();

	// The switcher is a menu button named for the Practice she is in.
	// Not staffLandingSettled: the layout's who-am-I read is not made again
	// on a move inside it, so only the page's own reads answer here.
	await page.getByRole('button', { name: 'Rooted Test Collective' }).filter({ visible: true }).click();
	await page.getByRole('link', { name: /Ridgeline Test Doulas/ }).filter({ visible: true }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${other.practiceId}$`));
	await page.getByRole('status', { name: 'Loading your Practice' }).waitFor({ state: 'detached' });

	// #2335: the switcher's panel shut as she chose, rather than staying
	// open over the new Overview, and focus is where the router puts it
	// after any link: the body, the starting point for the new page's Tab
	// order, not back on the switcher.
	await expect(page.getByRole('link', { name: /Rooted Test Collective/ }).filter({ visible: true })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Ridgeline Test Doulas' }).filter({ visible: true })).toHaveAttribute(
		'aria-expanded',
		'false'
	);
	await expect.poll(() => page.evaluate(() => document.activeElement === document.body)).toBe(true);

	// She is a contractor there, so its empty Overview says how work
	// reaches her (#2273).
	await expect(page.getByText(/Work at this Practice reaches you as an Offer/)).toBeVisible();
	await expect(page.getByText(/Reese Bishop/)).toHaveCount(0);
	await expect(page.getByRole('heading', { name: 'Recent activity' })).toHaveCount(0);
});
