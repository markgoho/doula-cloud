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
 * `GLOSSARY.md` defines the continuum check and the drag surface as one
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
 * It sees inside a closed disclosure too, since #710: `sweep` opens every
 * closed `<details>` under the frame before measuring and closes it again
 * after, so a route that keeps content behind one -- the Client portal's
 * Activity ledger -- is measured on what is actually in it. That lives in
 * `continuum.ts` with the sweep, not here and not in a fixture, for the
 * reason the next section gives. A route whose disclosure FETCHES its
 * content when it opens is still measured on its loading state, since the
 * sweep opens and closes again inside one task and a `toggle` handler
 * never sees it open -- so #1126 opens them a second time here, in
 * preparation, once the fixture's answering has gone quiet and the rows
 * that carry them exist. The Staff roster is that case: what the sweep
 * measures is a Member's work-state history, not the word `Loading...`.
 *
 * ## It waits for the screen, not for its title
 *
 * The fixture names the screen's own `<h1>` and the mount below waits for
 * it, so a route drawing a Skeleton is not reported as a screen that fits.
 * That is a weaker wait than it reads: a route whose `+page.ts` already
 * handed it the record has its heading up on the first paint, so the wait
 * is over before the sections behind it have asked for anything. #885
 * found it -- the Staff Engagement hub's `onMount` awaits eight reads in
 * order, and the sweep was being taken after the first, on a screen whose
 * Contract section had not been drawn. So the mount also waits for the
 * fixture's answering to go quiet, which ADR-0025 records beside #710's
 * disclosures: a hole in what the instrument can see is the instrument's
 * to close, never a fixture's.
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
import {
	awaitSettled,
	isCanonicalEnvironment,
	mountInFrame,
	overflowReport,
	quiescence,
	RESOLUTION,
	revealDisclosures,
	sweep,
	TOLERANCE,
	type Mounted,
} from './style-guide/continuum.js';
import {
	atConditionWidth,
	conditionContainer,
	findConditions,
	forceLive,
	isTableFitAcceptable,
	measureTableFit,
	remToPx,
	scopeClasses,
	tableFitReport,
	type Condition,
	type TableFitMeasurement,
} from './style-guide/floor.js';
import {
	toApiResponder,
	toPageState,
	toRoutePath,
	toSweptFixtures,
	type RouteFixture,
} from './routeFixture.js';

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
	data: {} as Record<string, unknown>,
}));
vi.mock('$app/state', () => ({ page: pageState }));

/*
 * `goto` is imported at module scope by several routes and would reach
 * SvelteKit's real router, which no test has initialized. Nothing here
 * navigates -- the sweep measures, it does not interact -- so these exist
 * to be importable, not to be asserted on.
 */
vi.mock('$app/navigation', () => ({
	afterNavigate: vi.fn(),
	goto: vi.fn(),
	invalidate: vi.fn(),
	invalidateAll: vi.fn(),
	preloadCode: vi.fn(),
	pushState: vi.fn(),
	replaceState: vi.fn(),
	refreshAll: vi.fn(),
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
	// `/` and both login screens call this directly (#lib/api.js's own
	// doc comment) rather than through apiFetchWithSession -- mirrored
	// here rather than re-exported from the real module, since the real
	// one closes over the real apiFetch, not this file's mock.
	probeSession: async <Session>(path: string): Promise<Session | undefined> => {
		try {
			const response = await apiFetch(path);
			if (!response.ok) return undefined;
			return (await response.json()) as Session;
		} catch {
			return undefined;
		}
	},
}));

/*
 * Keys only -- no `eager`, so nothing here is imported. See the note above
 * on #527.
 */
const routePages = import.meta.glob('./**/+page.svelte');
const fixtureModules = import.meta.glob<{ fixture: RouteFixture }>(
	'./**/page.fixture.ts',
	{
		eager: true,
	}
);

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
		module.fixture,
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
 * actually lands. Empty again since #725 capped `DescriptionList`'s
 * label TRACK at `fit-content(40%)` and gave the `dt` the wrapping rule
 * that lets a pasted label reach that cap.
 */
const KNOWN_BROKEN: Readonly<Record<string, string>> = {};

if (!customElements.get('stack-l')) registerLayoutPrimitives();

beforeEach(() => {
	apiFetchWithSession.mockReset();
	apiFetch.mockReset();
});

/*
 * One route's screen, mounted the way the sweep below measures it: its
 * `page` and its answers installed, its heading up, its fetches quiet and
 * its disclosures open and filled in. The floor check over routes (#1945)
 * mounts the same screen through this, so the two read one screen.
 */
async function mountRoute(fixture: RouteFixture): Promise<Mounted> {
	// Which fields a fixture contributes is `toPageState`'s to say, so
	// the drag surface installs the same `page` this does (#597); how
	// it gets installed is each half's own business, since only this
	// half has a module to mock.
	Object.assign(pageState, toPageState(fixture));
	fixture.beforeMount?.();
	// And how a fixture's fetches get answered is `toApiResponder`'s
	// to say, for the same reason: since #596 every route spec that
	// fetches installs the same implementation, so unwrapping
	// `respond` here as well would be the third copy.
	let answered = 0;
	if (fixture.respond) {
		const respond = toApiResponder(fixture);
		const counted = (path: string) => {
			answered += 1;
			return respond(path);
		};
		apiFetchWithSession.mockImplementation(counted);
		apiFetch.mockImplementation(counted);
	}

	/*
	 * The one cast, and it is where a route's props actually arrive:
	 * `mountInFrame` takes the component tier's propless `Component`,
	 * and a route's `PageProps` is not assignable to it (see
	 * `routeFixture.ts`). The props themselves are the fixture's own
	 * `props`, checked against nothing here for the same reason a
	 * route's `load` output is checked by its own `+page.ts`.
	 */
	const mounted = await mountInFrame(fixture.component as Component, {
		...fixture.props,
	});
	const { frame } = mounted;
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
			.element(
				testPage.getByRole('heading', { name: fixture.readyText, level: 1 })
			)
			.toBeVisible();
		/*
		 * And then for the rest of the screen (#885). The heading
		 * above is on the page from the first paint on a route whose
		 * `load` already handed it one, so waiting for it says nothing
		 * about the sections that fill in behind it: this route's
		 * `onMount` awaits eight reads in order, and the sweep was
		 * taken after the first of them, measuring a screen whose
		 * Contract section had not been drawn at all. A fixture cannot
		 * close that -- the order is the route's -- so the instrument
		 * waits instead, the same reasoning #710 used for a closed
		 * disclosure: a hole in what the sweep can see belongs to the
		 * sweep.
		 *
		 * The wait is the fixture's own answering going quiet, not a
		 * duration: every response is served synchronously from
		 * `respond`, so one macrotask drains however many `await`s a
		 * section chains, and a cascade advances by at least one read
		 * per turn. The loop ends the first turn that asks for nothing
		 * new, and the bound is a guard against a route that polls
		 * rather than a budget anything is expected to spend.
		 */
		// A fresh reader per wait, rather than one closure serving both:
		// a `quiescence` carries the last count it saw, so a reused one
		// can report quiet without ever yielding a turn, and a wait that
		// can return without waiting is not the same wait twice.
		const answering = quiescence(() => answered);
		/*
		 * A bound that ran out is a screen still arriving, and
		 * measuring it anyway would reopen the hole this wait exists
		 * to close -- silently, which is the one way this file must
		 * not fail. `awaitSettled` throws instead, and it is the same
		 * bounded wait the disclosures below are revealed through:
		 * #1126 made it one function rather than two loops that would
		 * have had to be got right separately.
		 */
		await awaitSettled(answering, `${fixture.name} was still fetching`);
		/*
		 * And then the screen's own disclosures, opened and filled in
		 * (#1126). This is the second call -- `mountInFrame` made the
		 * first, before the route had fetched anything, and a route
		 * keeps its disclosures behind rows that only exist once the
		 * cascade above has run. The Staff roster was the case that made
		 * it: two history disclosures per Member, each fetching its own
		 * history when it opened, none of them in the DOM at mount, until
		 * #2041 moved both histories onto the Staff member's own page.
		 *
		 * It happens HERE, in preparation, and not inside `sweep` --
		 * which still opens and closes inside one task and still fires
		 * no handler, so the measurement itself asks the screen for
		 * nothing. What a fixture answers during this reveal is the
		 * route's own request, made because a disclosure is open, the
		 * same way a person opening one on the drag surface makes it.
		 */
		await revealDisclosures(frame, fixture.name);
		// What the reveal asked for, drained through the same wait the
		// cascade above used -- a history is a fetch like any other.
		await awaitSettled(
			quiescence(() => answered),
			`${fixture.name} was still fetching a disclosure's content`
		);
	} catch (error) {
		mounted.remove();
		throw error;
	}
	return mounted;
}

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

	/*
	 * One route, one map entry, and one or more screens (#913). A route
	 * that branches on the caller's session had only the branch its own
	 * fixture's session selected swept; `toSweptFixtures` is what expands
	 * the declared fixture into every branch it stands for, and the drag
	 * surface calls the same function for the same reason `toPageState`
	 * exists -- a field walked by each half separately is a field the two
	 * halves come to disagree about.
	 *
	 * The expansion happens HERE rather than in `fixtures` above, so the
	 * map stays keyed by route path: `sweeps or names every route this
	 * repo ships` and its stale-`UNSWEPT` companion both compare route
	 * paths, and a route with three branches must not read to them as
	 * three routes owing three entries. `KNOWN_BROKEN` is keyed the same
	 * way and so marks every branch of a route at once -- it is empty
	 * today, and a branch-specific entry is work for whoever needs one.
	 */
	// Sorted so the report reads in a stable order whatever the glob
	// returns; `flatMap` preserves that order, and a route's own branches
	// stay in the order its fixture declared them.
	const swept = [...fixtures]
		.toSorted(([a], [b]) => a.localeCompare(b))
		.flatMap(([routePath, declared]) =>
			toSweptFixtures(declared).map((fixture) => [routePath, fixture] as const)
		);

	for (const [routePath, fixture] of swept) {
		async function assertion() {
			const { run, frame, remove } = await mountRoute(fixture);
			try {
				const found = sweep(frame, run.clientWidth);
				expect(
					found,
					found && overflowReport(fixture.name, found)
				).toBeUndefined();
			} finally {
				remove();
			}
		}

		const brokenOn = KNOWN_BROKEN[routePath];
		if (brokenOn) {
			it.fails(
				`${fixture.name} (${brokenOn}) still needs more room than it is given`,
				assertion
			);
		} else {
			it(`${fixture.name} never needs more room than it is given`, assertion);
		}
	}
});

/*
 * The floor check, over routes (#1945).
 *
 * `style-guide/floor.svelte.spec.ts` checks every `@container` floor under
 * `src/lib/components` against that component's own demo. Since #1945 a
 * route can set a floor too: a `DataTable` shows its records until the
 * route's own containment context reaches the width at which that route's
 * table fits at its natural width, because below it the table's cells wrap
 * to more lines than they did at the old type floors -- the founder's
 * answer on #1945, and `DataTable.svelte`'s floor comment says why the
 * floor is the route's and not the component's. A route's floor is read
 * off its own fixture, so this file is the one that can mount it.
 *
 * Off the fixture's typical screens only (#1981), the variants marked
 * `typical`: the founder's answer of 2026-10-25 is that a table's floor
 * is measured on what a 14-doula agency really holds, not on the content
 * #537 makes hostile on purpose. On a hostile screen a table past its
 * floor wraps a long name or address onto more lines, and the continuum
 * sweep above is what holds that table inside its frame at every width.
 * A route floor that no typical screen renders fails.
 *
 * The same discovery as the component check -- `floor.ts`'s
 * `findConditions` over the raw source, never a list -- and the same two
 * halves: sufficient in every environment, and minimal in the canonical
 * one, where some width in the 5% below the floor does not fit (every
 * step of it, for the reason given where it is built). A route condition with no entry in `ROUTE_FLOORS` fails,
 * and so does an entry with no condition.
 */
const routeSources = import.meta.glob(['./**/+page.svelte', './**/*Screen.svelte'], {
	eager: true,
	query: '?raw',
	import: 'default',
}) as Record<string, string>;

const routeConditions = findConditions(
	Object.fromEntries(
		Object.entries(routeSources).filter(
			([modulePath]) => !modulePath.startsWith('./style-guide/')
		)
	)
);

// By the container's name, which a route gives each table it floors, and
// never by a width; an unnamed one falls back to its place in the file.
function toRouteFloorKey(condition: Condition): string {
	return `${toRoutePath(condition.file)}#${condition.containerName ?? condition.index}`;
}

/*
 * Which table each route floor is for. Every one is the table-fit
 * criterion (`floor.ts`'s `measureTableFit`), so the entry names the
 * table rather than a criterion. Since #1981 the tables whose typical
 * screen fits under `DataTable`'s own 55rem keep that floor and are not
 * here: the activity ledgers other than the hub's, a Client's history, both Contracts tables,
 * Engagement Requests, the schedule, Blocked email addresses and Pending
 * invitations. Since #2022 the Clients list is one of them: its cells were
 * cut to Name and a Status of badges, and its typical screen fits. Since
 * #2091 the Visits on an Engagement are too: their Notes column wraps, so
 * the table's floor is its facts beside a note's longest word. Since
 * #2098 the Engagement's Activity ledger and both histories on a Staff
 * member's page wrap their What column the same way, and since #2112
 * every activity ledger does: the hub's keeps a floor of its own, lowered
 * from 48rem to 28.5rem. Since #2146 the Invoices on an Engagement are
 * measured with the typical screen's rows too, with their Invoice and Raised
 * columns: an Owner's rows need 586px and an employee Doula's 480px
 * (macOS/Chromium), under 55rem, so they keep DataTable's own floor.
 */
const ROUTE_FLOORS: Readonly<Record<string, string>> = {
	// The hub's two tables floor LOWER than 55rem (#2063): both sit in its
	// primary column, which is narrower than that at a laptop's window.
	'practices/[practiceId]#activity-table': 'Recent activity',
	'practices/[practiceId]#waiting-table': 'Clients waiting on a reply',
	'practices/[practiceId]/invoices#invoices-table': 'The invoice list',
	'practices/[practiceId]/on-call#roster-table': 'The on-call roster',
	'practices/[practiceId]/staff#members-table': 'Staff members',
};

describe('the floor check, over routes (#1945)', () => {
	it('names the table of every route floor it discovers', () => {
		const unregistered = routeConditions
			.map((condition) => toRouteFloorKey(condition))
			.filter((key) => !Object.hasOwn(ROUTE_FLOORS, key));
		expect(unregistered, unregistered.join(', ')).toEqual([]);
	});

	it('names no route floor the source no longer has', () => {
		const discovered = new Set(
			routeConditions.map((condition) => toRouteFloorKey(condition))
		);
		const stale = Object.keys(ROUTE_FLOORS).filter(
			(key) => !discovered.has(key)
		);
		expect(stale, stale.join(', ')).toEqual([]);
	});

	it('finds every route floor in a route that has a fixture', () => {
		const unfixtured = routeConditions
			.filter((condition) => !fixtures.has(toRoutePath(condition.file)))
			.map((condition) => toRouteFloorKey(condition));
		expect(unfixtured, unfixtured.join(', ')).toEqual([]);
	});

	for (const condition of routeConditions) {
		const key = toRouteFloorKey(condition);
		const declared = fixtures.get(toRoutePath(condition.file));
		if (!declared || !Object.hasOwn(ROUTE_FLOORS, key)) continue; // reported above
		const screens = toSweptFixtures(declared).filter(
			(screen) => screen.typical
		);
		const floorPx = remToPx(condition.floorRem);
		// The component check's depth, for its reason: 5%, never less than
		// two steps of the sweep.
		const belowFloorPx =
			floorPx - Math.max(2 * RESOLUTION, Math.round(floorPx * 0.05));

		/*
		 * Every typical screen of the route that renders this table, measured at
		 * `px`. A screen that does not render it -- the landing hub with no
		 * Client yet draws its empty state, not its ledger -- is left out,
		 * and a floor no screen renders is reported by the caller.
		 */
		async function measureAt(
			widths: readonly number[]
		): Promise<TableFitMeasurement[]> {
			const measurements: TableFitMeasurement[] = [];
			for (const fixture of screens) {
				const { frame, remove } = await mountRoute(fixture);
				try {
					if (!conditionContainer(frame, condition)) continue;
					forceLive(condition, scopeClasses(frame));
					for (const px of widths) {
						atConditionWidth(frame, condition, px, TOLERANCE);
						/*
						 * A fixture's rows arrive after `mountInFrame` waited
						 * for the face, and the table is only shown from the
						 * sizing above, so any face its text asks for is
						 * requested now. Measuring before it lands reads the
						 * fallback.
						 */
						await document.fonts.ready;
						measurements.push(
							measureTableFit(conditionContainer(frame, condition)!)
						);
					}
				} finally {
					remove();
				}
			}
			return measurements;
		}

		/*
		 * Every step of the sweep's resolution from the probe depth up to
		 * just under the floor, not the one probe the component check
		 * takes. A table's natural width is not monotonic in the width it
		 * is given: measured in CI, the invoice list needed 1647px of a
		 * 1668px column and 1745px of a 1703px one, so a single probe 5%
		 * down can land in a width that fits while the floor is still the
		 * width above which the table always fits. Minimal means some
		 * width in that band does not fit.
		 */
		const belowFloorWidths = Array.from(
			{ length: Math.floor((floorPx - belowFloorPx) / RESOLUTION) },
			(_, step) => belowFloorPx + step * RESOLUTION
		);

		function report(measurements: TableFitMeasurement[]): string {
			return measurements.length === 0
				? `${key}: no typical screen of the route renders its ${condition.containerName} container.`
				: measurements
						.map((measurement) => tableFitReport(key, measurement))
						.join(' ');
		}

		it(`${key} is sufficient at its floor (${condition.floorRem}rem) on every typical screen`, async () => {
			const measurements = await measureAt([floorPx]);
			expect(
				measurements.length > 0 &&
					measurements.every((measurement) =>
						isTableFitAcceptable(measurement, TOLERANCE)
					),
				report(measurements)
			).toBe(true);
		});

		const minimalityName = `${key} is minimal: some typical screen's table does not fit somewhere in the ${floorPx - belowFloorPx}px below its floor`;
		if (isCanonicalEnvironment()) {
			it(minimalityName, async () => {
				const measurements = await measureAt(belowFloorWidths);
				expect(
					measurements.some(
						(measurement) => !isTableFitAcceptable(measurement, TOLERANCE)
					),
					report(
						measurements.filter(
							(_, index) => index % belowFloorWidths.length === 0
						)
					)
				).toBe(true);
			});
		} else {
			it.skip(`${minimalityName} (skipped outside the canonical environment)`, () => {});
		}
	}
});
