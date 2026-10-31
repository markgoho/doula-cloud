import { resolve } from '$app/paths';
import { withReturnTo } from './returnTo.js';

/**
The one place the "your session ended under you" convention is spelled.

A session can end without the person doing anything: a cookie expires, a
Membership is revoked, or she removes the second factor her session was
minted against. Whatever ended it, she lands on a login form, and that
form has to say why -- otherwise she is looking at a bare sign-in box
with no account of the screen that vanished under her.

The way it says so is a flag on the login address. Nothing here is
clever; the point is that the writers and the readers stop agreeing by
coincidence. Before #1131 the flag was typed by hand in fourteen places
-- twelve redirects building the query string, two screens matching it
back -- so renaming it, or spelling it `sessionended` in one new
redirect, was a silent miss rather than a type error, and the notice
would simply not render.

`sessionEnded.usage.spec.ts` keeps it that way: no file in `app/src` but
this one may name the flag.
*/
const PARAM = 'sessionEnded';

/*
 * The flag is a word, not a bare presence, so a URL carrying the param
 * with any other value reads as an ordinary visit rather than a
 * half-recognized one.
 */
const FLAG_VALUE = 'true';

/**
Everything this module needs of an address: the ability to be asked for
one query parameter. A real `URL` satisfies it, and so does what both
login screens actually have to hand -- `page.url` through
`#lib/appState.svelte.js`, typed as `$app/state`'s own page URL, whose
`searchParams` is a `ReadonlyURLSearchParams` and is therefore not
assignable to `URL`.
*/
interface AddressQuery {
	readonly searchParams: { get(name: string): string | null };
}

/**
Whether url is a login address entered after a session ended, as opposed
to an ordinary visit to the same screen. Takes the whole address rather
than its `searchParams` so a caller passes what it already has -- both
login screens read `page.url`.
*/
export function didSessionEnd(url: AddressQuery): boolean {
	return url.searchParams.get(PARAM) === FLAG_VALUE;
}

/*
 * Resolved per population rather than from a union: `resolve` is
 * overloaded per route, and a union argument stops matching any single
 * overload. That is also why these are two functions rather than one
 * taking a population -- the route id has to be a literal at the call.
 */

/**
The Staff login address to send someone to when her Staff session ended.
*/
export function staffLoginAfterSessionEnded(): string {
	return `${resolve('/(signed-out)/login')}?${PARAM}=${FLAG_VALUE}`;
}

/*
 * #2259: a refusal alone cannot say a session ended. A saved address
 * opened on a device that never signed in is refused the same way, and
 * the BFF cannot tell the two apart once the browser has dropped an
 * expired cookie. What can tell is this document: it held a session if a
 * credentialed request here was answered. Reset by sign-out.
 */
const held = { inThisDocument: false };

/**
Records that this document is talking to the BFF with a live session.
*/
export function noteSessionHeld(): void {
	held.inThisDocument = true;
}

/**
Records that she signed out, so a refusal after it is not read as a
session ending under her.
*/
export function forgetSessionHeld(): void {
	held.inThisDocument = false;
}

/**
The Staff login address for a request refused for want of a session:
back to returnTo after sign-in, and saying the session ended only when
this document held one.
*/
export function staffLoginAfterRefusal(returnTo: string): string {
	const login = held.inThisDocument ? staffLoginAfterSessionEnded() : resolve('/(signed-out)/login');
	return withReturnTo(login, returnTo);
}

/**
The Client-portal login address for a request refused for want of a
session, saying the session ended only when this document held one.
*/
export function portalLoginAfterRefusal(): string {
	const login = resolve('/portal/(signed-out)/login');
	return held.inThisDocument ? `${login}?${PARAM}=${FLAG_VALUE}` : login;
}
