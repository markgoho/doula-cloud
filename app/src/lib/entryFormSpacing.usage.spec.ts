import { globSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #660's static gate, in the shape `pageTitle.usage.spec.ts` and
 * `formErrors.usage.spec.ts` already use.
 *
 * An archetype-A route builds its own `<form>` inside `EntryPage`'s
 * `content` region, because ADR-0018 leaves region-internal arrangement to
 * the page. `EntryPage` stacks the *top-level* siblings of that region, and
 * `LabeledField` stacks its own label, hint and control -- but `stack-l`'s
 * rule is `> * + *`, which reaches a child and never a grandchild. So a
 * `<form>` that puts its fields in directly gets no spacing between them at
 * all: the Password label sits flush against the Email input, and the
 * submit button flush against the last field. That is what #660 found in a
 * browser at 1440 and 480, on all five archetype-A screens at once.
 *
 * A rendering spec cannot cover this. Three of the six forms are behind a
 * step the page only reaches after a successful sign-in or a second-factor
 * challenge, so a route spec would assert the wrapper on whichever form
 * happened to render first and say nothing about the others. Reading the
 * source says it about all of them, and about the next one somebody adds.
 *
 * The gate is scoped by what a route *composes*, not by a list of paths:
 * importing `EntryPage` is what makes a route archetype A. A `FormPage`
 * route is deliberately out of scope -- there the `<form>` wraps the
 * Template, and the Template already stacks each fieldset's content at the
 * same `var(--space-5)`.
 */

const TEMPLATE = 'EntryPage';
const REQUIRED_WRAPPER = '<stack-l space="var(--space-5)">';

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const entryRoutes = globSync('src/routes/**/+page.svelte', { cwd: appRoot }).filter((file) =>
	readFileSync(new URL(file, `file://${appRoot}`), 'utf8').includes(TEMPLATE)
);

/*
 * The markup that follows each `<form …>` start tag, in source order.
 *
 * The first `>` after `<form` is not reliably the end of the tag: the
 * style-guide's own demo writes `onsubmit={(event) => event.preventDefault()}`,
 * and the arrow's `>` sits inside a Svelte expression. So the expressions
 * are blanked before the scan rather than parsed, which is the smallest
 * rule that reads every form this app has.
 */
const SVELTE_EXPRESSION = /\{[^{}]*\}/g;

function markupAfterEachFormTag(source: string): string[] {
	/*
	 * Blank every Svelte expression to spaces of the same length. Offsets
	 * are unchanged, so a `>` found in here indexes the real source -- and
	 * the arrow's own `>` is no longer one of them.
	 */
	const flattened = source.replaceAll(SVELTE_EXPRESSION, (match) => ' '.repeat(match.length));

	const found: string[] = [];
	let from = flattened.indexOf('<form');
	while (from !== -1) {
		const tagEnd = flattened.indexOf('>', from);
		found.push(source.slice(tagEnd + 1));
		from = flattened.indexOf('<form', tagEnd);
	}

	return found;
}

describe('every archetype-A form stacks its own fields', () => {
	it('finds archetype-A routes to check', () => {
		expect(entryRoutes.length).toBeGreaterThan(0);
	});

	for (const file of entryRoutes) {
		it(`${file} opens each <form> with ${REQUIRED_WRAPPER}`, () => {
			const source = readFileSync(new URL(file, `file://${appRoot}`), 'utf8');
			const forms = markupAfterEachFormTag(source);

			/*
			 * Compared as arrays rather than asserted in the loop, because
			 * several archetype-A routes have no `<form>` at all -- the portal's
			 * own accept-invite is a warning and a button, and `no-practice` is
			 * a sentence and a link. A loop body asserts nothing on those, which
			 * Vitest reports as a test that ran no expectation. An empty array
			 * equal to an empty array is the honest statement: this route has
			 * no form for the rule to be false about.
			 */
			expect(forms.map((markup) => markup.trimStart().startsWith(REQUIRED_WRAPPER))).toEqual(
				forms.map(() => true)
			);
		});
	}
});
