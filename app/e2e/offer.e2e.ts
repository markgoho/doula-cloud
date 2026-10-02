import { expect, test } from '@playwright/test';
import { API_URL } from './ports';
import { seedEngagement } from './stack';
import { signInEnrolled, enterPracticeAsEnrolled } from './mfa';
import { seedContractorDoula } from './portalClient';
import { seedFoundingOwner, uniqueEmail } from './staffSignup';

// Drives ADR-0008's Offer flow (#317) through the two screens it adds:
// the make-an-offer panel on the Engagement, and the Doula's own inbox.
// offer.ts, OfferSection.svelte, and OfferInbox.svelte all have their own
// Vitest coverage; this is the only test that renders the real routes and
// hits the real API, proving the Offer lands, appears to the person
// offered, and mints her attachment when she takes it.
//
// Two people, because an Offer is one person asking another (#1598):
// the founding Owner sends, and a contractor Doula who joined through a
// real Staff invitation (seedContractorDoula) receives. The API refuses an
// Offer whose target is its sender, and the form does not list her.
test('Owner offers an Engagement to a Doula, who accepts it from her own inbox', async ({
	page,
	request,
	context
}) => {
	const { idToken, localId, practiceId } = await seedFoundingOwner(request);

	// #606: an Owner is gated behind a second factor at every Practice-scoped
	// route (see mfa.ts's signInEnrolled doc comment), including the
	// invitation and createClient calls right below.
	const headers = await signInEnrolled(request, idToken, localId);

	// A contractor, which is what makes the fee required.
	const doula = await seedContractorDoula(request, practiceId, headers);

	const createClient = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers,
		data: { givenName: 'Rosa', familyName: 'Martinez', email: uniqueEmail('client') }
	});
	const createClientBody = await createClient.text();
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${createClientBody}`).toBe(true);
	const { id: clientId } = JSON.parse(createClientBody);
	const engagementId = seedEngagement(clientId, practiceId);

	await enterPracticeAsEnrolled(context, page, headers, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	await expect(page.getByRole('heading', { name: 'Offers' })).toBeVisible();
	await expect(page.getByText('Nobody has been offered this work yet.')).toBeVisible();

	// #1598: the person at the screen holds the Doula role too (a founding
	// Owner signs up with it), and she is not offered her own work.
	await expect(page.getByRole('group', { name: 'Doula', exact: true })).toBeVisible();
	await expect(page.getByLabel('Jamie Owner')).toHaveCount(0);
	await page.getByLabel('Casey Contractor').check();
	await page.getByLabel('Fee (USD)').fill('450');
	await page.getByLabel('General area').fill('North side');
	await page.getByLabel('Due date').fill('2027-01-04');
	await page.getByLabel('Terms').fill('Two prenatal visits, on call from 38 weeks.');
	await page.getByRole('button', { name: 'Send Offer' }).click();

	// The Practice side now names who was asked and what she has said.
	await expect(page.getByText('Awaiting a decision')).toBeVisible();
	await expect(page.getByText('$450.00')).toBeVisible();

	// And the Doula's own inbox carries the four decidable facts, with no
	// Client name anywhere on it. Her session replaces the Owner's in this
	// browser; a plain Doula has no second factor to pass (#606).
	await enterPracticeAsEnrolled(context, page, doula.headers, practiceId);
	await page.goto(`/practices/${practiceId}/offers`);
	await expect(page.getByRole('heading', { name: 'Your offers' })).toBeVisible();
	// exact: true -- #515's hidden "North side, due 2027-01-04" sibling
	// (naming the Accept/Decline buttons) otherwise substring-matches too.
	await expect(page.getByText('North side', { exact: true })).toBeVisible();
	await expect(page.getByText('2027-01-04', { exact: true })).toBeVisible();
	await expect(page.getByText('Two prenatal visits, on call from 38 weeks.')).toBeVisible();
	await expect(page.getByText('Rosa Martinez')).toHaveCount(0);

	await page.getByRole('button', { name: 'Accept' }).click();
	await expect(page.getByText('Accepted')).toBeVisible();
	await expect(page.getByRole('button', { name: 'Accept' })).toHaveCount(0);
	// #230's terminal rule, through the real stack: the Client's own
	// fields stop being served the moment the Offer leaves 'offered', so
	// the row keeps her fee and loses the area and the date.
	await expect(page.getByText('North side')).toHaveCount(0);
	await expect(page.getByText('2027-01-04')).toHaveCount(0);
	await expect(page.getByText('$450.00')).toBeVisible();

	// And the Practice side reads the same answer back. { exact: true }
	// because the page now also carries #486's own Activity ledger, whose
	// "Offer accepted" row is a substring match for a bare 'Accepted'
	// query too.
	await enterPracticeAsEnrolled(context, page, headers, practiceId);
	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	await expect(page.getByText('Accepted', { exact: true })).toBeVisible();
	// Her acceptance put her on the Engagement, and the summary names her.
	await expect(page.getByText('Casey Contractor', { exact: true }).first()).toBeVisible();
});

// #1598: the path that is not an Offer. A solo Owner (Owner, Admin and
// employee Doula from signup) opens an Engagement that started with No
// Doula yet. The Offers form has nobody to list, because its only Doula
// is the person at the screen, and one press puts her on the Engagement.
test('A solo Owner puts herself on an Engagement that started with no Doula', async ({
	page,
	request,
	context
}) => {
	const { idToken, localId, practiceId } = await seedFoundingOwner(request);
	const headers = await signInEnrolled(request, idToken, localId);

	const createClient = await request.post(`${API_URL}/api/practices/${practiceId}/clients`, {
		headers,
		data: { givenName: 'Rosa', familyName: 'Martinez', email: uniqueEmail('client') }
	});
	const createClientBody = await createClient.text();
	expect(createClient.ok(), `create client failed: ${createClient.status()} ${createClientBody}`).toBe(true);
	const { id: clientId } = JSON.parse(createClientBody);
	const engagementId = seedEngagement(clientId, practiceId);

	await enterPracticeAsEnrolled(context, page, headers, practiceId);
	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);

	await expect(page.getByText('No Doula yet', { exact: true })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Offers' })).toBeVisible();
	await expect(page.getByText(/^There is no one at this practice to offer this work to/)).toBeVisible();
	await expect(page.getByLabel('Jamie Owner')).toHaveCount(0);

	await page.getByRole('button', { name: 'Put me on this Engagement' }).click();

	await expect(page.getByText('You are on this Engagement.')).toBeVisible();
	await expect(page.getByRole('button', { name: 'Put me on this Engagement' })).toHaveCount(0);
	await expect(page.getByText('No Doula yet', { exact: true })).toHaveCount(0);
	// The audit trail: who, in the ledger's own sentence. .first() because
	// DataTable draws the row in both of its trees.
	await expect(page.getByText('Put on this Engagement as the Doula: Jamie Owner').first()).toBeVisible();

	// And it holds on a fresh read: she is the Doula, and there is no control.
	await page.reload();
	await expect(page.getByText('Put on this Engagement as the Doula: Jamie Owner').first()).toBeVisible();
	await expect(page.getByRole('button', { name: 'Put me on this Engagement' })).toHaveCount(0);
	await expect(page.getByText('No Doula yet', { exact: true })).toHaveCount(0);
});
