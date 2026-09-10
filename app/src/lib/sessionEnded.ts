import { resolve } from '$app/paths';

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
const SET = 'true';

/**
Whether url is a login address entered after a session ended, as opposed
to an ordinary visit to the same screen. Takes the whole `URL` rather
than its `searchParams` so a caller passes what it already has -- both
login screens read `page.url`.
*/
export function sessionEndedFrom(url: URL): boolean {
	return url.searchParams.get(PARAM) === SET;
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
	return `${resolve('/(signed-out)/login')}?${PARAM}=${SET}`;
}

/**
The Client-portal login address to send someone to when her portal
session ended.
*/
export function portalLoginAfterSessionEnded(): string {
	return `${resolve('/portal/(signed-out)/login')}?${PARAM}=${SET}`;
}
