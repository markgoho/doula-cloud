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
 * route's own spec asserts on can become one object rather than two that
 * drift.
 *
 * The content itself is hostile, never polite (ADR-0025, #537): a route's
 * values come from a Practice's own typing rather than from a file, so a
 * fixture holding representative content measures a screen nobody will
 * ever see. #521's approval screen fits at 320px on polite content and is
 * 93px past its edge on one pasted URL.
 */
import type { Component } from 'svelte';

export interface RouteFixture {
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
	readonly params: Readonly<Record<string, string>>;
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
	The screen's own level-1 heading, matched by role. It must be on the page before a measurement is taken. A route
	that loads in `onMount` renders a Skeleton first, and sweeping that
	measures the loading state rather than the screen -- an honest wait, not
	a timeout.
	*/
	readonly readyText: string;
}
