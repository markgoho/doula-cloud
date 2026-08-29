/**
 * A Practice's Clients: the free-standing intake record (ADR-0017) that
 * predates any Engagement. This module holds the list/create
 * orchestration for the Clients screens, decoupled from SvelteKit and
 * the DOM so it can be unit-tested directly -- mirrors invoice.ts.
 */

/** One row of the Clients list, Client-shaped: one row per Client, never
 * one per Client+Engagement pair (ADR-0017) -- mirrors the Go BFF's
 * client.ListItem (api/internal/client/list.go). */
export interface ClientListItem {
	clientId: string;
	name: string;
	email: string;
	hasWork: boolean;
	portalInviteStatus?: string;
}

/** The body of an Add-a-Client submission -- the two fields the screen
 * collects today; GivenName is the only fact ADR-0017 requires. */
export interface NewClient {
	givenName: string;
	email: string;
}

/** A minimal fetch-shaped function, injected rather than imported -- see
 * invoice.ts's Fetcher for why. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function clientsPath(practiceId: string): string {
	return `/api/practices/${practiceId}/clients`;
}

/** Loads a Practice's Clients, Client-shaped -- ListHandler's default
 * "Clients with work" filter (ADR-0017), narrowed further for a
 * contractor Doula server-side. Throws with the response body text on a
 * non-2xx response, mirroring loadBalance's error-surfacing convention. */
export async function loadClients(fetcher: Fetcher, practiceId: string): Promise<ClientListItem[]> {
	const response = await fetcher(clientsPath(practiceId));
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Creates a free-standing Client: no Engagement, no Credit spent.
 * Throws with the response body text on a non-2xx response -- e.g. a
 * contractor Doula's refusal, or ADR-0017's possible-duplicate match
 * conflict, whose JSON body this does not decode into anything more than
 * its raw text (the screen has no override flow yet to act on it). */
export async function createClient(fetcher: Fetcher, practiceId: string, client: NewClient): Promise<void> {
	const response = await fetcher(clientsPath(practiceId), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(client)
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
}
