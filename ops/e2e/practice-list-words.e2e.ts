import { expect, test, type Page } from '../../app/e2e/playwrightTest';
import { seedPracticeWithStaff } from '../../app/e2e/stack';
import { signedInOperator } from './operator';

/*
 * No cell of the Practices table breaks a word across lines (#2615): a run
 * of letters and digits whose rendered text lands on more than one line has
 * been split mid-word. A hyphen or an "@" is a place to break, so an email
 * address wrapping at one is not a split word.
 */

async function wordsBrokenMidWord(page: Page): Promise<string[]> {
	return page.evaluate(() => {
		const broken: string[] = [];
		const roots = document.querySelectorAll('main td, main th, main dt, main dd, main .record-title');
		for (const root of roots) {
			if (!(root instanceof HTMLElement) || root.getClientRects().length === 0) continue;
			const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
			for (let node = walker.nextNode(); node; node = walker.nextNode()) {
				const text = node.textContent ?? '';
				for (const match of text.matchAll(/[\p{L}\p{N}]+/gu)) {
					const range = document.createRange();
					range.setStart(node, match.index);
					range.setEnd(node, match.index + match[0].length);
					const lines = new Set([...range.getClientRects()].map((rect) => Math.round(rect.top)));
					if (lines.size > 1) broken.push(match[0]);
				}
			}
		}
		return broken;
	});
}

for (const colorScheme of ['light', 'dark'] as const) {
	for (const width of [320, 900, 1300, 1360, 1440]) {
		test(`The Practices table breaks no word at ${width}px in ${colorScheme}`, async ({ page, request }) => {
			await page.emulateMedia({ colorScheme });
			await page.setViewportSize({ width, height: 900 });
			const suffix = crypto.randomUUID().slice(0, 8);
			seedPracticeWithStaff(`Lakeside Birth Collective ${suffix}`, 3);
			seedPracticeWithStaff(`Rooted ${suffix}`, 1);
			await signedInOperator(page, request, 'operator-words-password', 'Words Operator');
			await page.getByRole('navigation', { name: 'Operations' }).getByRole('link', { name: 'Practices' }).click();
			await expect(page.getByRole('heading', { name: 'Practices', level: 1 })).toBeVisible();
			await expect(page.locator('a:visible', { hasText: 'Refund Credits' }).first()).toBeVisible();
			if (process.env.PRACTICE_WALK_DIR) {
				await page.screenshot({
					path: `${process.env.PRACTICE_WALK_DIR}/practices-${width}-${colorScheme}.png`,
					fullPage: true
				});
			}

			expect(await wordsBrokenMidWord(page)).toEqual([]);
			expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
		});
	}
}
