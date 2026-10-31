import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { signIn } from './auth';
import { readStaffInviteToken } from './stack';
import { staffLandingSettled } from './mountSettled';
import { seedFoundingOwner } from './staffSignup';

// #2265: a person who already belongs to one Practice accepts a second
// Practice's Invitation on the account she has. She lands on the Practice
// she joined, and that Practice is the one her next sign-in goes to.
test('accepting a second Practice lands there, and the next sign-in goes there too', async ({
	page,
	request,
	context
}) => {
	const sasha = await seedFoundingOwner(request, { practiceName: 'Rooted Test Collective', staffName: 'Sasha Two' });
	const sashaAtA = await signIn(request, API_URL, sasha.idToken);
	const atA = await request.get(`${API_URL}/api/practices/${sasha.practiceId}/clients`, { headers: sashaAtA });
	expect(atA.ok(), `reading Practice A failed: ${atA.status()}`).toBe(true);
	const before = await request.get(`${API_URL}/api/staff/session`, { headers: sashaAtA });
	const { lastPracticeId } = await before.json();
	expect(lastPracticeId).toBe(sasha.practiceId);

	const other = await seedFoundingOwner(request, { practiceName: 'Ridgeline Test Doulas', staffName: 'Morgan Owner' });
	const otherHeaders = await signIn(request, API_URL, other.idToken);
	const invite = await request.post(`${API_URL}/api/practices/${other.practiceId}/staff/invitations`, {
		headers: otherHeaders,
		data: { email: sasha.email, roles: ['doula'], employmentType: 'contractor' }
	});
	const inviteBody = await invite.text();
	expect(invite.ok(), `invite failed: ${invite.status()} ${inviteBody}`).toBe(true);
	const inviteToken = await readStaffInviteToken(request, JSON.parse(inviteBody).invitationId);

	await page.goto(`/accept-invite?token=${inviteToken}`);
	await page.getByLabel('Password').fill(sasha.password);
	const landed = staffLandingSettled(page, other.practiceId);
	await page.getByRole('button', { name: 'Accept invite' }).click();
	await landed;

	await expect(page).toHaveURL(new RegExp(`/practices/${other.practiceId}$`));
	const joined = page.getByRole('status').filter({
		hasText: "You've joined Ridgeline Test Doulas. You're a Doula, as a contractor. Your other Practices are unchanged."
	});
	await expect(joined).toBeVisible();
	await expect(joined).toBeFocused();

	await context.clearCookies();
	await page.goto('/login');
	await page.getByLabel('Email address').fill(sasha.email);
	await page.getByLabel('Password').fill(sasha.password);
	await page.getByRole('button', { name: 'Sign in' }).click();
	await expect(page).toHaveURL(new RegExp(`/practices/${other.practiceId}$`));
});
