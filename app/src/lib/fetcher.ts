/**
 * A minimal fetch-shaped function, injected rather than imported. Every
 * `lib/*.ts` domain module (client.ts, contract.ts, invoice.ts, and
 * seventeen more, #840) took its own copy of this exact type so its
 * load/save functions could be unit-tested without mocking the global
 * `fetch` or SvelteKit's `$app` modules -- a route wires this to
 * `#lib/api.js`'s `apiFetchWithSession` (or, for the two best-effort
 * calls that must not redirect on a 401, `apiFetch`); a test hands a
 * `vi.fn()`. Declared once here so a route choosing which domain module
 * to call is never also choosing among twenty identical types.
 */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;
