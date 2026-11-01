import { expect, test } from '../../app/e2e/playwrightTest';
import { signedInOperator } from './operator';

/*
 * The Operator log against the local stack: the walk's own sign-in is
 * its newest row, under the operator's name.
 */

test('An operator opens the Operator log and sees the sign-in just made', async ({ page, request }) => {
	const name = `Log Operator ${crypto.randomUUID().slice(0, 8)}`;
	await signedInOperator(page, request, 'operator-log-password', name);

	await page.getByRole('link', { name: 'Operator log' }).click();

	await expect(page.getByRole('heading', { name: 'Operator log', level: 1 })).toBeVisible();
	const newest = page.getByRole('listitem').filter({ hasText: name }).first();
	await expect(newest.getByRole('heading', { name: 'Signed in' })).toBeVisible();
});
