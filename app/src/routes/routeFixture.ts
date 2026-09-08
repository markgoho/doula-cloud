/*
 * What a route needs to be put in front of the continuum check (#570).
 *
 * A component demo needs nothing: it is a Svelte component with no props,
 * so `toDemos` can hand the sweep a whole style-guide page as it stands.
 * A route is not that. It reads `page.params` out of `$app/state`, it
 * takes `data` from its own `load`, and it fetches through
 * `#lib/api.js` in `onMount` -- the reach trial on #551 needed a
 * hand-written harness for exactly this reason, and the harness it needed
 * was written for one route.
 *
 * So a route declares the four things instead of the check guessing them.
 * The declaration lives beside the route as `page.fixture.ts`, not in a
 * central registry, so the content the sweep measures and the content the
 * route's own spec asserts on are one object rather than two that drift.
 *
 * That last clause read "can become one object" until
 * [#596](https://github.com/markgoho/doula-cloud/issues/596), which is
 * the ticket that made it true: every route spec now imports its
 * happy-path content from the fixture beside it rather than declaring its
 * own. The rule, and what a spec still owns, is in
 * `.claude/rules/svelte-tests.md`.
 *
 * The content itself is hostile, never polite (ADR-0025, #537): a route's
 * values come from a Practice's own typing rather than from a file, so a
 * fixture holding representative content measures a screen nobody will
 * ever see. #521's approval screen fits at 320px on polite content and is
 * 93px past its edge on one pasted URL.
 */
import type { Component } from 'svelte';

/**
The route a module path belongs to, for both halves of the check
(CONTEXT.md): `./practices/[practiceId]/invoices/+page.svelte` and
`../../practices/[practiceId]/invoices/page.fixture.ts` alike read as
`practices/[practiceId]/invoices`, so a break found by dragging and a
break found by the sweep name the same screen.

It lives here rather than in either caller because it was about to be
written twice, and the second copy had already drifted before anyone
compared them -- the drag surface's version required a separator the
check's version made optional, which loses the root route, whose fixture
is `page.fixture.ts` with no directory in front of it. That is the same
finding [#570](https://github.com/markgoho/doula-cloud/issues/570)
recorded when it moved `mountInFrame` into `continuum.ts`: one artifact
is enforced by there being one function, not by two files agreeing.

The relative prefix differs because the two callers glob from different
directories, and the fixture is `page.fixture.ts` rather than
`+page.fixture.ts` because a leading `+` is reserved by SvelteKit's own
routing.
*/
export function toRoutePath(modulePath: string): string {
	return modulePath
		.replace(/^(\.\.?\/)+/, '')
		.replace(/\/?(\+page\.svelte|page\.fixture\.ts)$/, '');
}

/**
The `page` a route reads, built from its fixture.

Both halves of the check install this, and they install it differently
because they have to: `route-continuum.svelte.spec.ts` writes it onto a
hoisted object behind `vi.mock('$app/state')`, while the drag surface
hands it to `overridePage` on a page where no module can be mocked. What
they must not differ about is WHICH fields a fixture contributes -- a
fifth field added to `RouteFixture` would otherwise reach whichever half
someone remembered, and the two would then be measuring and showing
different screens. Reading the fixture is one function; installing the
result is each half's own business.
*/
export function toPageState(fixture: RouteFixture): {
	params: Record<string, string>;
	url: URL;
	data: Record<string, unknown>;
} {
	return {
		params: { ...fixture.params },
		url: new URL(fixture.url),
		data: { ...fixture.pageData }
	};
}

/**
The mock implementation installed on `apiFetchWithSession` / `apiFetch`
by a route's own spec and by the check alike.

`fixture.respond` is unwrapped here rather than in each of the route
specs that fetch plus both halves of the check, which is #570's
`mountInFrame` rule again: one artifact is enforced by there being one
function. A `Response` body reads once, so the fixture is called per
request rather than resolved once into a `mockResolvedValue` -- a spec
that renders its route twice would otherwise get a consumed body the
second time.
*/
export function toApiResponder(fixture: RouteFixture): (path: string) => Promise<Response> {
	const respond = fixture.respond;
	if (!respond) {
		throw new Error(`${fixture.name} declares no respond(), so nothing can answer a fetch for it.`);
	}
	return (path: string) => Promise.resolve(respond(path));
}

/*
 * `RouteParameters` exists because a fixture's `params` is handed to two different
 * consumers with two different demands. The check installs it on a mocked
 * `$app/state`, which wants any string map; a route's own spec passes it
 * to `render` as a `PageProps`, whose `params` is the route's generated
 * `RouteParams` and names each id. Erasing it to `Record<string, string>`
 * for both left the spec with a cast asserting what the fixture already
 * knows, so a fixture whose spec passes it through says its own shape --
 * `RouteFixture<RouteParams>` -- and every other fixture takes the
 * default and is unchanged (#596).
 */
export interface RouteFixture<RouteParameters extends Record<string, string> = Record<string, string>> {
	/**
	How the route is named in a failure sentence -- what a person calls the screen.
	*/
	readonly name: string;
	/*
	 * The route component itself. `Component<never>` rather than a bare
	 * `Component`, which means `Component<{}>`: a route's own `PageProps`
	 * is not assignable to that, because a component's props are
	 * contravariant and a route demands `data` where `{}` offers nothing.
	 * `never` is assignable to any props type, so this accepts every route
	 * and the sweep casts once where it mounts -- which is honest, since
	 * what a route's props actually are is `props` below, not this.
	 */
	readonly component: Component<never>;
	/**
	`page.params` for the mocked `$app/state`.
	*/
	readonly params: Readonly<RouteParameters>;
	/**
	`page.url` for the mocked `$app/state`.
	*/
	readonly url: string;
	/**
	Props the route's own `load` would have handed it, if it has one.
	*/
	readonly props?: Readonly<Record<string, unknown>>;
	/**
	Every `apiFetchWithSession` / `apiFetch` this route makes, answered by
	path. A route that fetches nothing omits it.
	*/
	readonly respond?: (path: string) => Response;
	/**
	`page.data` for the mocked `$app/state` -- what an ancestor
	`+layout.ts`'s `load` merged in (#595). Distinct from `props`: a
	route's OWN `+page.ts`/`+page.server.ts` result arrives as a `data`
	component prop, but `portal/(authenticated)/engagements/
	[engagementId]/+layout.ts` hands its result down through
	`page.data` instead, read via `$app/state` rather than a prop. The
	four routes under that layout are the only ones that need this; every
	other route omits it.
	*/
	readonly pageData?: Readonly<Record<string, unknown>>;
	/**
	The screen's own level-1 heading, matched by role. It must be on the page before a measurement is taken. A route
	that loads in `onMount` renders a Skeleton first, and sweeping that
	measures the loading state rather than the screen -- an honest wait, not
	a timeout.
	*/
	readonly readyText: string;
	/**
	The other sessions this same screen renders differently under
	([#913](https://github.com/markgoho/doula-cloud/issues/913)).

	A route whose render branches on the caller -- `isOwner`,
	`isOwnerOrAdmin`, `isAmbientContractor` -- is several screens behind
	one path, and the fixture above describes exactly one of them. Whichever
	branch its own session selects was swept and dragged; the others were
	swept and dragged never, because neither half of the check had any way
	to know they existed. That is the sibling of ADR-0025's row-set rule for
	the axis a row set cannot reach: these states differ by who is looking,
	not by what the data holds.

	A variant is read through `toSweptFixtures` and nowhere else. It is a
	shallow override of the fixture it sits on, so a variant that restates
	`pageData` restates it whole -- the session is what varies, and a
	partial merge of two `pageData` objects would leave a variant looking
	like it inherited a key it had actually dropped.
	*/
	readonly variants?: readonly RouteVariant<RouteParameters>[];
}

/**
One other session the route above renders differently under, named as its
own screen.

`name` is required and is the whole name, not a suffix: the check titles
its `it` with it and the drag surface keys its `Select` on it, so two
branches sharing a name collide in both. Name what varies -- "The Stripe
Connect settings screen, as a Doula" -- so a failure sentence says which
branch broke.

There is no `component`: a variant is the same route seen by a different
caller. A different component is a different route, and it declares its
own fixture.
*/
export interface RouteVariant<
	RouteParameters extends Record<string, string> = Record<string, string>
> {
	/**
	How this branch is named in a failure sentence and on the drag
	surface's picker -- unique across every subject, route and component
	alike.
	*/
	readonly name: string;
	/**
	`page.params`, when this branch is reached at other ids.
	*/
	readonly params?: Readonly<RouteParameters>;
	/**
	`page.url`, when this branch is reached at another address.
	*/
	readonly url?: string;
	/**
	The route's own `load` output, restated whole.
	*/
	readonly props?: Readonly<Record<string, unknown>>;
	/** This branch's answers to the fetches it makes -- which are not the
	other branch's, since what a route asks for is often what it is
	allowed to ask for. */
	readonly respond?: (path: string) => Response;
	/** `page.data`, restated whole. This is where a Practice route's
	session lives, so it is the field nearly every variant sets. */
	readonly pageData?: Readonly<Record<string, unknown>>;
	/** The level-1 heading, when this branch's screen is titled
	differently. */
	readonly readyText?: string;
}

/**
Every complete fixture one declared fixture stands for: itself, then one
per variant.

This sits beside `toPageState` and `toApiResponder` for the reason that
comment gives, and it is the reason `variants` could be added at all. A
field on `RouteFixture` that each half walked for itself reaches whichever
half somebody remembered, and the two then measure and show different
screens -- `route-continuum.svelte.spec.ts` would sweep three branches
while the drag surface offered one. Neither half touches `variants`; both
call this.

The realized fixtures carry no `variants` of their own, so there is
nothing left for a caller to expand a second time, and a variant of a
variant is not a shape this supports. A fixture with no variants is
returned as itself, unchanged and identical.
*/
export function toSweptFixtures<RouteParameters extends Record<string, string>>(
	fixture: RouteFixture<RouteParameters>
): readonly RouteFixture<RouteParameters>[] {
	if (!fixture.variants) return [fixture];
	const { variants, ...base } = fixture;
	return [base, ...variants.map((variant) => ({ ...base, ...variant }))];
}
