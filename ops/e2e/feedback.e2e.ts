import { expect, test } from '../../app/e2e/playwrightTest';
import { countFeedbackReads, seedDeadLetteredClientFeedback, seedPracticeWithStaff } from '../../app/e2e/stack';
import { signedInOperator } from './operator';

/*
 * Feedback read against the local stack: a piece whose issue
 * dead-lettered is on the list under Issue not opened, and opening it
 * shows the sender's name and email and records the read.
 */

test('An operator finds a dead-lettered piece of Feedback and opens it', async ({ page, request }) => {
	const suffix = crypto.randomUUID().slice(0, 8);
	const practiceId = seedPracticeWithStaff(`Riverside Doulas ${suffix}`, 1);
	const text = `The invoice total does not match the contract ${suffix}.`;
	const feedbackId = seedDeadLetteredClientFeedback(practiceId, `Nadia Haddad ${suffix}`, text);
	await signedInOperator(page, request, 'operator-feedback-password', 'Feedback Operator');

	await page.getByRole('link', { name: 'Feedback' }).click();

	await expect(page.getByRole('heading', { name: 'Feedback', level: 1 })).toBeVisible();
	const row = page.getByRole('listitem').filter({ has: page.locator(`a[href="/feedback/${feedbackId}"]`) }).first();
	await expect(row.getByText('Not opened: all 8 attempts failed, and nothing will try again')).toBeVisible();
	await expect(row.getByText('401 Bad credentials')).toBeVisible();
	expect(countFeedbackReads(feedbackId)).toBe(0);

	await row.locator(`a[href="/feedback/${feedbackId}"]`).click();

	await expect(page.getByRole('heading', { name: /^Feedback sent/, level: 1 })).toBeVisible();
	await expect(page.getByText(text)).toBeVisible();
	await expect(page.getByText(`Nadia Haddad ${suffix}, a Client`)).toBeVisible();
	await expect(page.getByText(/^portal_.+@doula-cloud\.invalid$/)).toBeVisible();
	await expect(page.getByText(`Riverside Doulas ${suffix}`)).toBeVisible();
	expect(countFeedbackReads(feedbackId)).toBe(1);

	await page.getByRole('link', { name: 'Back to Feedback' }).click();
	await expect(page.getByRole('heading', { name: 'Feedback', level: 1 })).toBeVisible();
});
