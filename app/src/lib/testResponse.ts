/**
Builds a fake Response for a spec's mocked fetcher. Vitest runs specs in
Node, with no real HTTP response to hand back -- this builds just enough
of a Response's shape (ok/status/text/json) for #lib/api.ts's callers to
read. status defaults to 200; pass a non-2xx status for a refusal.
*/
export function jsonResponse(body: unknown, status = 200): Response {
	return {
		ok: status >= 200 && status < 300,
		status,
		text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
		json: () => Promise.resolve(body)
	} as Response;
}
