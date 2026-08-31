import { describe, expect, it } from 'vitest';
import { styleLines } from './styleLines';

/*
 * `styleLines` is the shared scanner under `tokens.usage.spec.ts` and
 * `layout.usage.spec.ts`. Both of those exercise it across every component
 * in the application, which proves it works on the shapes the repo happens
 * to contain today -- and proves nothing about the shapes it does not.
 *
 * The marker convention allows a reason to be written as a multi-line
 * comment, and nothing in the codebase currently puts the marker anywhere
 * but the opening line. So the continuation-line case has never run, and
 * the first author to write a reason that reads better on its second line
 * would silently lose their exception. That case is the reason this file
 * exists; the rest is the surrounding behaviour it would be dishonest to
 * assert around without.
 */

function wrap(css: string): string {
	return `<div class="x"></div>\n<style>\n${css}\n</style>\n`;
}

const MARKER = 'layout:ignore';

describe('styleLines', () => {
	it('returns only what is inside a style block', () => {
		const source = `<script>const inline-size = 1;</script>\n${wrap('.x { color: red; }')}`;
		expect(styleLines(source, MARKER).map(({ text }) => text.trim())).toEqual([
			'.x { color: red; }'
		]);
	});

	it('drops a rule whose marker sits on the comment it opens', () => {
		const source = wrap(`/* ${MARKER} -- a reason */\n.x {\n\tcolor: red;\n}\n.y {\n\tcolor: blue;\n}`);
		expect(styleLines(source, MARKER).map(({ text }) => text.trim())).toEqual([
			'.y {',
			'color: blue;',
			'}'
		]);
	});

	it('drops a rule whose marker sits on a later line of a multi-line comment', () => {
		const source = wrap(
			`/* A reason long enough to wrap, so the marker\n   lands here: ${MARKER}\n   and the rule below is excused. */\n.x {\n\tcolor: red;\n}\n.y {\n\tcolor: blue;\n}`
		);
		expect(styleLines(source, MARKER).map(({ text }) => text.trim())).toEqual([
			'.y {',
			'color: blue;',
			'}'
		]);
	});

	it('keeps the exception in force to the end of a nested marked block', () => {
		const source = wrap(
			`/* ${MARKER} -- a reason */\n@media (min-width: 30rem) {\n\t.x {\n\t\tcolor: red;\n\t}\n}\n.y {\n\tcolor: blue;\n}`
		);
		expect(styleLines(source, MARKER).map(({ text }) => text.trim())).toEqual([
			'.y {',
			'color: blue;',
			'}'
		]);
	});

	it('ignores a marker belonging to a different gate', () => {
		const source = wrap(`/* tokens:ignore -- not this gate's exception */\n.x {\n\tcolor: red;\n}`);
		expect(styleLines(source, MARKER).map(({ text }) => text.trim())).toEqual([
			'.x {',
			'color: red;',
			'}'
		]);
	});

	/*
	 * The `css` mode, which `layout.usage.spec.ts` needs so the gate can
	 * reach the token layer (ADR-0025). A plain stylesheet has no `<style>`
	 * tag to open the block, so the default mode returns nothing at all from
	 * one -- which is exactly the blind spot #532 fell into.
	 */
	describe('a source that is a stylesheet rather than one that contains it', () => {
		it('returns nothing from a plain stylesheet in the default mode', () => {
			expect(styleLines('.x {\n\tcolor: red;\n}', MARKER)).toEqual([]);
		});

		it('reads every line of one in css mode', () => {
			expect(styleLines('.x {\n\tcolor: red;\n}', MARKER, 'css').map(({ text }) => text.trim())).toEqual([
				'.x {',
				'color: red;',
				'}'
			]);
		});

		it('honours the marker there too', () => {
			const source = `/* ${MARKER} -- a reason */\n.x {\n\tcolor: red;\n}\n.y {\n\tcolor: blue;\n}`;
			expect(styleLines(source, MARKER, 'css').map(({ text }) => text.trim())).toEqual([
				'.y {',
				'color: blue;',
				'}'
			]);
		});
	});
});
