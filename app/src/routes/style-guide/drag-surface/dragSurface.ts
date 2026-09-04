import type { Component } from 'svelte';
import { toRoutePath, type RouteFixture } from '../../routeFixture.js';

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
	 * Present only for a route. A component demo needs nothing: it is a
	 * Svelte component with no props. A route needs the environment its
	 * fixture describes -- props its `load` would have returned, the
	 * `page` its own code reads, and answers to the fetches it makes.
	 */
	fixture?: RouteFixture;
}

export interface PageModule {
	default: Component;
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
	const componentBySlug = new Map(
		Object.entries(modules).map(([modulePath, module]) => [toSlug(modulePath), module.default])
	);
	return pages.flatMap((page) => {
		const component = componentBySlug.get(page.slug);
		return component ? [{ name: page.name, slug: page.slug, component }] : [];
	});
}

/*
 * The route half of the picker. A route's fixture already names the
 * screen (`fixture.name`) for the check's own failure sentence, so the
 * surface reuses that rather than deriving a second display name -- the
 * two halves report a screen under one name.
 */
export function toRouteDemos(modules: Record<string, { fixture: RouteFixture }>): Demo[] {
	return Object.entries(modules)
		.map(([modulePath, module]) => ({
			name: module.fixture.name,
			slug: toRoutePath(modulePath),
			component: module.fixture.component as Component,
			fixture: module.fixture
		}))
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
