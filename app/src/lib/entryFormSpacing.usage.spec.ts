import { globSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #660's static gate, in the shape `pageTitle.usage.spec.ts` already uses.
 *
 * `stack-l`'s rule is `> * + *`: it spaces a child and never a grandchild.
 * A Template stacks the top-level siblings of the region it hands a route,
 * and `LabeledField` stacks its own label, hint and control -- but nothing
 * between those two levels stacks a form's fields. So a `<form>` that held
 * `LabeledField`s directly rendered them flush, on every unauthenticated
 * entry screen in the app at once, which is what #660 found in a browser at
 * 1440 and 480.
 *
 * `StackedForm` is the fix, and this is the check that nobody writes the
 * unstacked shape again. A rendering spec could not do it: several of these
 * forms sit behind a step the page only reaches after a successful sign-in
 * or a second-factor challenge, so a rendering spec would assert on
 * whichever form happened to render first and say nothing about the others.
 *
 * ## Which routes it asks about, and how they are discovered
 *
 * Three rules, none of them a list of paths, so a new screen is checked
 * without opting in:
 *
 * 1. anything under `(signed-out)` -- the Staff side's own route group
 * 2. anything under `portal/(signed-out)` -- the Client portal's
 * 3. anything importing `EntryPage`, which is what makes a route archetype
 *    A wherever it lives (`mfa/enroll` and the style guide's own demo are
 *    both outside the two groups)
 *
 * ## What it asks
 *
 * That the route contains no bare `<form>` element -- `StackedForm` owns
 * that element now -- unless the form opens with the stack wrapper itself.
 * The exception is deliberate and there is exactly one user of it today:
 * see `EXEMPT` below.
 */

const REQUIRED_WRAPPER = '<stack-l space="var(--space-5)">';

/*
 * The pre-account Offer screen. `StackedForm` sets `novalidate`, because
 * ADR-0021's Recover from validation errors pattern is that the page
 * refuses the submit and says so once at the top -- and this screen has no
 * `ErrorSummary` and no refusal path to say it with, so it is still relying
 * on the browser's own bubble to stop an empty access code. Adopting the
 * molecule there would take that refusal away and put nothing in its place.
 * It carries the wrapper inline instead, which is what this file lets it
 * do, until #1107 gives it an error summary of its own.
 */
const EXEMPT = ['src/routes/(signed-out)/offers/[offerId]/+page.svelte'];

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const HTML_COMMENT = /<!--[\S\s]*?-->/g;
const ENTRY_TEMPLATE_IMPORT = /from '#lib\/components\/templates\/EntryPage\.svelte'/;

function read(file: string): string {
	return readFileSync(new URL(file, `file://${appRoot}`), 'utf8');
}

// A comment that mentions `<form>` is prose about a form, not a form --
// `account/+page.svelte` has three of them.
function withoutComments(source: string): string {
	return source.replaceAll(HTML_COMMENT, '');
}

function isEntryScreen(file: string): boolean {
	if (file.startsWith('src/routes/(signed-out)/')) return true;
	if (file.startsWith('src/routes/portal/(signed-out)/')) return true;
	return ENTRY_TEMPLATE_IMPORT.test(read(file));
}

const entryRoutes = globSync('src/routes/**/+page.svelte', { cwd: appRoot }).filter((file) =>
	isEntryScreen(file)
);

describe('every unauthenticated entry form stacks its own fields', () => {
	it('finds the entry screens to check', () => {
		expect(entryRoutes.length).toBeGreaterThan(0);
	});

	it('names only exemptions that still exist', () => {
		expect(EXEMPT.filter((file) => entryRoutes.includes(file))).toEqual(EXEMPT);
	});

	for (const file of entryRoutes) {
		it(`${file} builds its forms with StackedForm`, () => {
			const markup = withoutComments(read(file));

			if (EXEMPT.includes(file)) {
				expect(markup.includes(REQUIRED_WRAPPER)).toBe(true);
				return;
			}

			expect(markup.includes('<form')).toBe(false);
		});
	}
});
