// #1549's e2e, built on #1833's seams: a material version of the Terms
// of Service starts, a Doula works as usual before any Owner agrees, and
// the Owner agrees on the screen at her next sign-in.
//
// It runs in the clock project (e2e/clockProject.ts): it restarts the BFF
// and moves the clock 31 days, which would end every other spec's session.
//
// The order is the order a real change takes. The Practice signs up on
// the shipped history, because a Practice that signs up while a version
// is coming has agreed to it at signup (ADR-0053's #1816 amendment) and
// is never asked. Then the version is written into the history -- here,
// the BFF restarts with it (stackswitch.LegalVar), the shape of a deploy
// -- and published 31 days ahead, and the clock moves past its date.
//
// Each attempt computes its date from the clock as it finds it, so a
// retry after the clock has moved still leaves 30 days of notice.
import { expect, test, type APIRequestContext } from '@playwright/test';
import { signIn } from './auth';
import { API_URL, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT, PREVIEW_SERVER_ORIGIN } from './ports';
import { seedClient } from './portalClient';
import { jump } from './simulation/clock';
import { WORKER_SECRET, readStaffInviteToken, restartAPI, simulatedDateAfter } from './stack';
import { STAFF_PASSWORD, acceptStaffInvite, seedFoundingOwner, uniqueEmail } from './staffSignup';

const EMULATOR_URL = `http://${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`;
// More than legalnotice.NoticeLead's 30 days, counted from today's
// midnight UTC, so the version can be published at any hour of today.
const NOTICE_DAYS = 31;
const CHANGE = 'Says how long a Practice has to pay for a Credit.';

// An employed Doula at practiceId, invited and accepted through the real
// endpoints, so her Membership is what any accepted invite produces.
async function seedEmployedDoula(request: APIRequestContext, practiceId: string, ownerHeaders: { Cookie: string }) {
	const email = uniqueEmail('doula');
	const invite = await request.post(`${API_URL}/api/practices/${practiceId}/staff/invitations`, {
		headers: ownerHeaders,
		data: { email, roles: ['doula'], employmentType: 'employee' }
	});
	expect(invite.ok(), `doula invite failed: ${invite.status()} ${await invite.text()}`).toBe(true);
	const { invitationId } = await invite.json();
	const inviteToken = await readStaffInviteToken(request, invitationId);

	const signUp = await request.post(`${EMULATOR_URL}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-key`, {
		data: { email, password: STAFF_PASSWORD, returnSecureToken: true }
	});
	expect(signUp.ok(), `doula signUp failed: ${signUp.status()} ${await signUp.text()}`).toBe(true);
	const { idToken } = await signUp.json();
	const accept = await acceptStaffInvite(request, idToken, { inviteToken, name: 'Robin Doula' });
	expect(accept.ok(), `doula accept-invite failed: ${accept.status()} ${await accept.text()}`).toBe(true);
	return { email, password: STAFF_PASSWORD };
}

test.describe('After a material change to the Terms starts', () => {
	test.afterAll(async () => {
		// The usual BFF, with the shipped history, for whatever runs next.
		await restartAPI(PREVIEW_SERVER_ORIGIN, {});
	});

	test('a Doula opens a Client before any Owner agrees, and the Owner agrees at her next sign-in', async ({ page, browser, request }) => {
		const owner = await seedFoundingOwner(request, { practiceName: 'Lakeside Doulas', staffName: 'Jamie Owner' });
		const ownerHeaders = await signIn(request, API_URL, owner.idToken);
		const clientId = await seedClient(request, owner.practiceId, ownerHeaders, { givenName: 'Ada' });
		// Before the jump: an invitation expires by the BFF's clock.
		const doula = await seedEmployedDoula(request, owner.practiceId, ownerHeaders);

		const effective = simulatedDateAfter(NOTICE_DAYS);
		await restartAPI(PREVIEW_SERVER_ORIGIN, {
			E2E_LEGAL_VERSIONS: JSON.stringify([{ document: 'terms', effective, material: true, change: CHANGE }])
		});

		const published = await request.post(`${API_URL}/api/internal/legal/publish`, {
			headers: { 'X-Internal-Secret': WORKER_SECRET },
			data: { document: 'terms', version: effective, publishedBy: 'terms-agreement.e2e.ts' }
		});
		expect(published.ok(), `publish failed: ${published.status()} ${await published.text()}`).toBe(true);
		expect(await published.json()).toMatchObject({ document: 'terms', version: effective, material: true });

		await jump({ value: NOTICE_DAYS, unit: 'days' }, { label: 'terms-agreement: the new Terms start', workerSecret: WORKER_SECRET });

		// The Doula is not an Owner: she is never asked, and the Practice
		// works for her as usual while it waits on an Owner.
		await page.goto('/login');
		await page.getByLabel('Email').fill(doula.email);
		await page.getByLabel('Password').fill(doula.password);
		await page.getByRole('button', { name: 'Log in' }).click();
		await expect(page).toHaveURL(new RegExp(`/practices/${owner.practiceId}$`));
		await page.goto(`/practices/${owner.practiceId}/clients/${clientId}`);
		await expect(page).toHaveURL(new RegExp(`/practices/${owner.practiceId}/clients/${clientId}$`));
		await expect(page.getByRole('heading', { level: 1, name: /Ada/ })).toBeVisible();

		const ownerContext = await browser.newContext({ baseURL: PREVIEW_SERVER_ORIGIN });
		const ownerPage = await ownerContext.newPage();
		await ownerPage.goto('/login');
		await ownerPage.getByLabel('Email').fill(owner.email);
		await ownerPage.getByLabel('Password').fill(owner.password);
		await ownerPage.getByRole('button', { name: 'Log in' }).click();
		await expect(ownerPage).toHaveURL(new RegExp(`/practices/${owner.practiceId}/agreement$`));
		await expect(ownerPage.getByRole('heading', { level: 1, name: 'The Terms of Service have changed' })).toBeVisible();
		await expect(ownerPage.getByText(CHANGE)).toBeVisible();

		await ownerPage.getByRole('button', { name: 'Agree and continue' }).click();
		await expect(ownerPage).toHaveURL(new RegExp(`/practices/${owner.practiceId}$`));

		// Two rows: the agreement at signup, and this one, on the day the
		// new version started. Wider than the activity ledger's own floor
		// (#1945), so it draws its <table> and the rows are rows.
		await ownerPage.setViewportSize({ width: 3200, height: 900 });
		const agreements = ownerPage.getByRole('row').filter({ hasText: 'Terms accepted' });
		await expect(agreements).toHaveCount(2);
		const agreedOnTheDay = agreements.filter({ has: ownerPage.locator(`time[datetime^="${effective}"]`) });
		await expect(agreedOnTheDay).toHaveCount(1);
		await expect(agreedOnTheDay).toContainText('Jamie Owner');

		await ownerContext.close();
	});
});
