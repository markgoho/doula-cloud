/**
 * A Practice's Clients: the free-standing intake record (ADR-0017) that
 * predates any Engagement. This module holds the list/create/edit
 * orchestration for the Clients screens, decoupled from SvelteKit and
 * the DOM so it can be unit-tested directly -- mirrors invoice.ts.
 */

import type { ClientRecord, EngagementSummary } from './clientDetail.js';

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

/** One page of the Clients list -- the cursor-pagination envelope from
 * docs/api-design.md section 4, mirroring client.ListResponse (#446). */
export interface ClientListPage {
	items: ClientListItem[];
	nextCursor?: string;
	hasMore: boolean;
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

/** Loads one page of a Practice's Clients, Client-shaped -- ListHandler's
 * default "Clients with work" filter (ADR-0017), narrowed further for a
 * contractor Doula server-side. `cursor` is undefined for the first page.
 * Throws with the response body text on a non-2xx response, mirroring
 * loadBalance's error-surfacing convention. */
export async function loadClients(
	fetcher: Fetcher,
	practiceId: string,
	cursor?: string
): Promise<ClientListPage> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`${clientsPath(practiceId)}${query}`);
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

/** The full structural core an edit submits -- every column but `id`,
 * which the endpoint takes from the path rather than the body. Includes
 * `fieldValues` so a save round-trips her Practice-defined values
 * unchanged rather than wiping them (see ClientRecord.fieldValues). */
export type ClientEditFields = Omit<ClientRecord, 'id'>;

/** One Client ADR-0017's match query turned up against an edit's values --
 * her record plus her Engagement history, unrestricted inside the
 * Practice -- mirrors the Go BFF's client.Match. */
export interface ClientMatch extends ClientRecord {
	engagements: EngagementSummary[];
}

/** The outcome of a save attempt: either it went through, or the match
 * query refused it and named who it matched. A discriminated union
 * rather than a thrown error, because a conflict is an expected outcome
 * this screen has a real next step for -- the override prompt -- not a
 * failure. */
export type EditClientResult =
	| { conflict: false; record: ClientRecord }
	| { conflict: true; matches: ClientMatch[] };

function clientPath(practiceId: string, clientId: string): string {
	return `${clientsPath(practiceId)}/${clientId}`;
}

/** Saves an edit to an existing Client -- a full-object PUT
 * (api/internal/client/edit.go). `shouldOverride`, when true, is
 * ADR-0017's single deliberate "No, a different person" act: send it
 * only after the caller has shown the reader the match a prior call
 * returned and she chose to proceed anyway. A 409 decodes into
 * `{ conflict: true, matches }` rather than throwing -- the caller's
 * expected path, not an exceptional one. Any other non-2xx response
 * throws with the response body text, so a refusal that is not a match
 * conflict (a validation failure, a permission refusal, a dropped
 * connection) is never a silent failure. */
export async function editClient(
	fetcher: Fetcher,
	practiceId: string,
	clientId: string,
	fields: ClientEditFields,
	shouldOverride: boolean
): Promise<EditClientResult> {
	const response = await fetcher(clientPath(practiceId, clientId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ ...fields, override: shouldOverride })
	});
	if (response.status === 409) {
		const body: { matches: ClientMatch[] } = await response.json();
		return { conflict: true, matches: body.matches };
	}
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return { conflict: false, record: await response.json() };
}
