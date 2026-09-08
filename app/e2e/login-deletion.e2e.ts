import { expect, test, type APIRequestContext } from '@playwright/test';
import { E2E_API_HOST, E2E_API_PORT, PREVIEW_SERVER_ORIGIN } from './ports';
import { seedFoundingOwner } from './staffSignup';
import { signInEnrolled } from './mfa';
import { seedContractorDoula } from './portalClient';

// #892: a Staff person deleting her own login -- ADR-0033's rule. This
// spec proves the two things unit tests cannot: that the real /account
// screen actually carries the control and the confirmation, and that the
// deletion really does end her ability to authenticate against the
// running BFF afterwards -- a redacted row in a test database is not the
// same claim as a live session cookie that stops working.
const API_URL = `http://${E2E_API_HOST}:${E2E_API_PORT}`;

async function useSession(
	context: { addCookies: (cookies: unknown[]) => Promise<void> },
	headers: { Cookie: string }
) {
	await context.addCookies([
		{
			name: '__session',
			value: headers.Cookie.replace('__session=', ''),
			url: PREVIEW_SERVER_ORIGIN,
			httpOnly: true,
			secure: false,
			sameSite: 'Lax'
		}
	]);
}

async function seedEnrolledOwner(request: APIRequestContext, practiceName: string) {
	const { idToken, localId, practiceId } = await seedFoundingOwner(request, { practiceName });
	const headers = await signInEnrolled(request, idToken, localId);
	return { practiceId, headers };
}

test('a doula deletes her own login from /account, and cannot get back in', async ({
	page,
	context,
	request
}) => {
	const { practiceId, headers: ownerHeaders } = await seedEnrolledOwner(request, 'Genesee Doulas');
	const doula = await seedContractorDoula(request, practiceId, ownerHeaders);
	await useSession(context, doula.headers);

	await page.goto('/account');

	// What it destroys and what it keeps, stated before she presses
	// anything -- the AC's own requirement, on the real page.
	await expect(page.getByText(/ends your access to Doula Cloud everywhere/)).toBeVisible();
	await expect(page.getByText(/membership of every practice you work at/).first()).toBeVisible();
	await expect(page.getByText(/Everything you did stays with the practices/).first()).toBeVisible();

	await page.getByRole('button', { name: 'Delete your login' }).first().click();

	const dialog = page.getByRole('dialog');
	await expect(dialog.getByRole('heading', { name: 'Delete your login' })).toBeVisible();
	await expect(dialog.getByText(/You cannot sign in again/)).toBeVisible();
	await dialog.getByRole('button', { name: 'Delete your login' }).click();

	// She lands signed out. The cookie she arrived with is a dead token
	// now, so the BFF refuses it rather than answering with her account.
	await expect(page).toHaveURL(/\/login$/);

	const afterwards = await request.get(`${API_URL}/api/staff/session`, { headers: doula.headers });
	expect(afterwards.status()).toBe(401);

	// And she is off the roster, in the same act -- no Owner had to
	// remove her.
	const roster = await request.get(`${API_URL}/api/practices/${practiceId}/staff`, {
		headers: ownerHeaders
	});
	expect(roster.ok()).toBe(true);
	expect(await roster.text()).not.toContain(doula.email);
});

test('a sole Owner is refused, with the practice in the way named', async ({
	page,
	context,
	request
}) => {
	const { headers } = await seedEnrolledOwner(request, 'Lakeshore Birth Partners');
	await useSession(context, headers);

	await page.goto('/account');
	await page.getByRole('button', { name: 'Delete your login' }).first().click();
	await page.getByRole('dialog').getByRole('button', { name: 'Delete your login' }).click();

	await expect(page.getByText(/only Owner of Lakeshore Birth Partners/)).toBeVisible();
	// Still signed in, and still on the page she was on.
	await expect(page).toHaveURL(/\/account$/);
	const stillWorks = await request.get(`${API_URL}/api/staff/session`, { headers });
	expect(stillWorks.ok()).toBe(true);
});
