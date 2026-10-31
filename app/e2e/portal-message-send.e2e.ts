import { expect, test } from '@playwright/test';
import { seedPortalClient, signInPortalClient } from './portalClient';

// #2210: the Client's Send says what happened and what is next, and says
// it truthfully with no signal. The portal queues no write (ADR-0059,
// position 4), so with the browser offline the send is refused in
// position 5's words, the draft stays in the box, and nothing says sent.
const SENT =
	'Message sent to Riverside Doulas. A reply will show here. Do not rely on messages for anything urgent.';
const UNREACHABLE = 'We could not reach the service. Check your connection and try again.';

test('A Client is told a message was sent, and is told the truth when there is no signal', async ({
	page,
	request,
	context
}) => {
	const { engagementId, clientEmail } = await seedPortalClient(request, 'Riverside Doulas');
	await signInPortalClient(page, request, clientEmail);
	await page.goto(`/portal/engagements/${engagementId}/messages`);

	const box = page.getByLabel('Message', { exact: true });
	const send = page.getByRole('button', { name: 'Send', exact: true });

	await box.fill('Is it normal that my ankles are this swollen?');
	await send.click();
	await expect(page.getByRole('status').filter({ hasText: SENT })).toBeVisible();
	await expect(page.getByText('Is it normal that my ankles are this swollen?')).toBeVisible();
	await expect(box).toHaveValue('');

	await context.setOffline(true);
	try {
		await box.fill('And my hands too.');
		await send.click();
		await expect(page.getByRole('alert').getByText(UNREACHABLE)).toBeVisible();
		await expect(box).toHaveValue('And my hands too.');
		await expect(page.getByRole('status').filter({ hasText: SENT })).toHaveCount(0);
	} finally {
		await context.setOffline(false);
	}

	// Back online, the same draft goes, and only now is it said to be sent.
	await send.click();
	await expect(page.getByRole('status').filter({ hasText: SENT })).toBeVisible();
	await expect(page.getByText('And my hands too.')).toBeVisible();
});
