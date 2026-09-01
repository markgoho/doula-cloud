/*
 * The floor check's own instrument (#564), pulled apart from
 * `floor.svelte.spec.ts` the same way `continuum.ts` is pulled apart from
 * `continuum.svelte.spec.ts`: discovery and the DOM-touching helpers here,
 * the `describe`/`it` blocks and the criteria registry there. CONTEXT.md
 * defines a content floor as a fixed point discovered from the content and
 * never chosen from a set; this module is what turns that sentence into an
 * assertion.
 *
 * Discovery reads the RAW SOURCE TEXT of every component (a Vite `?raw`
 * glob, not `import.meta.glob`'s default lazy-module form), so nothing
 * here imports or executes a component -- which is what keeps this file
 * safe to import from a plain `.spec.ts` without repeating #527's coverage
 * regression (see `continuum.ts`'s own note on that). `styleLines`
 * (`layout.usage.spec.ts`'s own tool) does the actual scanning, so a
 * `@container` mentioned in a markup comment -- DataTable.svelte carries
 * exactly one -- is never mistaken for a rule.
 */
import { CONFORMANCE_COMMITMENT } from './continuum.js';
import { styleLines } from '#lib/styles/styleLines.js';

// No `layout:ignore`-style marker applies here, so this sentinel -- which
// nothing can contain -- makes `styleLines` return every style line
// unfiltered, the same trick `layout.usage.spec.ts` uses for its own
// unconditional rules.
const NO_MARKER = String.fromCodePoint(0);

const CONTAINER_MIN_WIDTH = /@container\s*([\w-]+)?\s*\(min-width:\s*([\d.]+)rem\)/;

export interface Condition {
	/**
	Path as the glob keys it, e.g. `../../lib/components/organisms/DataTable.svelte`.
	*/
	readonly file: string;
	/**
	1-based occurrence of a `@container (min-width: …)` rule within `file`.
	*/
	readonly index: number;
	/**
	`${file}#${index}` -- stable across edits elsewhere in the file, never a width.
	*/
	readonly key: string;
	/**
	1-based source line, for a failure message only -- never part of `key`.
	*/
	readonly line: number;
	readonly containerName: string | undefined;
	readonly floorRem: number;
	readonly raw: string;
	/**
	 * The first class selector inside the rule's own block, e.g. `.body`
	 * for `QuestionPage`'s condition, `.rail` for `StepRail`'s. Two
	 * conditions can share both a literal AND a rendered frame -- `StepRail`
	 * renders inside `QuestionPage`, and both measure to 67.5rem (#564) --
	 * so `forceLive` needs more than the condition text and a scope class to
	 * tell them apart.
	 */
	readonly hint: string | undefined;
}

const CLASS_SELECTOR = /\.[a-zA-Z][\w-]*/;

/*
 * The first class selector in the lines that follow `from`, inside the
 * condition's own block -- good enough to disambiguate, since it is only
 * ever compared against a compiled rule already matched on the condition
 * text and a scope class.
 */
function findHint(lines: ReturnType<typeof styleLines>, from: number): string | undefined {
	for (const { text } of lines.slice(from)) {
		const selector = CLASS_SELECTOR.exec(text);
		if (selector) return selector[0];
	}
	return undefined;
}

// Any `@container` rule at all, matched before the stricter shape below --
// this is what lets `findMalformedConditions` see a rule the strict regex
// missed, rather than the rule silently not existing as far as discovery
// is concerned.
const ANY_CONTAINER = /@container\b/;

export interface Malformed {
	readonly file: string;
	readonly line: number;
	readonly raw: string;
}

interface Scanned {
	readonly conditions: Condition[];
	readonly malformed: Malformed[];
}

function scanFile(file: string, source: string): Scanned {
	const lines = styleLines(source, NO_MARKER, 'svelte');
	const conditions: Condition[] = [];
	const malformed: Malformed[] = [];
	let index = 0;
	for (const [lineIndex, { line, text }] of lines.entries()) {
		if (!ANY_CONTAINER.test(text)) continue;
		const match = CONTAINER_MIN_WIDTH.exec(text);
		if (!match) {
			malformed.push({ file, line, raw: text.trim() });
			continue;
		}
		index += 1;
		conditions.push({
			file,
			index,
			key: `${file}#${index}`,
			line,
			containerName: match[1],
			floorRem: Number(match[2]),
			raw: text.trim(),
			hint: findHint(lines, lineIndex + 1)
		});
	}
	return { conditions, malformed };
}

function sortedFiles(sources: Record<string, string>): string[] {
	return Object.keys(sources).toSorted((a, b) => a.localeCompare(b));
}

/*
 * Every `@container (min-width: …rem)` rule in the given sources, in file
 * order. Takes raw source text rather than a glob pattern so it can be
 * unit-tested against a literal string (`floor.spec.ts`) without touching
 * the filesystem or Vite at all.
 */
export function findConditions(sources: Record<string, string>): Condition[] {
	return sortedFiles(sources).flatMap((file) => scanFile(file, sources[file]).conditions);
}

/*
 * Every `@container` rule that is NOT `(min-width: …rem)` -- a `px`
 * literal, `min-inline-size`, a `max-width`, a second condition joined
 * with `and`. `findConditions`'s own regex simply does not match these,
 * which would otherwise make a condition written in any other shape
 * invisible to discovery rather than caught by it -- the exact silent
 * exemption ADR-0025's fixture rule warns about, moved from "a component
 * with no fixture" to "a condition with no shape this check recognizes."
 * Decision #3 (#564) is that a floor stays a `rem` literal, so this list
 * failing empty is that decision enforced, not merely documented.
 */
export function findMalformedConditions(sources: Record<string, string>): Malformed[] {
	return sortedFiles(sources).flatMap((file) => scanFile(file, sources[file]).malformed);
}

export function remToPx(rem: number): number {
	return rem * 16;
}

/*
 * Every discovered `(min-width: …rem)` condition whose literal resolves
 * BELOW `CONFORMANCE_COMMITMENT` (320px, ADR-0024) -- a floor that fires
 * across the whole continuum this repo verifies has no narrow branch left
 * to switch FROM, which is exactly CONTEXT.md's failure sentence: "one
 * configuration at every available space and no content floor to switch
 * on." A number this low is not evidence of a narrow floor; it is
 * evidence the criterion used to measure it was wrong (#564's own
 * CheckAnswers row, first measured under the overflow criterion, which
 * cannot see this component's true constraint at all). Kept separate
 * from `findMalformedConditions`, whose own conditions never reached a
 * number to judge in the first place -- this one did, and the number is
 * itself the defect.
 */
export function findConditionsBelowConformance(sources: Record<string, string>): Malformed[] {
	return findConditions(sources)
		.filter((condition) => remToPx(condition.floorRem) < CONFORMANCE_COMMITMENT)
		.map((condition) => ({
			file: condition.file,
			line: condition.line,
			raw: `${condition.raw} -- resolves to ${remToPx(condition.floorRem)}px, below the ${CONFORMANCE_COMMITMENT}px conformance commitment`
		}));
}

/*
 * Forces a discovered `@container` rule permanently live, so its wide
 * configuration can be inspected at a width the rule itself would not
 * have chosen. `conditionText` has no setter in this engine (verified
 * against Chromium: `CSSConditionRule` throws `... has only a getter`),
 * and rewriting the existing rule in place through `deleteRule` /
 * `insertRule` was tried and abandoned: Chromium accepts the mutation --
 * `conditionText` reads back `(min-width: 0px)` -- but does not
 * re-evaluate the container against elements already on the page, so an
 * UNNAMED condition (`QuestionPage`, `CheckAnswers`'s rail query,
 * `OverviewHub`, `RecordDetail`, `StepRail`) silently keeps its old,
 * un-forced behaviour while a NAMED one (`data-table`, `staff-top-bar`,
 * `portal-top-bar`) happened to work, for reasons that were not chased
 * further once a reliable alternative was found.
 *
 * The alternative: APPEND a new rule with the same declarations rather
 * than rewrite the old one. `(min-width: 0px)` always matches, so the new
 * rule's declarations apply at every width regardless of what the
 * original condition decides, and re-opening `@layer components` in a
 * second stylesheet adds to the SAME layer rather than a new one -- CSS
 * layers are cumulative by name, so this still cascades exactly where the
 * original rule would have. The original is left untouched, and the
 * appended stylesheet is never removed either -- it outlives the test
 * that forced it, into any later test in the same file that happens to
 * mount the same component again. Harmless in practice: a forced rule
 * only ever repeats the SAME declarations the real one would already
 * apply once its own condition is met, so a stale forced copy changes
 * nothing about what a later test measures.
 *
 * `scopes` are candidate Svelte scope classes (`svelte-xxxxxx`), most
 * likely first, read off the rendered demo (`scopeClasses`). They
 * disambiguate two conditions that happen to share a literal --
 * `QuestionPage` and `CheckAnswers` both measure to 67.5rem (#564) --
 * because every style-guide demo's compiled CSS is present in the
 * document at once (the eager glob `floor.svelte.spec.ts` shares with
 * `continuum.svelte.spec.ts`), so a plain text match on the condition
 * alone can silently grab the wrong component's rule. Each candidate is
 * tried in turn rather than trusting the first: a demo page's own chrome
 * (`check-answers/+page.svelte`'s wide-column toggle) or a repeated child
 * component can out-count the Template actually under test, so "most
 * common under the frame" is a good first guess and not a proof.
 */
function isRuleMatch(rule: CSSRule, condition: Condition, scope: string): rule is CSSContainerRule {
	return (
		rule instanceof CSSContainerRule &&
		rule.cssText.includes(condition.raw) &&
		rule.cssText.includes(scope) &&
		(!condition.hint || rule.cssText.includes(condition.hint))
	);
}

function findInRules(
	rules: CSSRuleList,
	condition: Condition,
	scope: string
): CSSContainerRule | undefined {
	for (const rule of rules) {
		if (isRuleMatch(rule, condition, scope)) return rule;
		if ('cssRules' in rule) {
			const nested = findInRules((rule as CSSGroupingRule).cssRules, condition, scope);
			if (nested) return nested;
		}
	}
	return undefined;
}

function findInDocument(condition: Condition, scope: string): CSSContainerRule | undefined {
	for (const sheet of document.styleSheets) {
		let rules: CSSRuleList;
		try {
			rules = sheet.cssRules;
		} catch {
			continue;
		}
		const found = findInRules(rules, condition, scope);
		if (found) return found;
	}
	return undefined;
}

export function forceLive(condition: Condition, scopes: readonly string[]): void {
	for (const scope of scopes) {
		const rule = findInDocument(condition, scope);
		if (!rule) continue;
		const forced = rule.cssText.replace(/\(min-width:\s*[^)]+\)/, '(min-width: 0px)');
		const style = document.createElement('style');
		style.textContent = `@layer components {\n${forced}\n}`;
		document.head.append(style);
		return;
	}
	throw new Error(
		`could not find the @container rule for ${condition.key} ("${condition.raw}") under any of [${scopes.join(', ')}]`
	);
}

/*
 * The Svelte scope class Vite compiles onto every element a component's
 * own `<style>` targets -- `svelte-xxxxxx`. `forceLive` needs it to tell
 * two conditions with the same literal apart (`QuestionPage` and
 * `CheckAnswers` both measure to 67.5rem).
 *
 * The MOST COMMON scope class under `root`, not the first: a style-guide
 * demo page wraps the Template or organism under test in a little of its
 * own chrome (`check-answers/+page.svelte`'s "Show wide column" toggle),
 * which carries the DEMO page's own scope and would otherwise be the
 * first match found in document order -- the wrong component entirely.
 * The component actually under test contributes far more elements than a
 * couple of demo controls, so its scope wins the count.
 */
export function scopeClasses(root: Element): string[] {
	const counts = new Map<string, number>();
	for (const element of root.querySelectorAll('[class*="svelte-"]')) {
		const match = /svelte-[a-z0-9]+/.exec(element.getAttribute('class') ?? '');
		if (!match) continue;
		counts.set(match[0], (counts.get(match[0]) ?? 0) + 1);
	}
	return [...counts].toSorted(([, a], [, b]) => b - a).map(([scope]) => scope);
}
