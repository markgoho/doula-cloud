import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';

/*
 * #2590: SvelteKit warns (`window_fetch_in_load`) when a `load` reads through
 * the window's `fetch`, and skips its own dedupe and replay. A route's `load`
 * reads through `apiFetch`, `streamRead` or `probeSession` only with the
 * `fetch` it is passed as the last argument, so a new load that forgets it
 * fails here rather than putting the warning on every walk.
 */

const routesRoot = fileURLToPath(new URL('../routes/', import.meta.url));
const READERS = ['apiFetch', 'streamRead', 'probeSession', 'readSession', 'hasAnyClient'];

const loadFiles = globFiles('**/+{layout,page}.ts', { cwd: routesRoot });

function withoutComments(source: string): string {
	return source.replaceAll(/\/\*[\s\S]*?\*\//g, '').replaceAll(/^\s*\/\/.*$/gm, '');
}

/*
 * The text between a call's parentheses, or undefined when they never close.
 */
function argumentsOf(source: string, open: number): string | undefined {
	let depth = 0;
	for (let index = open; index < source.length; index += 1) {
		if (source[index] === '(') depth += 1;
		if (source[index] === ')') depth -= 1;
		if (depth === 0) return source.slice(open + 1, index);
	}
	return undefined;
}

function callsWithoutFetch(source: string): string[] {
	const offenders: string[] = [];
	const readers = new RegExp(String.raw`(?<!function\s)\b(${READERS.join('|')})\(`, 'g');
	for (const match of source.matchAll(readers)) {
		const arguments_ = argumentsOf(source, match.index + match[0].length - 1);
		if (arguments_ === undefined || !/\bfetch(er)?\b/.test(arguments_)) {
			offenders.push(`${match[1]}(${arguments_ ?? ''})`);
		}
	}
	return offenders;
}

describe("a route's load reads through the fetch it is passed (#2590)", () => {
	it('finds the route loads', () => {
		expect(loadFiles.length).toBeGreaterThan(10);
	});

	it.each(loadFiles)('%s', (file) => {
		const source = withoutComments(readFileSync(path.join(routesRoot, file), 'utf8'));

		expect(callsWithoutFetch(source)).toEqual([]);
	});

	it('names a call that leaves its fetch out', () => {
		expect(callsWithoutFetch('await apiFetch(path);')).toEqual(['apiFetch(path)']);
		expect(callsWithoutFetch('await apiFetch(path, {}, fetch);')).toEqual([]);
		expect(callsWithoutFetch('async function readSession(path, fetcher) { return apiFetch(path, {}, fetcher); }')).toEqual([]);
	});
});
