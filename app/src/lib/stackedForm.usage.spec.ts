import { globSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #1108's static gate, the authenticated-side counterpart to
 * `entryFormSpacing.usage.spec.ts`.
 *
 * ## What is still true after #1105
 *
 * ADR-0039 moved `stack-l`'s spacing from `> * + *` sibling margins onto
 * the container as `gap`. That fixed a *different* defect -- a child that
 * reset its own margin canceled the space it was being given -- and it
 * changes nothing here. A gap is placed between a flex container's own
 * children, and a `<form>` is one child. Its fields are grandchildren, and
 * grandchildren were never spaced under either mechanism. So #660's
 * finding stands as written: a `<form>` holding `LabeledField`s directly
 * renders them flush, whichever way the stack above it spaces things.
 *
 * ## Which files it asks about, and how they are discovered
 *
 * Every `.svelte` file under `src`, less two kinds:
 *
 * 1. `StackedForm` itself, which is where the one allowed `<form>` lives
 * 2. the unauthenticated entry screens, which `entryFormSpacing.usage.spec.ts`
 *    already holds to the stricter rule that they carry no `<form>` at all
 *
 * No list of paths, so a new screen -- a route, an organism, a molecule --
 * is asked the question without opting in.
 *
 * ## What it asks
 *
 * That a `<form>` element is either `StackedForm` (in which case the file
 * has no `<form>` of its own) or carries `stacked-form:ignore: <reason>`
 * beside it, in the marker style `tokens:ignore`, `layout:ignore` and
 * `primitives:ignore` already use here. The escape hatch is the point
 * rather than a leak: a form inside a `DataTable` row, a `method="get"`
 * filter, and a page-wide `<form>` wrapping a Template that stacks its own
 * fieldsets all want something other than a run of fields at
 * `var(--space-5)`, and #1108's complaint was never that they are wrong --
 * it was that nothing on the page said which they were.
 *
 * The marker is read from the `<form` line itself or from the three
 * source lines above it, which is where an HTML comment explaining a form
 * naturally sits.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const SCRIPT_OR_STYLE = /<(script|style)\b[\S\s]*?<\/\1>/g;
const ENTRY_TEMPLATE_IMPORT = /from '#lib\/components\/templates\/EntryPage\.svelte'/;
const IGNORE_MARKER = /stacked-form:ignore:\s*\S/;

const STACKED_FORM = 'src/lib/components/molecules/StackedForm.svelte';

function read(file: string): string {
	return readFileSync(new URL(file, `file://${appRoot}`), 'utf8');
}

/*
 * `<script>` and `<style>` are stripped rather than HTML comments, because
 * on this side of the app the prose about a form is mostly a block comment
 * at the top of the module -- `FormPage`, `IntakeQuestion`, `IntakeActions`
 * and `ReauthPrompt` all explain in `<script>` why their `<form>` is where
 * it is. The HTML comments are kept, because that is where the marker
 * lives.
 */
function markup(source: string): string {
	return source.replaceAll(SCRIPT_OR_STYLE, '');
}

function isEntryScreen(file: string): boolean {
	if (file.startsWith('src/routes/(signed-out)/')) return true;
	if (file.startsWith('src/routes/portal/(signed-out)/')) return true;
	return ENTRY_TEMPLATE_IMPORT.test(read(file));
}

function unmarkedForms(source: string): number[] {
	const lines = markup(source).split('\n');
	const unmarked: number[] = [];

	for (const [index, line] of lines.entries()) {
		if (!line.includes('<form')) continue;
		const nearby = lines.slice(Math.max(0, index - 3), index + 1);
		if (nearby.some((candidate) => IGNORE_MARKER.test(candidate))) continue;
		unmarked.push(index + 1);
	}

	return unmarked;
}

const authenticatedFiles = globSync('src/**/*.svelte', { cwd: appRoot }).filter(
	(file) => file !== STACKED_FORM && !isEntryScreen(file)
);

describe('a form outside the entry screens is StackedForm, or says why it is not', () => {
	it('finds the files to check', () => {
		expect(authenticatedFiles.length).toBeGreaterThan(0);
	});

	for (const file of authenticatedFiles) {
		it(`${file} has no unexplained bare form`, () => {
			expect(unmarkedForms(read(file))).toEqual([]);
		});
	}
});
