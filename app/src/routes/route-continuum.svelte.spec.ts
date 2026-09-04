/*
 * The continuum check, over shipped routes (#570).
 *
 * `style-guide/continuum.svelte.spec.ts` sweeps the component demo
 * registry, which is every `.svelte` file under `src/lib/components`. A
 * route is not in that registry, so no screen this repo has ever shipped
 * had been swept by the repo's own check. The reach trial on
 * [#551](https://github.com/markgoho/doula-cloud/issues/551) is what made
 * that sharp: telling whether the screen it shipped laid out correctly
 * took a hand-written throwaway harness written for one route, and
 * [#521](https://github.com/markgoho/doula-cloud/issues/521)'s screen --
 * 93px past its edge at 320px -- would not have fired the check either.
 * ADR-0025 asks that a surface be verified across the continuum; a surface
 * a person actually visits is a route.
 *
 * ## This is the same sweep, not a third one
 *
 * `CONTEXT.md` defines the continuum check and the drag surface as one
 * artifact seen two ways, and #550 is what happens when the two halves
 * drift. So this file imports `sweep`, `mountInFrame` and
 * `overflowReport` from `style-guide/continuum.js` and adds nothing of its
 * own to any of them: the same 320px start, the same 4px resolution, the
 * same `Math.max(availableSpace, CONFORMANCE_COMMITMENT)` ceiling read off
 * an unconstrained run, the same wait for the real webfont, the same
 * failure sentence pointing at the layout exercise. A route reaching the
 * instrument differently is the only new thing here.
 *
 * `mountInFrame` moved into `continuum.ts` on this ticket for that reason.
 * It had been inline in the component check's own `it` and copied into
 * `floor.svelte.spec.ts`; this would have been the third copy, and a third
 * copy is where "one artifact" stops being true.
 *
 * ## A route reaches the drag surface too
 *
 * This file once recorded a narrowing: a route could be swept here and
 * could not be dragged, because a style-guide page is a Svelte component
 * with no props while a route reads `page.params`, takes `data` from its
 * own `load`, and fetches -- and `vi.mock` does not exist in a running dev
 * page. [#597](https://github.com/markgoho/doula-cloud/issues/597) undid
 * it, and the fixture below is what both halves now read.
 *
 * The dev page needs no `vi.mock`. It reads `page` through
 * `#lib/appState.svelte.js`, which every route imports instead of
 * `$app/state` -- and which reads `$app/state` rather than replacing it,
 * so the mock right below still works here unchanged. For fetches it
 * answers `fetch` itself, since every API call this app makes funnels
 * through one line in `#lib/api.js`.
 *
 * ## How a route joins: discovery, never opt-in
 *
 * The route list is a glob, and a route is swept if it has a
 * `page.fixture.ts` beside it. A route with neither a fixture nor an
 * entry in `UNSWEPT` below fails this file loudly. That shape is
 * `floor.svelte.spec.ts`'s (`has a criterion for every discovered
 * condition` plus `UNDERIVABLE`), and it is chosen over each route spec
 * opting in because #521 is this map's own proof that a carrier a session
 * can walk past gets walked past. There is no width in the mechanism: it
 * is every route this repo ships.
 *
 * ## What this measures, and what it does not
 *
 * The window Vitest gives this file is 414px wide -- measured here, not
 * assumed -- so a route is swept from 320px to 414px and no further. The
 * reach trial's own throwaway harness swept from 1400px down to 280px, so
 * the check that replaced it makes a narrower statement than the harness
 * did. [#600](https://github.com/markgoho/doula-cloud/issues/600) took
 * that up and decided the ceiling STAYS, for a reason that belongs to both
 * checks at once and is written out beside `sweep` in `continuum.ts` and
 * in ADR-0025: an unconstrained component runs out of space at 320px,
 * where this already looks, and a constrained one runs out nowhere. The
 * one case a wider sweep would reach -- a different DOM tree above a
 * content floor -- is `floor.svelte.spec.ts`'s. The narrow window is
 * load-bearing elsewhere too: #544 records it as the reason
 * `layout.usage.spec.ts` rule 3 reads source rather than rendering.
 *
 * The `+page.svelte` glob is deliberately LAZY. #527 lost six points of
 * coverage to an eager glob that dragged every component into a report at
 * near-zero execution; a lazy glob yields keys and loaders and imports
 * nothing, so discovery costs nothing at all and only fixtured routes are
 * ever loaded -- through their own fixture, which names its `Page` import
 * directly.
 */
import type { Component } from 'svelte';
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { mountInFrame, overflowReport, sweep } from './style-guide/continuum.js';
import { toApiResponder, toPageState, toRoutePath, type RouteFixture } from './routeFixture.js';

/*
 * One mock of `$app/state` for every route, because `vi.mock` is hoisted
 * per file and cannot be re-declared per subject. The object is mutable
 * and each fixture sets it before its own mount -- `page` is read inside
 * the route's own functions rather than destructured at module scope, so
 * a later write is seen.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	// What an ancestor `+layout.ts` merged into `page.data` (#595) -- the
	// four routes under `portal/(authenticated)/engagements/
	// [engagementId]/+layout.ts` read `page.data.practiceName` via
	// `$app/state` rather than through a component `data` prop, so they
	// need this set alongside `params`/`url`. Every other route's own
	// `data` arrives through `fixture.props` instead and leaves this `{}`.
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

/*
 * `goto` is imported at module scope by several routes and would reach
 * SvelteKit's real router, which no test has initialized. Nothing here
 * navigates -- the sweep measures, it does not interact -- so these exist
 * to be importable, not to be asserted on.
 */
vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	invalidate: vi.fn(),
	invalidateAll: vi.fn()
}));

/*
 * Every export of `#lib/api.js`, not only the one a given route happens to
 * call: a partial mock of a module is a module whose missing export is
 * `undefined` at the call site, which fails as a confusing TypeError
 * rather than as an honest missing fixture.
 */
const apiFetchWithSession = vi.hoisted(() => vi.fn());
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetch,
	apiFetchWithSession,
	apiBaseURL: () => 'https://api.example.test',
	apiErrorMessage: (response: Response) => Promise.resolve(`HTTP ${response.status}`),
	// `/` and both login screens call this directly (#lib/api.js's own
	// doc comment) rather than through apiFetchWithSession -- mirrored
	// here rather than re-exported from the real module, since the real
	// one closes over the real apiFetch, not this file's mock.
	probeSession: async <Session,>(path: string): Promise<Session | undefined> => {
		try {
			const response = await apiFetch(path);
			if (!response.ok) return undefined;
			return (await response.json()) as Session;
		} catch {
			return undefined;
		}
	}
}));

/*
 * Keys only -- no `eager`, so nothing here is imported. See the note above
 * on #527.
 */
const routePages = import.meta.glob('./**/+page.svelte');
const fixtureModules = import.meta.glob<{ fixture: RouteFixture }>('./**/page.fixture.ts', {
	eager: true
});

/*
 * The style guide is the component check's own territory: every page under
 * it is a demo of one component, swept by `continuum.svelte.spec.ts`
 * through the same registry the drag surface uses. Sweeping them again
 * here would measure the same thing twice and report every break under two
 * names.
 */
const routePaths = Object.keys(routePages)
	.map((modulePath) => toRoutePath(modulePath))
	.filter((routePath) => !routePath.startsWith('style-guide'))
	.toSorted((a, b) => a.localeCompare(b));

const fixtures = new Map(
	Object.entries(fixtureModules).map(([modulePath, module]) => [
		toRoutePath(modulePath),
		module.fixture
	])
);

/*
 * Routes with no fixture yet, every one of them named. This is
 * deliberately a list rather than a filter: a new route joining the repo
 * with no fixture and no entry here fails `every route is either swept
 * or named` below, so the sweep's coverage cannot quietly stop growing
 * with the app.
 */
const UNSWEPT_ON = '#595';
const UNSWEPT: readonly string[] = [];

/*
 * Known-broken today, named rather than suppressed -- the same shape and
 * the same reason as the component check's own `KNOWN_BROKEN`. `it.fails`
 * turns red, not green, the day an entry's own issue closes without this
 * file changing -- that is what forces the entry out once the retrofit
 * actually lands. Empty since #530 (this route's own DescriptionList
 * value column) landed.
 */
const KNOWN_BROKEN: Readonly<Record<string, string>> = {};

if (!customElements.get('stack-l')) registerLayoutPrimitives();

beforeEach(() => {
	apiFetchWithSession.mockReset();
	apiFetch.mockReset();
});

describe('the continuum check, over routes', () => {
	it('sweeps or names every route this repo ships', () => {
		const unaccounted = routePaths.filter(
			(routePath) => !fixtures.has(routePath) && !UNSWEPT.includes(routePath)
		);
		expect(
			unaccounted,
			`No page.fixture.ts and no entry in UNSWEPT (${UNSWEPT_ON}) for: ${unaccounted.join(', ')}. ` +
				'A route joins this sweep by discovery, never by opting in -- see this file for why.'
		).toEqual([]);
	});

	/*
	 * An exception has to stay pointed at something real. A route that has
	 * since gained a fixture, or that no longer exists, leaves a line here
	 * claiming coverage is owed where none is -- which is how a list of
	 * exceptions becomes a list nobody reads.
	 */
	it('names no unswept route the repo no longer has, or already sweeps', () => {
		const stale = UNSWEPT.filter(
			(routePath) => !routePaths.includes(routePath) || fixtures.has(routePath)
		);
		expect(stale, stale.join(', ')).toEqual([]);
	});

	// Sorted so the report reads in a stable order whatever the glob returns.
	const swept = [...fixtures].toSorted(([a], [b]) => a.localeCompare(b));

	for (const [routePath, fixture] of swept) {
		async function assertion() {
			// Which fields a fixture contributes is `toPageState`'s to say, so
			// the drag surface installs the same `page` this does (#597); how
			// it gets installed is each half's own business, since only this
			// half has a module to mock.
			Object.assign(pageState, toPageState(fixture));
			// And how a fixture's fetches get answered is `toApiResponder`'s
			// to say, for the same reason: since #596 every route spec that
			// fetches installs the same implementation, so unwrapping
			// `respond` here as well would be the third copy.
			if (fixture.respond) {
				const respond = toApiResponder(fixture);
				apiFetchWithSession.mockImplementation(respond);
				apiFetch.mockImplementation(respond);
			}

			/*
			 * The one cast, and it is where a route's props actually arrive:
			 * `mountInFrame` takes the component tier's propless `Component`,
			 * and a route's `PageProps` is not assignable to it (see
			 * `routeFixture.ts`). The props themselves are the fixture's own
			 * `props`, checked against nothing here for the same reason a
			 * route's `load` output is checked by its own `+page.ts`.
			 */
			const { run, frame, remove } = await mountInFrame(fixture.component as Component, {
				...fixture.props
			});
			try {
				/*
				 * A route that loads in `onMount` draws a Skeleton first, and a
				 * sweep taken then measures the loading state -- which fits at
				 * any width and would report every such route as passing. The
				 * fixture names the screen's own heading, so this waits for the
				 * subject rather than for a timeout.
				 *
				 * By role rather than by text: a screen's heading words turn up
				 * again in its own content -- `Invoices` is both the `<h1>` and
				 * part of the `Unpaid invoices` label beside the total -- and a
				 * text query matching two elements fails as a strict-mode
				 * violation rather than as a fixture that named the wrong thing.
				 */
				await expect
					.element(testPage.getByRole('heading', { name: fixture.readyText, level: 1 }))
					.toBeVisible();
				const found = sweep(frame, run.clientWidth);
				expect(found, found && overflowReport(fixture.name, found)).toBeUndefined();
			} finally {
				remove();
			}
		}

		const brokenOn = KNOWN_BROKEN[routePath];
		if (brokenOn) {
			it.fails(`${fixture.name} (${brokenOn}) still needs more room than it is given`, assertion);
		} else {
			it(`${fixture.name} never needs more room than it is given`, assertion);
		}
	}
});
