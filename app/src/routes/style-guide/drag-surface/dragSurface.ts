import type { Component } from 'svelte';
import {
	toRoutePath,
	toSweptFixtures,
	toSweptSubjects,
	type RouteFixture
} from '../../routeFixture.js';

/*
 * The demo half of the drag surface (CONTEXT.md): the list of components a
 * reader can put inside the frame.
 *
 * Every component already has a style-guide page, and that page is an
 * ordinary Svelte component -- so the surface renders the page itself
 * rather than carrying a second copy of every demo's props. A fixture
 * improved for one is improved for both, which is what keeps the drag
 * surface and the continuum check one artifact rather than two.
 *
 * A route reaches the same frame through `page.fixture.ts` (#597), which
 * is the same file the continuum check sweeps -- so a route's content is
 * described once as well, and #570's narrowing of "one artifact seen two
 * ways" is undone rather than left standing.
 */

export interface Demo {
	name: string;
	slug: string;
	component: Component;
	/*
	 * What the subject is mounted with, for both tiers: the props a route's
	 * `load` would have returned, or the props that select one state of a
	 * component demo (#1638). The page as it stands has none.
	 */
	props?: Readonly<Record<string, unknown>>;
	/*
	 * Present only for a route. A component demo needs nothing more than
	 * `props`. A route needs the environment its fixture describes -- the
	 * `page` its own code reads, and answers to the fetches it makes.
	 */
	fixture?: RouteFixture;
}

/**
One other state a component demo renders, named as its own subject
([#1638](https://github.com/markgoho/doula-cloud/issues/1638)) -- a
`RouteVariant` (`routeFixture.ts`) cut down to the two fields a component
has.

`name` is the whole name, not a suffix, and unique across every subject:
the check titles its `it` with it and the picker keys on it. Name what
varies -- "Overview hub, empty".

`Properties` is the demo page's own props type and has no default, so a
variant is checked against the props its page really declares. Svelte
drops a prop a component does not declare without a word, and a variant
with a mistyped key would mount the page as it stands a second time under
a new name -- the silent failure #928 recorded for a route.
*/
export interface DemoVariant<Properties> {
	readonly name: string;
	readonly props: Readonly<Properties>;
}

export interface PageModule {
	default: Component;
	/**
	The other states this page renders, exported from its `<script module>`.
	Read through `toDemos` and nowhere else.
	*/
	variants?: readonly DemoVariant<Record<string, unknown>>[];
}

/**
`../description-list/+page.svelte` -> `description-list`.
*/
export function toSlug(modulePath: string): string {
	return modulePath.split('/').at(-2) ?? '';
}

export function toDemos(
	modules: Record<string, PageModule>,
	pages: readonly { name: string; slug: string }[]
): Demo[] {
	const moduleBySlug = new Map(
		Object.entries(modules).map(([modulePath, module]) => [toSlug(modulePath), module])
	);
	/*
	 * A page that renders more than one state offers one entry per state
	 * (#1638), the page as it stands first -- `floor.svelte.spec.ts` finds a
	 * demo by slug and must keep getting that one. Every state of a page
	 * keeps the page's slug, as every branch of a route keeps its path.
	 *
	 * This is the one reader of a page's `variants`, as `toSweptFixtures` is
	 * of a route's, and the two share the expansion itself.
	 */
	return pages.flatMap((page) => {
		const module = moduleBySlug.get(page.slug);
		if (!module) return [];
		return toSweptSubjects({
			name: page.name,
			slug: page.slug,
			component: module.default,
			variants: module.variants ?? []
		});
	});
}

/*
 * The route half of the picker. A route's fixture already names the
 * screen (`fixture.name`) for the check's own failure sentence, so the
 * surface reuses that rather than deriving a second display name -- the
 * two halves report a screen under one name.
 *
 * A route that renders differently for different callers offers one entry
 * per branch (#913), each under the name its own variant carries. The
 * expansion is `toSweptFixtures`, the same reader `route-continuum.svelte.
 * spec.ts` uses, so a branch that can be swept can be dragged and the two
 * halves cannot disagree about which branches exist.
 *
 * Every branch of one route keeps that route's slug: the slug names the
 * screen, and `toSorted` is stable, so the branches stay in the order the
 * fixture declared them and sit together in the picker.
 */
export function toRouteDemos(modules: Record<string, { fixture: RouteFixture }>): Demo[] {
	return Object.entries(modules)
		.flatMap(([modulePath, module]) =>
			toSweptFixtures(module.fixture).map((fixture) => ({
				name: fixture.name,
				slug: toRoutePath(modulePath),
				component: fixture.component as Component,
				props: fixture.props,
				fixture
			}))
		)
		.toSorted((a, b) => a.slug.localeCompare(b.slug));
}

/*
 * `fetch` takes a string, a `URL` or a `Request`, and a route reaches it
 * through all three: `#lib/api.js` passes a string, and a `Request` turns
 * up wherever one is built to carry headers.
 */
function toRequestedURL(input: RequestInfo | URL): string {
	if (typeof input === 'string') return input;
	return input instanceof URL ? input.href : input.url;
}

/*
 * What answers a dragged route's fetches (#597).
 *
 * `route-continuum.svelte.spec.ts` does this with `vi.mock('#lib/api.js')`,
 * which a running dev page has no equivalent of. It does not need one:
 * every API call this app makes funnels through a single line in
 * `#lib/api.js` -- `fetch(apiBaseURL() + path, ...)` -- so answering
 * `fetch` answers `apiFetch`, `apiFetchWithSession` and `probeSession`
 * alike, with no seam added to `api.ts` and no test-runner dependency in
 * the style-guide bundle.
 *
 * It is deliberately total rather than a passthrough with an escape: a
 * dragged route that reached the real BFF would render whatever a
 * developer's own database holds, which is a screen the check never
 * measured. An unfixtured path is a fixture that did not describe its
 * own screen, so it fails loudly here rather than quietly fetching.
 */
export function respondWith(
	respond: RouteFixture['respond'],
	baseURL: string,
	name: string
): typeof globalThis.fetch {
	return (input: RequestInfo | URL) => {
		const requested = toRequestedURL(input);
		const path = requested.startsWith(baseURL) ? requested.slice(baseURL.length) : requested;
		if (!respond) {
			return Promise.reject(
				new Error(
					`${name} fetched ${path} while being dragged, but its page.fixture.ts declares no respond(). ` +
						'The drag surface answers every fetch from the fixture so that what you drag is what the continuum check sweeps.'
				)
			);
		}
		return Promise.resolve(respond(path));
	};
}
