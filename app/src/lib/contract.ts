/**
 * A Client's Draft/Sent/Signed/Voided Contract for an Engagement -- a
 * snapshot of the Practice's Contract Template prose taken at creation,
 * plus whatever merge field values Staff has filled in. This module holds
 * the load/create/save orchestration for the Engagement view's Contract
 * section, decoupled from SvelteKit and the DOM so it can be
 * unit-tested directly -- mirrors planInstance.ts.
 */
import { MERGE_FIELDS } from './contractTemplate.js';

export interface Contract {
	engagementId: string;
	status: string;
	prose: string;
	mergeFields: string[];
	values: Record<string, string>;
}

/** A minimal fetch-shaped function, injected rather than imported -- see
 * planInstance.ts's Fetcher for why. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

/** Maps a merge field key (e.g. "client_name") to its display label (e.g.
 * "Client name"), sourced from contractTemplate.ts's MERGE_FIELDS list.
 * Falls back to the raw key for a placeholder not in that list -- prose
 * is free text, so a Practice Owner can put in an ad hoc token. */
export function mergeFieldLabel(key: string): string {
	const entry = MERGE_FIELDS.find((f) => f.token === `{{${key}}}`);
	return entry?.label ?? key;
}

/** Matches a {{merge_field_key}} placeholder in Contract prose -- mirrors
 * the Go BFF's mergeFieldPattern (api/internal/contracts/contract.go). */
const mergeFieldPattern = /\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g;

/** Substitutes every {{merge_field_key}} placeholder in prose with its
 * filled value, leaving an unfilled placeholder blank -- the read-only
 * Client-portal Contract view's "filled Contract text" is this, not the
 * raw prose. */
export function fillProse(prose: string, values: Record<string, string>): string {
	return prose.replaceAll(mergeFieldPattern, (_match, key: string) => values[key] ?? '');
}

function contractPath(practiceId: string, engagementId: string): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}/contract`;
}

function clientContractPath(engagementId: string): string {
	return `/api/portal/engagements/${engagementId}/contract`;
}

/** Loads the sent/signed/voided Contract for engagementId from the
 * Client-portal route, or null if none has been sent yet (a 404 from
 * ClientGetContractHandler -- a Draft Contract 404s the same way, since
 * it's unreachable from this role) -- mirrors planInstance.ts's
 * loadClientBirthPlan, distinguishing "not yet sent" (null) from the
 * caller's own "not yet loaded" (undefined) state. Throws with the
 * response body text on any other non-2xx response. */
export async function loadClientContract(fetcher: Fetcher, engagementId: string): Promise<Contract | null> {
	const response = await fetcher(clientContractPath(engagementId));
	if (response.status === 404) {
		// eslint-disable-next-line unicorn/no-null
		return null;
	}
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Loads the Contract for engagementId, or undefined if none has been
 * created yet (a 404 from GetContractHandler) -- callers use that to show
 * a "Create" control instead of the fill-out form. Throws with the
 * response body text on any other non-2xx response. */
export async function loadContract(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<Contract | undefined> {
	const response = await fetcher(contractPath(practiceId, engagementId));
	if (response.status === 404) {
		return undefined;
	}
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Creates the Draft Contract for engagementId, snapshotting the
 * Practice's current Contract Template prose server-side. Throws with the
 * response body text on a non-2xx response (e.g. the Practice has no
 * template yet, or a Contract already exists for this Engagement). */
export async function createContract(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<Contract> {
	const response = await fetcher(contractPath(practiceId, engagementId), { method: 'POST' });
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Replaces the full merge field Values map of the Contract for
 * engagementId -- PUT is a full replacement, not a merge, so callers must
 * pass every value they want kept, not just the one that changed. Only
 * succeeds while the Contract is still a Draft. */
export async function saveContractValues(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	values: Record<string, string>
): Promise<Contract> {
	const response = await fetcher(contractPath(practiceId, engagementId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ values })
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Transitions the Contract for engagementId from Draft to Sent --
 * one-way, and only while it's still a Draft (a non-Draft Contract 409s).
 * Triggers a content-free push notification to the Client server-side.
 * Throws with the response body text on a non-2xx response. */
export async function sendContract(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<Contract> {
	const response = await fetcher(`${contractPath(practiceId, engagementId)}/send`, { method: 'POST' });
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Sets the value for a merge field key within values, returning a new
 * object (values is never mutated) -- mirrors planInstance.ts's
 * setAnswer, minus the polymorphic-type handling a Contract's merge
 * fields don't need (every merge field is plain text). */
export function setMergeFieldValue(
	values: Record<string, string>,
	key: string,
	value: string
): Record<string, string> {
	return { ...values, [key]: value };
}
