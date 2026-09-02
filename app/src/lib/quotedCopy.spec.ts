import { describe, expect, it } from 'vitest';
import { quotedStringsInSource, withoutComments } from './quotedCopy';

/*
 * `formErrors.usage.spec.ts` and `copy.pronoun.usage.spec.ts` exercise this
 * walk incidentally, against every real component and route -- but neither
 * happens to write a file with an unterminated block comment, or two
 * comment markers racing on one line, so those branches need a synthetic
 * case of their own to reach 100% (docs/testing.md).
 */
describe('withoutComments', () => {
	it('drops a line-comment line entirely', () => {
		expect(withoutComments("// a comment\nconst x = 'kept';")).toEqual(['', "const x = 'kept';"]);
	});

	it('drops a same-line block comment and keeps the rest', () => {
		expect(withoutComments("const x = /* note */ 'kept';")).toEqual(["const x =  'kept';"]);
	});

	it('drops a same-line HTML comment and keeps the rest', () => {
		expect(withoutComments("<!-- note --> <p>kept</p>")).toEqual([' <p>kept</p>']);
	});

	it('carries a block comment across lines until it closes', () => {
		const source = ['/* opens', 'still open', 'closes */ kept'].join('\n');

		expect(withoutComments(source)).toEqual(['', '', ' kept']);
	});

	it('carries an HTML comment across lines until it closes', () => {
		const source = ['<!-- opens', 'still open', 'closes --> kept'].join('\n');

		expect(withoutComments(source)).toEqual(['', '', ' kept']);
	});

	it('treats an unterminated HTML comment as open when it starts after a block comment', () => {
		// Neither marker closes on this line, so whichever one opens later
		// is the one still open at end of line; everything up to it is kept.
		expect(withoutComments('a/*b<!--c')).toEqual(['a/*b']);
	});

	it('treats an unterminated block comment as open when it starts after an HTML comment', () => {
		expect(withoutComments('a<!--b/*c')).toEqual(['a<!--b']);
	});
});

describe('quotedStringsInSource', () => {
	it('reads a single-quoted literal', () => {
		expect(quotedStringsInSource("const x = 'kept';")).toEqual([{ line: 1, text: 'kept' }]);
	});

	it('reads a double-quoted literal', () => {
		expect(quotedStringsInSource('const x = "kept";')).toEqual([{ line: 1, text: 'kept' }]);
	});

	it('reads a template-literal', () => {
		expect(quotedStringsInSource('const x = `kept`;')).toEqual([{ line: 1, text: 'kept' }]);
	});

	it('ignores a quote inside a stripped comment', () => {
		expect(quotedStringsInSource("// 'not kept'\nconst x = 'kept';")).toEqual([
			{ line: 2, text: 'kept' }
		]);
	});
});
