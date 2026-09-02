import { globSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #487's static gate: a route with no `<title>` is invisible in the axe
 * scan's own `document-title` rule until the scan actually runs, which is
 * a slower, deploy-shaped feedback loop. This runs in the unit suite,
 * which `scripts/hooks/pre-commit` runs in full -- the same shape as
 * `tokens.usage.spec.ts` and `formErrors.usage.spec.ts` -- so a new route
 * with no title fails a commit rather than reaching CI.
 *
 * A `+page.svelte` passes if it imports `PageTitle` directly, or imports
 * one of the Templates that already render `PageTitle` internally
 * (#487's ownership decision: every Template calls the shared primitive,
 * every bare route calls it directly). Every style-guide demo page is
 * exempt -- covered once, by the static title on `style-guide/
 * +layout.svelte`, not per component demo.
 */

const TEMPLATES_WITH_PAGE_TITLE = [
	'OverviewHub',
	'RecordDetail',
	'FormPage',
	'QuestionPage',
	'CheckAnswers',
	'EntryPage'
];

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const routeFiles = globSync('src/routes/**/+page.svelte', { cwd: appRoot }).filter(
	(file) => !file.startsWith('src/routes/style-guide/')
);

function hasPageTitle(source: string): boolean {
	if (source.includes('PageTitle')) return true;
	return TEMPLATES_WITH_PAGE_TITLE.some((name) => source.includes(name));
}

describe('every route sets a page title', () => {
	for (const file of routeFiles) {
		it(`${file} calls the shared PageTitle primitive`, () => {
			const source = readFileSync(new URL(file, `file://${appRoot}`), 'utf8');
			expect(hasPageTitle(source)).toBe(true);
		});
	}
});
