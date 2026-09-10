import { describe, expect, it } from 'vitest';
import { findDuplicateIds } from './duplicateIds.js';

/*
 * A browser spec rather than a Node one: the subject reads a real
 * `ParentNode`, and the Node project has no DOM to give it.
 */
function fragment(html: string): ParentNode {
	const host = document.createElement('div');
	host.innerHTML = html;
	return host;
}

describe('findDuplicateIds', () => {
	it('names nothing when every id is unique', () => {
		expect(findDuplicateIds(fragment('<p id="a"></p><p id="b"></p><p></p>'))).toEqual([]);
	});

	it('names a repeated id once however many copies of it there are', () => {
		expect(findDuplicateIds(fragment('<p id="a"></p><p id="a"></p><p id="a"></p>'))).toEqual(['a']);
	});

	it('names every repeated id, in the order each was first met', () => {
		expect(
			findDuplicateIds(fragment('<p id="b"></p><p id="a"></p><p id="b"></p><p id="a"></p>'))
		).toEqual(['b', 'a']);
	});

	it('counts an id under a display:none ancestor, which is the case axe skips', () => {
		expect(
			findDuplicateIds(fragment('<div style="display: none"><p id="a"></p></div><p id="a"></p>'))
		).toEqual(['a']);
	});
});
