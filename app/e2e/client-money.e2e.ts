import { expect, test } from '@playwright/test';
import { seedInvoice } from './stack';
import { seedPortalClient, signInPortalClient } from './portalClient';

// #1564: a Client walks from the hub's one money line to her Invoices,
// opens one, and comes back. The Invoices are seeded straight into the
// database: the e2e stack has no Stripe, so nothing can raise one through
// the API. What is under test is the whole read path -- the BFF's
// Client-gated routes, RLS, and the screens -- not the words, which the
// component specs own.

test('a Client walks from the hub to an Invoice and back', async ({ page, request }) => {
	const { practiceId, engagementId, clientEmail } = await seedPortalClient(request, 'Riverside Doulas');
	seedInvoice(practiceId, engagementId, { reference: 'A4B2-0011', stripeInvoiceId: 'in_e2e_open' });
	seedInvoice(practiceId, engagementId, { status: 'paid', amountCents: 90_000, reference: 'INV-0002' });
	// A draft never reaches her, whatever the Practice has raised.
	seedInvoice(practiceId, engagementId, { status: 'draft', amountCents: 1, reference: 'DRAFT-0001' });

	await signInPortalClient(page, request, clientEmail);
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));

	// The hub's one line: the figure, and a way through.
	const main = page.getByRole('main');
	await expect(main.getByText('Total to pay $3,350.00')).toBeVisible();
	await main.getByRole('link', { name: 'Invoices' }).click();

	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}/invoices$`));
	await expect(page.getByRole('heading', { level: 1, name: 'Invoices' })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'What you still owe' })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'What you have paid' })).toBeVisible();
	await expect(main.getByText('Total to pay').first()).toBeVisible();
	await expect(page.getByRole('link', { name: 'DRAFT-0001' })).toHaveCount(0);

	await page.getByRole('link', { name: 'A4B2-0011' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}/invoices/[0-9a-f-]+$`));
	await expect(page.getByRole('heading', { level: 1, name: 'Invoice A4B2-0011' })).toBeVisible();
	await expect(page.getByText('Not yet paid')).toBeVisible();

	await page.getByRole('link', { name: 'Back to Invoices' }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}/invoices$`));

	await page.getByRole('link', { name: 'Back', exact: true }).click();
	await expect(page).toHaveURL(new RegExp(`/portal/engagements/${engagementId}$`));
});
