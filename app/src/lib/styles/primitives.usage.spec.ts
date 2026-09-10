import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { primitiveSpecs } from '#lib/primitives/primitives.js';
import { styleLines } from './styleLines';

/*
 * #1105's source gate (ADR-0039): a layout primitive spaces its children
 * with the container's own geometry -- `gap` -- and not with a margin on
 * the children.
 *
 * ## Why this is a rule rather than a preference
 *
 * A primitive's rules are injected into `@layer utilities`, and every
 * component's scoped `<style>` compiles into `@layer components`, which
 * `app.css` declares after it. So a margin a primitive writes onto a child
 * loses outright to any margin that child writes on itself -- and `margin:
 * 0` on a root element is the most ordinary line in this codebase, sitting
 * in `Notice`, `Text`, `Heading`, `WarningText` and roughly twenty others.
 * Specificity never enters into it. The result is spacing that silently
 * does nothing, wherever a component of that shape is placed, which is
 * exactly what #1105 measured on `/signup` at 1440, 480 and 320.
 *
 * That is why the check reads the mechanism instead of an outcome. A
 * rendering test can only ever see the stacks somebody thought to render;
 * the cancellation is a property of the pair (primitive writes a child
 * margin, component resets its own), and either half can arrive years
 * after the other. `stack.spec.ts` is the rendering half and measures the
 * defect itself; this half is what stops the mechanism coming back.
 *
 * ## What it forbids, exactly
 *
 * A `margin*` declaration inside a rule whose selector reaches for a
 * child -- one containing `>`. Two deliberate narrowings:
 *
 * - `auto` is allowed. Auto margins are not spacing, they are alignment
 *   (`cover-l > h1 { margin-block: auto }` is Cover's centering), and no
 *   `gap` expresses that.
 * - A host rule is untouched. A primitive is free to margin ITSELF; what
 *   it may not do is margin something it does not own.
 *
 * ## Both halves of the source
 *
 * `primitives.css` is the zero-JS default. `primitives.ts` is what an
 * instance with a non-default attribute gets, and it is read by CALLING
 * each spec's own `css()` rather than by scanning the template literals --
 * so the gate judges the rules that actually reach the document, and a
 * thirteenth primitive is covered the day it is added, with nothing to
 * opt in.
 *
 * ## The escape hatch
 *
 * `primitives:ignore` with a reason, scoped to the rule block it
 * introduces, exactly as `tokens:ignore` and `layout:ignore` already work
 * here. `cover-l` is its one user and ADR-0039 records why. The generated
 * half has no line a marker could sit on, so its exception is `EXEMPT`
 * below -- a tag name and the same reason, which is the narrowest form a
 * generated rule can carry.
 */

const IGNORE = 'primitives:ignore';

/*
 * Cover keeps sibling margins deliberately (ADR-0039): its centering is an
 * auto margin, and a gap cannot write `space` at the ends and `auto` in
 * the middle. It carries the same exposure Stack just lost, and #1220 is
 * where that gets its own answer -- not an exemption that quietly grows.
 */
const EXEMPT: ReadonlyMap<string, string> = new Map([
	['cover-l', 'deliberately margined; ADR-0039, follow-up #1220']
]);

const MARGIN_DECLARATION = /(?:^|[\s;{])(margin(?:-[a-z-]+)?)\s*:\s*([^;}]+)/g;

interface Offense {
	readonly where: string;
	readonly selector: string;
	readonly property: string;
	readonly value: string;
}

function isAlignment(value: string): boolean {
	return value
		.trim()
		.split(/\s+/)
		.every((part) => part === 'auto');
}

// One `selector { declarations }` pair. Neither source nests a rule --
// primitives.css declares no at-rule at all, and an injected rule is one
// flat block -- so a pair that holds no brace of its own is the whole
// structure this needs.
const RULE = /([^{}]+)\{([^{}]*)\}/g;

/*
 * Reports every margin declaration sitting under a selector that reaches
 * for a child. Deliberately not a CSS parser: the only structure it needs
 * is which selector a declaration sits under.
 */
function findOffenses(where: string, ruleText: string): Offense[] {
	const offenses: Offense[] = [];
	for (const [, rawSelector, body] of ruleText.matchAll(RULE)) {
		const selector = rawSelector.trim().replaceAll(/\s+/g, ' ');
		if (!selector.includes('>')) continue;
		for (const [, property, value] of body.matchAll(MARGIN_DECLARATION)) {
			if (!isAlignment(value)) offenses.push({ where, selector, property, value: value.trim() });
		}
	}
	return offenses;
}

function report(offenses: readonly Offense[]): string[] {
	return offenses.map(
		(offense) =>
			`${offense.where}: \`${offense.selector}\` sets ${offense.property}: ${offense.value} on a child -- ` +
			`space with \`gap\` on the container instead (ADR-0039), or put ${IGNORE} and a reason on the rule`
	);
}

const stylesheet = fileURLToPath(new URL('primitives.css', import.meta.url));

describe('a layout primitive spaces its children with gap, not with their margins', () => {
	it('writes no child margin in the zero-JS defaults', () => {
		const judged = styleLines(readFileSync(stylesheet, 'utf8'), IGNORE, 'css')
			.map((entry) => entry.text)
			.join('\n');

		expect(report(findOffenses('primitives.css', judged))).toEqual([]);
	});

	it.each(primitiveSpecs.filter((spec) => !EXEMPT.has(spec.tagName)))(
		'writes no child margin in the rules $tagName injects for a configured instance',
		(spec) => {
			const rules = spec.css(spec.defaults, spec.tagName);

			expect(report(findOffenses(`primitives.ts (${spec.tagName})`, rules))).toEqual([]);
		}
	);

	/*
	 * An exemption that names a primitive no longer in the set is an
	 * exemption nobody is reading, and the next person inherits it as
	 * fact. It goes when its subject does.
	 */
	it('holds no exemption for a primitive that no longer exists', () => {
		const tagNames = new Set(primitiveSpecs.map((spec) => spec.tagName));

		expect(EXEMPT.keys().filter((tagName) => !tagNames.has(tagName)).toArray()).toEqual([]);
	});
});
