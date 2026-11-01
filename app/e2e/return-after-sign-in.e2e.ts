import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { enterPractice, signIn } from './auth';
import { staffLandingSettled } from './mountSettled';
import { seedEngagement } from './stack';
import { seedFoundingOwner, uniqueEmail } from './staffSignup';
import { followMagicLink, seedPortalClient } from './portalClient';

function endsWith(path: string): RegExp {
	return new RegExp(`${path.replaceAll(/[.*+?^${}()|[\]\\]/g, String.raw`\$&`)}$`);
}

const SIGNED_OUT_NOTICE = 'For your security, we signed you out. Sign in again to continue.';

// #2259: a Birth Plan address she saved, opened on a device that has never
// signed in, says nothing about being signed out and comes back after sign-in.
test('a saved Birth Plan address opened signed out lands on that Birth Plan after sign-in', async ({
	page,
	request
}) => {
	const { email, password, idToken, practiceId } = await seedFoundingOwner(request);
	const headers = await signIn(request, API_URL, idToken);
	const created = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers,
		data: { givenName: 'Pat', familyName: 'Client', email: uniqueEmail('client') }
	});
	expect(created.ok(), `create client failed: ${created.status()} ${await created.text()}`).toBe(true);
	const { id: clientId } = await created.json();
	const engagementId = seedEngagement(clientId, practiceId);
	const birthPlan = `/practices/${practiceId}/engagements/${engagementId}/birth-plan`;

	await page.goto(birthPlan);

	await expect(page).toHaveURL(endsWith(`/login?returnTo=${encodeURIComponent(birthPlan)}`));
	await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
	await expect(page.getByText(SIGNED_OUT_NOTICE)).toHaveCount(0);

	await page.getByLabel('Email address').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Sign in' }).click();

	await expect(page).toHaveURL(endsWith(birthPlan));
	await expect(page.getByRole('heading', { level: 1, name: "Pat Client's Birth Plan" })).toBeVisible();
});

// #2496: the portal's sign-in is a mailed link, so the address she opened
// rides on the link's token at the BFF and survives the round trip.
test('a saved portal Birth Plan address opened signed out lands on that Birth Plan after the magic link', async ({
	page,
	request
}) => {
	const { clientEmail, engagementId } = await seedPortalClient(request, 'Riverside Doulas');
	const birthPlan = `/portal/engagements/${engagementId}/birth-plan`;

	await page.goto(birthPlan);

	await expect(page).toHaveURL(endsWith(`/portal/login?returnTo=${encodeURIComponent(birthPlan)}`));
	await page.getByLabel('Email address').fill(clientEmail);
	await page.getByRole('button', { name: 'Send me a sign-in link' }).click();
	await expect(page.getByText(/we have sent a sign-in link/i)).toBeVisible();

	await followMagicLink(page, request, clientEmail);
	await page.getByRole('button', { name: 'Continue' }).click();

	await expect(page).toHaveURL(endsWith(birthPlan));
	await expect(page.getByRole('heading', { level: 1, name: 'Birth plan' })).toBeVisible();
});

// #2534: a dead link still names the page it was asked for from, at the
// BFF, so the new link she asks for from it lands there too. Used here,
// on another device; the BFF test owns the expired one, which is the same
// refusal.
test('a used portal sign-in link asks for a new one that still lands on the saved Birth Plan', async ({
	page,
	browser,
	request
}) => {
	const { clientEmail, engagementId } = await seedPortalClient(request, 'Riverside Doulas');
	const birthPlan = `/portal/engagements/${engagementId}/birth-plan`;

	await page.goto(birthPlan);
	await page.getByLabel('Email address').fill(clientEmail);
	await page.getByRole('button', { name: 'Send me a sign-in link' }).click();
	await expect(page.getByText(/we have sent a sign-in link/i)).toBeVisible();
	await followMagicLink(page, request, clientEmail);
	const usedLink = page.url();
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page).toHaveURL(endsWith(birthPlan));

	const other = await browser.newContext();
	const phone = await other.newPage();
	await phone.goto(usedLink);
	await phone.getByRole('button', { name: 'Continue' }).click();
	await expect(phone.getByRole('alert')).toContainText('This link has expired or was already used.');
	await phone.getByRole('link', { name: 'Ask for a new sign-in link' }).click();

	await expect(phone).not.toHaveURL(/returnTo=/);
	await phone.getByLabel('Email address').fill(clientEmail);
	await phone.getByRole('button', { name: 'Send me a sign-in link' }).click();
	await expect(phone.getByText(/we have sent a sign-in link/i)).toBeVisible();
	await followMagicLink(phone, request, clientEmail, { nth: 2 });
	await phone.getByRole('button', { name: 'Continue' }).click();

	await expect(phone).toHaveURL(endsWith(birthPlan));
	await expect(phone.getByRole('heading', { level: 1, name: 'Birth plan' })).toBeVisible();
	await other.close();
});

// #757's notice still says so when the session ends under her in the app,
// and she still comes back to where she was going.
test('a session that ends while she is in the app says so and returns her after sign-in', async ({
	page,
	request,
	context
}) => {
	const { email, password, idToken, practiceId } = await seedFoundingOwner(request, { staffName: 'Robin Owner' });
	const headers = await signIn(request, API_URL, idToken);
	await Promise.all([staffLandingSettled(page, practiceId), enterPractice(context, page, headers, practiceId)]);

	await context.clearCookies();
	await page.getByRole('navigation', { name: 'Practice' }).getByRole('link', { name: 'Staff' }).click();

	const staff = `/practices/${practiceId}/staff`;
	await expect(page).toHaveURL(endsWith(`/login?sessionEnded=true&returnTo=${encodeURIComponent(staff)}`));
	await expect(page.getByText(SIGNED_OUT_NOTICE)).toBeVisible();

	await page.getByLabel('Email address').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Sign in' }).click();

	await expect(page).toHaveURL(endsWith(staff));
	await expect(page.getByRole('heading', { level: 1, name: 'Staff' })).toBeVisible();
});
