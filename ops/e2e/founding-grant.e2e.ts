import { expect, test } from '../../app/e2e/playwrightTest';
import { seedFoundingGrant, seedPracticeWithStaff } from '../../app/e2e/stack';
import { signedInOperator } from './operator';

/*
 * A founding grant walked through its check page against the local
 * stack: the amount is sized from the Practice's four Staff, never typed,
 * and a second visit finds the grant already given.
 */

test('An operator gives a founding grant sized from the Practice’s Staff', async ({ page, request }) => {
	const practiceId = seedPracticeWithStaff('Rooted Birth Collective', 4);
	await signedInOperator(page, request, 'operator-grant-password', 'Grant Operator');

	await page.goto(`/practices/${practiceId}/founding-grant`);
	await expect(page.getByRole('heading', { name: 'Check the founding grant before you give it' })).toBeVisible();
	const answers = page.getByRole('definition');
	await expect(answers.filter({ hasText: 'Rooted Birth Collective' })).toBeVisible();
	await expect(answers.filter({ hasText: '12 Credits, 3 for each Staff member' })).toBeVisible();
	await expect(page.getByRole('textbox')).toHaveCount(0);

	await page.getByRole('button', { name: 'Give 12 Credits' }).click();

	await expect(page.getByRole('heading', { name: 'Founding grant given' })).toBeVisible();
	await expect(page.getByRole('status')).toContainText('12 Credits given to Rooted Birth Collective, 3 for each of the 4 Staff.');

	await page.reload();
	await expect(page.getByText('This Practice already holds its founding grant of 12 Credits. A founding grant is given once.')).toBeVisible();
	await expect(page.getByRole('button', { name: /Give/ })).toHaveCount(0);
});

test('A Practice the demo seed gave a grant to shows it held, with nothing to give', async ({ page, request }) => {
	const practiceId = seedPracticeWithStaff('Seeded Grant Collective', 2);
	seedFoundingGrant(practiceId, 'Seed Operator');
	await signedInOperator(page, request, 'operator-grant-password', 'Grant Operator');

	await page.goto(`/practices/${practiceId}/founding-grant`);

	await expect(page.getByText('This Practice already holds its founding grant of 6 Credits. A founding grant is given once.')).toBeVisible();
	await expect(page.getByRole('button', { name: /Give/ })).toHaveCount(0);
});
