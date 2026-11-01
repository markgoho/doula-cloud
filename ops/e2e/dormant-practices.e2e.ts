import { expect, test } from '../../app/e2e/playwrightTest';
import { seedDormantPractice, seedPracticeWithStaff } from '../../app/e2e/stack';
import { signedInOperator } from './operator';

/*
 * The dormant-Practices list read against the local stack: a Practice
 * holding Credits that nobody has used in three years is on it, and one
 * holding none is not. The page fits a 320px screen.
 */

test('An operator reads the dormant-Practices list', async ({ page, request }) => {
	const suffix = crypto.randomUUID().slice(0, 8);
	const dormant = `Quiet Hills Doulas ${suffix}`;
	const empty = `No Credits Doulas ${suffix}`;
	seedDormantPractice(dormant);
	seedPracticeWithStaff(empty, 1);
	await signedInOperator(page, request, 'operator-dormant-password', 'Dormant Operator');

	await page.getByRole('link', { name: 'Practices' }).click();
	await page.getByRole('link', { name: 'Dormant Practices' }).click();

	await expect(page.getByRole('heading', { name: 'Dormant Practices', level: 1 })).toBeVisible();
	const practice = page.getByRole('listitem').filter({ has: page.getByRole('heading', { name: dormant }) });
	await expect(practice.getByText('3 Credits')).toBeVisible();
	await expect(practice.getByText(/^Last contact/)).toBeVisible();
	await expect(page.getByRole('heading', { name: empty })).toHaveCount(0);
	await page.screenshot({ path: test.info().outputPath('dormant-wide.png'), fullPage: true });

	await page.setViewportSize({ width: 320, height: 640 });

	await expect(practice.getByRole('heading', { name: dormant })).toBeVisible();
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
	await page.screenshot({ path: test.info().outputPath('dormant-320.png'), fullPage: true });
});
