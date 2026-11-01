/**
Where a screen sends her once it is done with her: the address she asked
for before sign-in (#2259) or before second-factor enrollment (#606).

The value comes off the address bar, so anyone can write one. The reader is
the boundary: it hands back a path on this app's own origin or nothing,
which is what keeps a crafted link from sending her off-site after she has
typed her password.
*/
const PARAM = 'returnTo';

interface Address {
	readonly origin: string;
	readonly pathname: string;
	readonly searchParams: { get(name: string): string | null };
}

/**
The path, query and fragment url names to come back to, or `undefined`
when it names none, names another origin (`//host`, `/\host`, a scheme),
or names the screen reading it.
*/
export function readReturnTo(url: Address): string | undefined {
	const target = sameOriginPath(url.searchParams.get(PARAM), url.origin);
	if (target === undefined || new URL(target, url.origin).pathname === url.pathname) return undefined;
	return target;
}

/**
target as a path, query and fragment on origin, or `undefined` when it is
absent or names another origin. For an address that did not come off the
address bar, such as the one the BFF hands back from a magic link (#2496).
*/
export function sameOriginPath(target: string | null | undefined, origin: string): string | undefined {
	if (!target?.startsWith('/')) return undefined;
	const resolved = new URL(target, origin);
	if (resolved.origin !== origin) return undefined;
	return `${resolved.pathname}${resolved.search}${resolved.hash}`;
}

/**
address with `returnTo` set to path, kept beside any query address already has.
*/
export function withReturnTo(address: string, path: string): string {
	const url = new URL(address, 'https://origin.invalid');
	url.searchParams.set(PARAM, path);
	return `${url.pathname}${url.search}`;
}
