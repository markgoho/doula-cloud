/*
 * The discovery rule for a component demo's states (#1638, ADR-0025): a
 * style-guide page has no control of its own.
 *
 * The continuum check mounts a demo page and measures it. It never clicks.
 * Seven Template demos kept a second state -- an empty hub, an error
 * summary, a loading page -- behind a button in a `.controls` strip, so the
 * check measured the default state and the other was seen only by a person
 * who clicked. That is the hole a closed disclosure was before #710.
 *
 * A state is declared instead: the page takes it as a prop and exports
 * `variants` from its `<script module>` (`drag-surface/dragSurface.ts`),
 * which the check sweeps and the drag surface's picker offers under one
 * name. This module is what stops the old shape coming back. It reads a
 * page's SOURCE, because what it looks for is which mechanism the page
 * reached for, and a rendered page cannot say that (ADR-0025's reason for
 * the source gate).
 *
 * It is a plain module with no glob, so the spec beside it runs in the node
 * project and reads the pages from disk (#527).
 */

/*
 * Everything `let name = $state(...)` declares, and everything a
 * `let { a, b = 1 } = $props()` destructures: the page's own state.
 *
 * Limits, named: a prop renamed in the destructure (`{ a: b }`) is read as
 * `a`, and a default that is itself an object literal ends the list early.
 * No demo page has either.
 */
function stateNames(source: string): string[] {
	const declared = source
		.matchAll(/\blet\s+(\w+)\s*(?::[^=]+)?=\s*\$state\b/g)
		.map(([, name]) => name);
	const destructured = /\blet\s*\{([^}]*)\}[^;]*?=\s*\$props\(/.exec(source)?.[1] ?? '';
	const properties = destructured.matchAll(/(?:^|,)\s*(\w+)/g).map(([, name]) => name);
	return [...declared, ...properties];
}

// The text between the brace at `open` and the brace that closes it.
function insideBraces(source: string, open: number): string {
	let depth = 0;
	for (let index = open; index < source.length; index += 1) {
		if (source[index] === '{') depth += 1;
		if (source[index] === '}') depth -= 1;
		if (depth === 0) return source.slice(open + 1, index);
	}
	return source.slice(open + 1);
}

/*
 * What a handler runs. An inline arrow is its own text. A bare name is
 * looked up one level -- `function toggle() { ... }` or `const toggle =
 * ...;` -- because `onClick={toggle}` is the same switch with a name on it.
 * A handler the page imports, or one a second function calls for it, is
 * not followed.
 */
function handlerBody(source: string, expression: string): string {
	const name = /^\s*(\w+)\s*$/.exec(expression)?.[1];
	if (!name) return expression;
	const declaration = new RegExp(String.raw`\bfunction\s+${name}\s*\([^)]*\)[^{]*\{`).exec(source);
	if (declaration) {
		return insideBraces(source, declaration.index + declaration[0].length - 1);
	}
	const constant = new RegExp(String.raw`\bconst\s+${name}\s*=`).exec(source);
	if (!constant) return '';
	const assigned = source.slice(constant.index + constant[0].length);
	const block = /^[^;{]*=>\s*\{/.exec(assigned);
	return block ? insideBraces(assigned, block[0].length - 1) : assigned.split(';', 1)[0];
}

/**
The names of the page's own state that one of its own `onClick` handlers
writes, sorted. Empty for a page with no switch.

A click, and only a click. A field's value is written from `onInput`,
`onChange` or a `bind:`, which is a person typing into the component under
demonstration and not the page choosing which state to show. The limit
this leaves is named, not hidden: a Checkbox or a Select used as a state
switch reads the same in source as a field's value, and is not seen.
*/
export function ownSwitches(source: string): string[] {
	const names = stateNames(source);
	const written = new Set<string>();
	for (const handler of source.matchAll(/\bon[Cc]lick=\{/g)) {
		const expression = insideBraces(source, handler.index + handler[0].length - 1);
		const body = handlerBody(source, expression);
		for (const name of names) {
			const write = new RegExp(
				String.raw`(?<![\w.$])${name}(?:\.\w+|\[[^\]]*\])*\s*(?:(?:[-+*/%]|\*\*|\|\||&&|\?\?)?=(?![=>])|\+\+|--)`
			);
			if (write.test(body)) written.add(name);
		}
	}
	return [...written].toSorted((a, b) => a.localeCompare(b));
}

/**
The sentence a page with a switch of its own fails the build with. It
carries the repair, because the session that reads it is the one that
copied the old shape from a neighboring page.
*/
export function switchReport(slug: string, switches: readonly string[]): string {
	return [
		`style-guide/${slug}/+page.svelte writes its own state (${switches.join(', ')}) from an onClick.`,
		'The continuum check mounts a demo page and never clicks, so a state behind a button is',
		'measured never and cannot be picked on the drag surface (ADR-0025).',
		'Take the state as a prop, export `variants` from the page\'s <script module> with one',
		'named entry per state, and delete the button (.claude/rules/svelte-tests.md).',
		'If the state cannot be swept at all, name the page in CLICK_ONLY with the reason.'
	].join(' ');
}
