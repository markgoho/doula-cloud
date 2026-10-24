import { expect, test } from '@playwright/test';

/*
 * #1863: the cascade layer order in the production build. A browser fixes
 * the order of cascade layers the first time it meets each name, in
 * stylesheet order (`src/lib/styles/layers.css` says what goes wrong when a
 * component's `@layer components` comes first). Lightning CSS, Vite's CSS
 * minifier, drops the names of the bare `@layer ...;` statement in
 * `app.css` whenever the same file's own blocks already meet them in that
 * order, so the built root stylesheet no longer declares the order at all.
 * `src/app.html` declares it in an inline `<style>` ahead of every
 * stylesheet SvelteKit links, and this spec reads what the built page
 * actually serves: Playwright's web server is `vite build && vite preview`.
 */
const LAYER_ORDER = ['reset', 'tokens', 'base', 'utilities', 'components'];

test('The first stylesheet the built page loads declares the cascade layer order before any layered rule', async ({
	page
}) => {
	await page.goto('/signup');
	await expect(page.getByRole('button', { name: 'Create Practice' })).toBeVisible();

	const firstLayerRule = await page.evaluate(() => {
		for (const sheet of document.styleSheets) {
			for (const rule of sheet.cssRules) {
				if (rule instanceof CSSLayerStatementRule) {
					return { kind: 'statement', names: [...rule.nameList] };
				}
				if (rule instanceof CSSLayerBlockRule) {
					return { kind: 'block', names: [rule.name] };
				}
			}
		}
		return { kind: 'none', names: [] };
	});

	expect(firstLayerRule).toEqual({ kind: 'statement', names: LAYER_ORDER });
});

test('A component rule beats the reset in the built app: a Link icon in a sentence stays inline', async ({
	page
}) => {
	await page.goto('/signup');
	const createPractice = page.getByRole('button', { name: 'Create Practice' });
	await expect(createPractice).toBeVisible();

	// `reset.css`'s `svg { display: block }` loses to `Link`'s inline icon
	// rule only while `components` is the last layer (#1856).
	const iconDisplays = await page
		.locator('p a svg')
		.evaluateAll((icons) => icons.map((icon) => getComputedStyle(icon).display));
	expect(iconDisplays.length).toBeGreaterThan(0);
	expect(iconDisplays.every((display) => display === 'inline')).toBe(true);

	await expect(createPractice).toHaveAccessibleDescription(
		'By creating your Practice, you agree to the Terms of Service (opens in new tab) and the Privacy Policy (opens in new tab).'
	);
});
