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
 * rather than a leak: a `<form>` wrapping a Template that stacks its own
 * fieldsets, a `method="get"` filter laying its controls out on two axes,
 * a form holding one button, and a form whose only refusal today is the
 * browser's own `required` (#1228, where `StackedForm`'s `novalidate`
 * would take that away) each want something other than a plain run of
 * fields. #1108's complaint was never that they are wrong -- it was that
 * nothing on the page said which they were.
 *
 * The marker is read from the `<form` line itself or from the nearest
 * line above it that holds anything, which is where the HTML comment
 * explaining a form sits. A comment that runs several lines counts as its
 * last one, so the prose can be as long as the reason needs.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

const SCRIPT_OR_STYLE = /<(script|style)\b[\S\s]*?<\/\1>/g;
const HTML_COMMENT = /<!--[\S\s]*?-->/g;
const ENTRY_TEMPLATE_IMPORT = /from '#lib\/components\/templates\/EntryPage\.svelte'/;
const IGNORE_MARKER = /stacked-form:ignore:\s*\S/;

const STACKED_FORM = 'src/lib/components/molecules/StackedForm.svelte';

function read(file: string): string {
	return readFileSync(new URL(file, `file://${appRoot}`), 'utf8');
}

/*
 * Prose that says `<form>` is prose about a form, not a form, and on this
 * side of the app there is a lot of it: `FormPage`, `IntakeQuestion`,
 * `IntakeActions` and `ReauthPrompt` all explain in a `<script>` block
 * comment why their form is where it is, and `account/+page.svelte` has
 * three HTML comments that mention one. So `<script>` and `<style>` go
 * whole, and an HTML comment goes unless it carries the marker -- which is
 * exactly where the marker is meant to live, in a comment beside the form
 * it explains. Line count is preserved on the way out, because the marker
 * is read from the line just above the form and a blanked comment has to
 * leave that line behind rather than closing the gap.
 */
function markup(source: string): string {
	return source
		.replaceAll(SCRIPT_OR_STYLE, (block) => blankOut(block))
		.replaceAll(HTML_COMMENT, (comment) =>
			IGNORE_MARKER.test(comment) ? `${blankOut(comment)}stacked-form:ignore: x` : blankOut(comment)
		);
}

/*
 * A comment collapses to its own last line rather than vanishing, so a
 * marker written across several lines is still the line immediately above
 * the form it explains.
 */
function blankOut(block: string): string {
	return '\n'.repeat((block.match(/\n/g) ?? []).length);
}

/*
 * The sibling gate's own three rules, narrowed to what it actually globs.
 * `entryFormSpacing.usage.spec.ts` reads `src/routes/**\/+page.svelte` and
 * nothing else, so a component that happens to live under one of the
 * signed-out route groups is asked by neither gate unless this one asks
 * it. Handing a file to the stricter check is only honest where the
 * stricter check would in fact see it.
 */
function isEntryScreen(file: string): boolean {
	if (!file.endsWith('/+page.svelte')) return false;
	if (file.startsWith('src/routes/(signed-out)/')) return true;
	if (file.startsWith('src/routes/portal/(signed-out)/')) return true;
	return ENTRY_TEMPLATE_IMPORT.test(read(file));
}

/*
 * The nearest line above that holds anything, rather than a fixed window.
 * A window of one is too narrow -- `markup` blanks a comment that says
 * nothing about stacking, and `MessageThread` writes an
 * `eslint-disable-next-line` comment inside a form, so a form could
 * easily have one above it -- and a window of three is too wide, because
 * a marker written for one form would then exempt an unrelated `<form>`
 * two lines below it. "Nothing but blank lines in between" is the rule
 * that means beside.
 */
function precedingLine(lines: string[], index: number): string {
	for (let above = index - 1; above >= 0; above--) {
		if (lines[above]!.trim() !== '') return lines[above]!;
	}
	return '';
}

function unmarkedForms(source: string): number[] {
	const lines = markup(source).split('\n');
	const unmarked: number[] = [];

	for (const [index, line] of lines.entries()) {
		if (!line.includes('<form')) continue;
		if (IGNORE_MARKER.test(line) || IGNORE_MARKER.test(precedingLine(lines, index))) continue;
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
