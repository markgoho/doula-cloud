import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/*
 * The motion gate for the design brief -- wayfinder #418.
 *
 * `tokens.css` says "Motion. Enforcement (including prefers-reduced-motion)
 * is #418" and this is that enforcement. The brief fixes three durations
 * (120ms state, 180ms enter, 200ms navigation), one easing curve, and one
 * rule about reduced motion: under `prefers-reduced-motion: reduce`,
 * movement goes and feedback stays. None of that was checkable, and every
 * one of the things it forbids was already in the codebase.
 *
 * Like `tokens.spec.ts`, this parses the source rather than a rendered DOM.
 * A rendered check only sees the components some test happens to mount; a
 * parse sees every file, including the one somebody adds next week.
 *
 * ## The escape hatch
 *
 * A declaration that must break a rule carries `motion:ignore` in a comment
 * attached to it, with the reason -- the same shape as `coverage:ignore` in
 * `api/` and `v8 ignore` in `app/`. The reason is the point: an unexplained
 * exception is the thing this file exists to prevent.
 */

const SOURCE_ROOT = new URL('../../', import.meta.url).pathname;

/**
 * The three durations and the one curve the brief allows, from `tokens.css`.
 */
const DURATION_TOKENS = new Set(['--motion-state', '--motion-enter', '--motion-nav']);
const EASING_TOKENS = new Set(['--ease-out']);

/*
 * A motion token with no consumer is a promise, not a rule, so each one has
 * to name what will consume it. The assertion runs both ways: an unused
 * token missing from here fails, and a token listed here that has since
 * found a consumer fails too, so this map empties itself rather than rotting
 * into a permanent exemption list.
 */
const AWAITING_A_CONSUMER = new Map([
	[
		'--motion-enter',
		'Toasts, menus and revealed field groups -- none exist yet; the shell design is #431.'
	],
	['--motion-nav', 'The single per-navigation view transition -- no view transition exists yet.']
]);

/**
 * Properties whose animation is movement, which `reduce` must be able to drop.
 */
const MOVEMENT = /\b(?:transform|translate|scale|rotate|offset-path|all)\b/;

/**
 * Every match of `pattern` replaced by an equal run of spaces. Blanking
 * rather than deleting is the whole trick: every index into the result
 * still points at the same character in the original, so a range found in
 * the stripped text can be looked up in the text that still has its
 * comments.
 */
function blankRanges(raw: string, pattern: RegExp): string {
	let result = '';
	let cursor = 0;
	for (const match of raw.matchAll(pattern)) {
		const start = match.index ?? 0;
		result += raw.slice(cursor, start) + ' '.repeat(match[0].length);
		cursor = start + match[0].length;
	}
	return result + raw.slice(cursor);
}

function sourceFiles(): string[] {
	const found: string[] = [];
	(function walk(directory: string) {
		for (const entry of readdirSync(directory)) {
			const full = path.join(directory, entry);
			if (statSync(full).isDirectory()) {
				walk(full);
			} else if (entry.endsWith('.svelte') || entry.endsWith('.css')) {
				found.push(full);
			}
		}
	})(SOURCE_ROOT);
	return found.toSorted((left, right) => left.localeCompare(right));
}

/**
 * The character ranges holding CSS: the whole file for a stylesheet, each
 * `<style>` body for a component.
 */
function styleRanges(filePath: string, raw: string): [number, number][] {
	if (filePath.endsWith('.css')) return [[0, raw.length]];
	return raw
		.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)
		.map((match) => {
			const start = (match.index ?? 0) + match[0].indexOf('>') + 1;
			return [start, start + match[1].length] as [number, number];
		})
		.toArray();
}

/**
 * Comments replaced by spaces, so brace matching is not fooled by a `{` in
 * prose and every index still lines up with the original text.
 */
function withoutComments(raw: string): string {
	return blankRanges(raw, /\/\*[\s\S]*?\*\//g);
}

/**
 * Markup with `<script>`, `<style>` and HTML comments blanked out, indices
 * preserved. Without this the `<img>` scan matches prose: two route files
 * discuss `<img src>` in a comment explaining why an attachment needs a
 * session cookie, and a talked-about tag is not a rendered one.
 */
function markupOnly(raw: string): string {
	return blankRanges(raw, /<script[\s\S]*?<\/script>|<style[\s\S]*?<\/style>|<!--[\s\S]*?-->/g);
}

function closingBrace(text: string, open: number): number {
	let depth = 0;
	for (let index = open; index < text.length; index++) {
		if (text[index] === '{') depth++;
		else if (text[index] === '}' && --depth === 0) return index;
	}
	return text.length;
}

/**
 * The ranges guarded by `prefers-reduced-motion: no-preference`, which is
 * where movement is allowed to live.
 */
function noPreferenceRanges(blanked: string): [number, number][] {
	return blanked
		.matchAll(/@media[^{]*prefers-reduced-motion:\s*no-preference[^{]*\{/g)
		.map((match) => {
			const open = (match.index ?? 0) + match[0].length - 1;
			return [open, closingBrace(blanked, open)] as [number, number];
		})
		.toArray();
}

function isWithin(ranges: [number, number][], index: number): boolean {
	return ranges.some(([start, end]) => index >= start && index < end);
}

function commentRanges(raw: string): [number, number][] {
	return raw
		.matchAll(/\/\*[\s\S]*?\*\/|<!--[\s\S]*?-->/g)
		.map((match) => [match.index ?? 0, (match.index ?? 0) + match[0].length] as [number, number])
		.toArray();
}

/**
 * True when `motion:ignore` appears in the comment attached to whatever sits
 * at `index` -- either directly above it, or above the rule that encloses it.
 *
 * Walking the comment run backwards rather than counting lines is what lets
 * a reason be as long as it needs to be: a fixed two-line window would stop
 * excusing a declaration the moment somebody explained it properly, which is
 * exactly the wrong incentive.
 */
function isExcused(raw: string, index: number): boolean {
	const comments = commentRanges(raw);
	const isInComment = (at: number) => comments.some(([start, end]) => at >= start && at < end);

	let cursor = index;
	let hasLeftEnclosingRule = false;
	for (;;) {
		while (cursor > 0 && /\s/.test(raw[cursor - 1])) cursor--;

		const comment = comments.find(([, end]) => end === cursor);
		if (comment) {
			if (raw.slice(comment[0], comment[1]).includes('motion:ignore')) return true;
			cursor = comment[0];
			continue;
		}

		// Step out of the rule this declaration sits in, so a comment above
		// the selector excuses the declarations inside it. Once only: a marker
		// two rules up is too far away to be about this line. The `isInComment`
		// guard matters -- a prose semicolon inside the reason would otherwise
		// end the walk in the middle of the very comment being looked for.
		if (!hasLeftEnclosingRule && cursor > 0 && raw[cursor - 1] === '{') {
			hasLeftEnclosingRule = true;
			cursor--;
			const isAtBoundary = () =>
				('{};'.includes(raw[cursor - 1]) && !isInComment(cursor - 1)) ||
				comments.some(([, end]) => end === cursor);
			while (cursor > 0 && !isAtBoundary()) cursor--;
			continue;
		}

		return false;
	}
}

interface Declaration {
	file: string;
	property: string;
	value: string;
	movesSomething: boolean;
	isInNoPreference: boolean;
	isExcused: boolean;
}

function declarations(): Declaration[] {
	return sourceFiles().flatMap((filePath) => {
		const raw = readFileSync(filePath, 'utf8');
		const blanked = withoutComments(raw);
		const styles = styleRanges(filePath, raw);
		const allowed = noPreferenceRanges(blanked);

		return blanked
			.matchAll(/(transition|animation)(?:-duration|-timing-function)?\s*:\s*([^;{}]+);/g)
			.filter((match) => isWithin(styles, match.index ?? 0))
			.map((match) => ({
				file: path.relative(SOURCE_ROOT, filePath),
				property: match[1],
				value: match[2].replaceAll(/\s+/g, ' ').trim(),
				movesSomething: match[1] === 'animation' || MOVEMENT.test(match[2]),
				isInNoPreference: isWithin(allowed, match.index ?? 0),
				isExcused: isExcused(raw, match.index ?? 0)
			}))
			.toArray();
	});
}

/**
 * Every `var(--name)` in a value, plus the value with those `var()` calls
 * blanked out, so a token name can never be mistaken for a literal keyword
 * (`var(--ease-out)` must not read as the raw keyword `ease-out`).
 */
function tokensIn(value: string): { names: string[]; literals: string } {
	const names: string[] = [];
	const literals = value.replaceAll(/var\(\s*(--[\w-]+)\s*[^)]*\)/g, (_, name: string) => {
		names.push(name);
		return 'TOKEN';
	});
	return { names, literals };
}

function referenceCount(token: string): number {
	// The declaration in `tokens.css` is not a use of the token; only `var()` is.
	const uses = new RegExp(String.raw`var\(\s*${token}\b`, 'g');
	return sourceFiles()
		.map((filePath) => readFileSync(filePath, 'utf8').matchAll(uses).toArray().length)
		.reduce((total, count) => total + count, 0);
}

function describeOffender(declaration: Declaration): string {
	return `${declaration.file}: ${declaration.property}: ${declaration.value}`;
}

describe('motion is spent only on the brief’s durations and curve', () => {
	it('never writes a raw duration or easing keyword into a transition or animation', () => {
		const offenders = declarations()
			.filter((declaration) => !declaration.isExcused)
			.filter((declaration) => {
				const { literals } = tokensIn(declaration.value);
				const hasRawTime = /(?<![\w.-])\d*\.?\d+\s*m?s(?![\w-])/.test(literals);
				const hasRawEasing =
					/\b(?:ease(?:-in)?(?:-out)?|linear|step-(?:start|end))\b|steps\(|cubic-bezier\(/.test(
						literals
					);
				return hasRawTime || hasRawEasing;
			})
			.map((declaration) => describeOffender(declaration));

		expect(offenders).toEqual([]);
	});

	it('references only the three duration tokens and the one easing token', () => {
		const offenders = declarations().flatMap((declaration) =>
			tokensIn(declaration.value)
				.names.filter((name) => name.startsWith('--motion') || name.startsWith('--ease'))
				.filter((name) => !DURATION_TOKENS.has(name) && !EASING_TOKENS.has(name))
				.map((name) => `${declaration.file}: ${name}`)
		);

		expect(offenders).toEqual([]);
	});
});

describe('movement can always be switched off', () => {
	it('animates a transform only inside a prefers-reduced-motion: no-preference block', () => {
		const offenders = declarations()
			.filter((declaration) => declaration.movesSomething)
			.filter((declaration) => !declaration.isInNoPreference && !declaration.isExcused)
			.map((declaration) => describeOffender(declaration));

		expect(offenders).toEqual([]);
	});

	it('justifies every @keyframes, because a keyframe animation moves without being asked', () => {
		const offenders = sourceFiles().flatMap((filePath) => {
			const raw = readFileSync(filePath, 'utf8');
			const styles = styleRanges(filePath, raw);
			return withoutComments(raw)
				.matchAll(/@keyframes\s+([\w-]+)/g)
				.filter((match) => isWithin(styles, match.index ?? 0))
				.filter((match) => !isExcused(raw, match.index ?? 0))
				.map((match) => `${path.relative(SOURCE_ROOT, filePath)}: @keyframes ${match[1]}`)
				.toArray();
		});

		expect(offenders).toEqual([]);
	});
});

describe('the motion tokens are honest about who uses them', () => {
	it('names a future consumer for every token nothing uses yet', () => {
		const unused = [...DURATION_TOKENS, ...EASING_TOKENS].filter(
			(token) => referenceCount(token) === 0
		);
		expect(unused.filter((token) => !AWAITING_A_CONSUMER.has(token))).toEqual([]);
	});

	it('empties the waiting list once a token finds a consumer', () => {
		const adopted = AWAITING_A_CONSUMER.keys()
			.filter((token) => referenceCount(token) > 0)
			.toArray();
		expect(adopted).toEqual([]);
	});

	it('spends --motion-nav on at most one view transition, as the brief allows exactly one', () => {
		expect(referenceCount('--motion-nav')).toBeLessThanOrEqual(1);
	});
});

describe('nothing shifts when an image arrives', () => {
	it('gives every img an intrinsic width and height', () => {
		const components = sourceFiles().filter((filePath) => filePath.endsWith('.svelte'));
		const offenders = components.flatMap((filePath) => {
			const raw = readFileSync(filePath, 'utf8');
			return markupOnly(raw)
				.matchAll(/<img\b[\s\S]{0,600}?\/?>/g)
				.filter((match) => !(/\bwidth[=\s]/.test(match[0]) && /\bheight[=\s]/.test(match[0])))
				.filter((match) => !isExcused(raw, match.index ?? 0))
				.map(() => `${path.relative(SOURCE_ROOT, filePath)}: <img> without width/height`)
				.toArray();
		});

		expect(offenders).toEqual([]);
	});
});
