import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';

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

/*
 * `JourneyQuestion` is not a Template. It is the Client journeys' own
 * shared question page (#466, #1610): each shared question in
 * `clients/questions/` composes it, and a Client's details journey
 * mounts them. It composes
 * `QuestionPage`, so it renders the title exactly as a Template does --
 * one or two levels further out than this scan can read. Named here
 * rather than dropped from the glob, so the gate still asks the question
 * of every route and the answer is that something they compose calls the
 * primitive.
 */
const TEMPLATES_WITH_PAGE_TITLE = [
	'JourneyQuestion',
	'DateOfBirthQuestion',
	'EmailQuestion',
	'PhoneQuestion',
	'AddressQuestion',
	'SectionQuestion',
	'OverviewHub',
	'RecordDetail',
	'FormPage',
	'QuestionPage',
	'CheckAnswers',
	'EntryPage',
	'ListPage',
	'DocumentPage'
];

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const routeFiles = globFiles('src/routes/**/+page.svelte', { cwd: appRoot }).filter(
	(file) => !file.startsWith('src/routes/style-guide/')
);

function hasPageTitle(source: string): boolean {
	if (source.includes('PageTitle')) return true;
	return TEMPLATES_WITH_PAGE_TITLE.some((name) => source.includes(name));
}

/*
 * #1705's static gate. Every Template titles its page "Error: " from
 * `Boolean(errorSummary)`, and GOV.UK keeps that prefix for a page that
 * was really refused. A snippet declared as a child of the Template is
 * always passed and always truthy, so the page read "Error: " on a
 * first visit however its body was guarded. The summary is declared at
 * the top level of the markup, beside the Template, and passed as
 * `errorSummary={<the refusals>.length > 0 ? errorSummary : undefined}`.
 *
 * Read from the source text, so it leans on the formatter: a snippet
 * declared as a component's child is indented, and one declared beside
 * it is not. Behavioral specs cover the titles themselves.
 */
const svelteFiles = globFiles('src/**/*.svelte', { cwd: appRoot });

const CHILD_SUMMARY = /^[\t ]+\{#snippet errorSummary\(\)\}/m;
const UNCONDITIONAL_SUMMARY = /errorSummary=\{(?![^}]*\?\s*errorSummary\s*:\s*undefined\})/;

describe('an error summary is passed only while there is a refusal', () => {
	for (const file of svelteFiles) {
		const source = readFileSync(new URL(file, `file://${appRoot}`), 'utf8');
		if (!source.includes('{#snippet errorSummary()}')) continue;
		it(`${file} declares its errorSummary beside the Template and passes it conditionally`, () => {
			expect(source).not.toMatch(CHILD_SUMMARY);
			expect(source).toContain('errorSummary={');
			expect(source).not.toMatch(UNCONDITIONAL_SUMMARY);
		});
	}
});

describe('every route sets a page title', () => {
	for (const file of routeFiles) {
		it(`${file} calls the shared PageTitle primitive`, () => {
			const source = readFileSync(new URL(file, `file://${appRoot}`), 'utf8');
			expect(hasPageTitle(source)).toBe(true);
		});
	}
});
