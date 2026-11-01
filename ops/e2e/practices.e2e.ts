import { expect, test } from '../../app/e2e/playwrightTest';
import { finalizePracticeDeletion, seedPracticeWithStaff } from '../../app/e2e/stack';
import { signedInOperator } from './operator';

/*
 * The Practice list read against the local stack: a new Practice is at
 * the top with its Owner's contact and its acts, and one whose Deletion
 * finalized is only on the Deleted list.
 */

test('An operator reads the Practice list and the Deleted list', async ({ page, request }) => {
	const suffix = crypto.randomUUID().slice(0, 8);
	const live = `Lakeside Birth ${suffix}`;
	const gone = `Closed Doulas ${suffix}`;
	const goneId = seedPracticeWithStaff(gone, 1);
	finalizePracticeDeletion(goneId);
	const liveId = seedPracticeWithStaff(live, 3);
	await signedInOperator(page, request, 'operator-list-password', 'List Operator');

	await page.getByRole('link', { name: 'Practices' }).click();

	await expect(page.getByRole('heading', { name: 'Practices', level: 1 })).toBeVisible();
	const practice = page.getByRole('listitem').filter({ has: page.getByRole('heading', { name: live }) });
	await expect(practice.getByText(/^Staff 1, staff-.+@doula-cloud\.invalid$/)).toBeVisible();
	await expect(practice.getByText('Not recorded yet')).toBeVisible();
	await expect(practice.getByText('Not given')).toBeVisible();
	await expect(page.getByRole('heading', { name: gone })).toHaveCount(0);
	await expect(practice.getByRole('link', { name: `Give a founding grant to ${live}` })).toHaveAttribute(
		'href',
		`/practices/${liveId}/founding-grant`
	);

	await page.getByRole('link', { name: 'Deleted Practices' }).click();

	await expect(page.getByRole('heading', { name: 'Deleted Practices', level: 1 })).toBeVisible();
	const deleted = page.getByRole('listitem').filter({ has: page.getByRole('heading', { name: gone }) });
	await expect(deleted.getByText(/^Deleted /)).toBeVisible();
	await expect(deleted.getByRole('link')).toHaveCount(0);
	await expect(page.getByRole('heading', { name: live })).toHaveCount(0);
});
