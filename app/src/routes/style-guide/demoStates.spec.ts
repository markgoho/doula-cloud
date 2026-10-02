import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { ownSwitches, switchReport } from './demoStates.js';

/*
 * Demo pages that keep a state behind a click of their own and cannot
 * declare it as a variant, each with its reason (#1638). The same shape as
 * `route-continuum.svelte.spec.ts`'s `UNSWEPT`: a page with a switch and no
 * entry here fails the build, and so does an entry whose page no longer
 * has one.
 */
const CLICK_ONLY: Readonly<Record<string, string>> = {
	dialog:
		'A modal <dialog> opens in the top layer and is sized by the window (100dvw), not by the ' +
		'frame, so a sweep of the frame measures nothing of it; and an open modal makes the drag ' +
		"surface's own picker inert. #1674 holds the check a modal needs.",
	'confirm-dialog':
		'The same top-layer modal as `dialog`, opened by the same kind of button. #1674.',
	'landing-page':
		'The switch steps through the three shorter greetings. They are shorter strings in the ' +
		'same tree, and the page as it stands shows the longest, which is the one ADR-0025 asks ' +
		'the sweep to measure.'
};

const styleGuide = new URL('./', import.meta.url);

const pages = readdirSync(styleGuide, { withFileTypes: true })
	.filter((entry) => entry.isDirectory())
	.map((entry) => entry.name)
	.filter((slug) => existsSync(new URL(`${slug}/+page.svelte`, styleGuide)))
	.map((slug) => ({
		slug,
		switches: ownSwitches(readFileSync(new URL(`${slug}/+page.svelte`, styleGuide), 'utf8'))
	}));

describe('a style-guide page has no control of its own (#1638)', () => {
	it('finds no page that switches its own state from a click, outside CLICK_ONLY', () => {
		const unaccounted = pages
			.filter((page) => page.switches.length > 0 && !Object.hasOwn(CLICK_ONLY, page.slug))
			.map((page) => switchReport(page.slug, page.switches));

		expect(unaccounted).toEqual([]);
	});

	it('names no page in CLICK_ONLY that has no switch left', () => {
		const withSwitch = new Set(
			pages.filter((page) => page.switches.length > 0).map((page) => page.slug)
		);

		expect(Object.keys(CLICK_ONLY).filter((slug) => !withSwitch.has(slug))).toEqual([]);
	});

	// The scan reading nothing at all would pass both assertions above.
	it('reads the pages it claims to', () => {
		expect(pages.length).toBeGreaterThan(50);
		expect(pages.find((page) => page.slug === 'dialog')?.switches).toEqual(['isOpen']);
	});
});

// A demo page in miniature: its script, then its markup.
function demoPage(script: string[], markup: string[]): string {
	return ['<script lang="ts">', ...script, '</script>', ...markup].join('\n');
}

describe('ownSwitches', () => {
	it('names the state an inline onClick writes', () => {
		const source = demoPage(
			['let isEmpty = $state(false);'],
			['<Button label="Show the empty hub" onClick={() => (isEmpty = !isEmpty)} />']
		);

		expect(ownSwitches(source)).toEqual(['isEmpty']);
	});

	it('names a prop the page reassigns, the same as a $state', () => {
		const source = demoPage(
			["let { shown = 'content', isWide = false }: Properties = $props();"],
			["<button onclick={() => { shown = 'loading'; }}>Loading</button>"]
		);

		expect(ownSwitches(source)).toEqual(['shown']);
	});

	it.each([
		['a function', 'function toggle() {\n\tif (true) { hasError = !hasError; }\n}'],
		['a function with a return type', 'function toggle(): void {\n\thasError = !hasError;\n}'],
		['a const', 'const toggle = () => (hasError = !hasError);'],
		['a const with a block body', 'const toggle = () => {\n\tlog();\n\thasError = true;\n};']
	])('follows a handler passed by name to %s', (_kind, declaration) => {
		const source = demoPage(
			['let hasError = $state(false);', declaration],
			['<Button label="Show the error state" onClick={toggle} />']
		);

		expect(ownSwitches(source)).toEqual(['hasError']);
	});

	it('reads a typed $state and a logical assignment', () => {
		const source = demoPage(
			['let isShown: boolean | undefined = $state();'],
			['<Button label="Loading" onClick={() => (isShown ??= true)} />']
		);

		expect(ownSwitches(source)).toEqual(['isShown']);
	});

	it('reads a counter and a member write as writes', () => {
		const source = demoPage(
			['let index = $state(0);', 'let shown = $state({ picker: false });'],
			[
				'<Button label="Next" onClick={() => index++} />',
				'<Button label="Picker" onClick={() => (shown.picker = true)} />'
			]
		);

		expect(ownSwitches(source)).toEqual(['index', 'shown']);
	});

	// A field's value is a person typing into the component under
	// demonstration, not the page choosing a state.
	it('does not read a field value written from onInput as a switch', () => {
		const source = demoPage(
			["let email = $state('');"],
			['<TextInput value={email} onInput={(value) => (email = value)} />']
		);

		expect(ownSwitches(source)).toEqual([]);
	});

	it.each([
		['a handler that writes nothing', 'onClick={noop}', 'const noop = () => {};'],
		['a handler that only compares', 'onClick={() => log(isEmpty === true)}', ''],
		['a handler that passes an arrow on', 'onClick={() => run((isEmpty) => isEmpty)}', ''],
		['a handler declared somewhere else', 'onClick={imported}', '']
	])('does not read %s as a switch', (_kind, attribute, declaration) => {
		const source = demoPage(
			['let isEmpty = $state(false);', declaration],
			['<Button label="Add a client" ' + attribute + ' />']
		);

		expect(ownSwitches(source)).toEqual([]);
	});

	// A page caught mid-edit: the scan answers rather than running off the
	// end of the file.
	it('reads a handler whose brace never closes as far as the file goes', () => {
		const source = 'let isEmpty = $state(false);\n<Button onClick={() => (isEmpty = true)';

		expect(ownSwitches(source)).toEqual(['isEmpty']);
	});
});

describe('switchReport', () => {
	it('names the page, the state and the repair', () => {
		const report = switchReport('overview-hub', ['isEmpty']);

		expect(report).toContain('style-guide/overview-hub/+page.svelte');
		expect(report).toContain('(isEmpty)');
		expect(report).toContain('export `variants`');
	});
});
