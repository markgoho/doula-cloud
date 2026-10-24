import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { NOT_SCANNED, SCANNED_ELSEWHERE } from '../../e2e/axeInventory';
import { globFiles } from './globFiles';

/*
 * #1810's gate: every screen is in the axe route inventory, or is named
 * with a reason as scanned elsewhere or not scanned. Twenty screens were
 * once in neither, because nothing made a new route join the inventory.
 * This runs in the unit suite, which `scripts/hooks/pre-commit` runs in
 * full, so a new `+page.svelte` with no row fails a commit rather than
 * waiting for someone to notice.
 *
 * The inventory is the `key` of every row in accessibility.e2e.ts, read
 * as text: a spec file cannot be imported here, since loading it would
 * register its Playwright tests. A key names its route the way
 * `routeName` below does, and a row that scans a state of a route rather
 * than the route itself says which state after it, in parentheses.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));
const e2eRoot = `${appRoot}e2e/`;
const inventorySource = readFileSync(`${e2eRoot}accessibility.e2e.ts`, 'utf8');

/**
 * A route's name: its path under `src/routes`, with each `(group)`
 * segment removed, and `/` for the root.
 */
function routeName(pageFile: string): string {
	const segments = pageFile
		.replace(/^src\/routes\//, '')
		.split('/')
		.slice(0, -1)
		.filter((segment) => !/^\(.+\)$/.test(segment));
	return segments.length === 0 ? '/' : segments.join('/');
}

const routes = globFiles('src/routes/**/+page.svelte', { cwd: appRoot }).map((file) => routeName(file));

/*
 * The guard fixtures at the end of accessibility.e2e.ts carry a `key`
 * too, but are not rows of the inventory: they exist to be refused.
 * `'*'` is a KNOWN entry's wildcard, not a route.
 */
const keys = inventorySource
	.matchAll(/\bkey: '([^']+)'/g)
	.map((match) => match[1]!)
	.filter((key) => key !== '*' && !/\(held /.test(key))
	.toArray();

/*
 * A row's route: its key without the state named after it.
 */
function keyRoute(key: string): string {
	return key.replace(/ \(.+\)$/, '');
}

const scanned = new Set(keys.map((key) => keyRoute(key)));
const listed = [...SCANNED_ELSEWHERE, ...NOT_SCANNED];

/*
 * Whether `name` (an entry's `route`) names `route`. `x/*` names `x` and
 * every route under it.
 */
function isNamedBy(name: string, route: string): boolean {
	if (!name.endsWith('/*')) return route === name;
	const parent = name.slice(0, -2);
	return route === parent || route.startsWith(`${parent}/`);
}

function isListed(route: string): boolean {
	return listed.some((entry) => isNamedBy(entry.route, route));
}

describe('the axe route inventory', () => {
	it('finds the routes and the rows it compares', () => {
		expect(routes.length).toBeGreaterThan(50);
		expect(keys.length).toBeGreaterThan(50);
	});

	it('reads every key, because every key is a single-quoted string', () => {
		const keyCount = inventorySource.match(/\bkey: ["'`]/g)?.length ?? 0;
		const singleQuoted = inventorySource.match(/\bkey: '/g)?.length ?? 0;

		expect(singleQuoted, 'write each row key as a single-quoted string, or this spec cannot read it').toBe(keyCount);
	});

	it('scans every route, or names it as scanned elsewhere or not scanned', () => {
		const missed = routes.filter((route) => !scanned.has(route) && !isListed(route));

		expect(
			missed,
			'add a row to app/e2e/accessibility.e2e.ts, or name the route with its reason in app/e2e/axeInventory.ts'
		).toEqual([]);
	});

	it('names only routes that exist, so a moved screen is not quietly unscanned', () => {
		const real = new Set(routes);

		expect([...scanned].filter((route) => !real.has(route)), 'a row key that is not a route').toEqual([]);
		expect(
			listed.map((entry) => entry.route).filter((name) => routes.every((route) => !isNamedBy(name, route))),
			'a listed route that is not a route'
		).toEqual([]);
	});

	it('gives every route that is not scanned here a reason', () => {
		expect(listed.filter((entry) => entry.reason.trim() === '')).toEqual([]);
	});

	it('points each route scanned elsewhere at a spec that runs axe', () => {
		const absent = SCANNED_ELSEWHERE.filter((entry) => {
			const spec = `${e2eRoot}${entry.spec}`;
			return !existsSync(spec) || !readFileSync(spec, 'utf8').includes('AxeBuilder');
		});

		expect(absent).toEqual([]);
	});

	it('agrees with the row count docs/testing/e2e.md gives', () => {
		const testingGuide = readFileSync(`${appRoot}../docs/testing/e2e.md`, 'utf8');
		const stated = /scans (\d+) rows/.exec(testingGuide)?.[1];

		expect(stated, 'docs/testing/e2e.md should say "scans <n> rows"').toBeDefined();
		expect(Number(stated)).toBe(new Set(keys).size);
	});
});
