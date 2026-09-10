/**
 * A Client's Draft/Sent/Signed/Voided Contract for an Engagement -- a
 * snapshot of the Practice's Contract Template prose taken at creation,
 * plus whatever merge field values Staff has filled in. This module holds
 * the load/create/save orchestration for the Engagement view's Contract
 * section, decoupled from SvelteKit and the DOM so it can be
 * unit-tested directly -- mirrors planInstance.ts.
 */
import type { Fetcher } from './fetcher.js';
import type { CursorPage } from './paginatedList.svelte.js';

import { MERGE_FIELDS } from './contractTemplate.js';
import { apiErrorMessage } from './apiErrorMessage.js';
import { fetchBlob } from './blobDownload.js';

export interface Contract {
	engagementId: string;
	status: string;
	prose: string;
	mergeFields: string[];
	values: Record<string, string>;
	/** When amountCents last moved after creation -- an Owner/Admin
	 * override, or a rate-driven reprice (#968) -- so a reader can see
	 * the price changed and when without hunting the activity ledger for
	 * it. Absent while the Contract still carries its as-created amount,
	 * and always absent for a contractor (the Go BFF withholds it the
	 * same way it withholds the "price" merge field value itself). */
	amountChangedAt?: string;
	/** Every void request against this Contract, newest first, regardless
	 * of who asked or who is reading (#971) -- unlike `values`, a
	 * request's reason and outcome carry no price, so nothing here needs
	 * ADR-0008's contractor redaction. Absent for a Contract nobody has
	 * ever asked to void, the common case. */
	voidRequests?: VoidRequestSummary[];
	/** Whether a signed PDF exists for this Engagement (#1119) -- the one
	 * fact a screen gates its download control on, and the same fact the
	 * signed-PDF endpoints key on, so the control and the route behind it
	 * cannot disagree. Not a reading of `status`: a voided Contract whose
	 * PDF was deliberately preserved (#299) reports `true`, and a Draft
	 * or an unsigned Sent Contract reports `false`. Always present -- the
	 * Go BFF sends it on every Contract-shaped response. */
	hasSignedPdf: boolean;
}

/** One void request against a Contract (#971): who asked, when, why, and
 * -- once an Owner or an Admin has decided -- whether it was granted (a
 * void, via the existing Void endpoint) or declined with a reason of its
 * own. `status` is `'open'` while nobody has decided yet, `'voided'`
 * once granted, `'declined'` once refused; the two outcomes are how the
 * person who asked tells which happened. Mirrors the Go BFF's
 * VoidRequestSummary (api/internal/contracts/voidrequest.go). */
export interface VoidRequestSummary {
	id: string;
	requestedBy: string;
	requestedByName: string;
	reason: string;
	status: string;
	declineReason?: string;
	decidedBy?: string;
	decidedAt?: string;
	createdAt: string;
}

/** The "price" merge field key is reserved: the Go BFF resolves it from
 * the Practice's rate card at creation time and never accepts it back
 * through PutContractHandler (#967's AC -- "never a value a person fills
 * in on the Contract form"). ContractForm renders every other merge
 * field as an editable input; the filled prose (ContractView, fillProse)
 * still shows price wherever the Practice's own template asks for it. */
const RESERVED_MERGE_FIELD_KEY = 'price';

/** mergeFields with the reserved price key removed, for ContractForm's
 * own editable-field list -- price has nothing to edit. */
export function editableMergeFields(mergeFields: string[]): string[] {
	return mergeFields.filter((key) => key !== RESERVED_MERGE_FIELD_KEY);
}

/** values with the reserved price key removed, for a PUT request body --
 * PutContractHandler refuses a request that carries price at all (#967),
 * so a caller building a full-replacement Values map from a Contract
 * already loaded (whose values always carry price resolved, per
 * withResolvedPrice) must strip it back out before saving, or the whole
 * save 400s. */
export function editableValues(values: Record<string, string>): Record<string, string> {
	return Object.fromEntries(Object.entries(values).filter(([key]) => key !== RESERVED_MERGE_FIELD_KEY));
}

/** Merge field keys among mergeFields whose entry in values is absent,
 * empty, or whitespace-only -- mirrors the Go BFF's own Send precondition
 * (contracts.missingMergeFieldKeys, #258). Used to block the Staff-side
 * Send control before a round trip and to name which fields still need
 * filling in. */
export function missingMergeFieldKeys(mergeFields: string[], values: Record<string, string>): string[] {
	return mergeFields.filter((key) => (values[key] ?? '').trim() === '');
}

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
		throw new Error(await apiErrorMessage(response));
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
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Downloads the Signed PDF for engagementId's Contract from the
 * Client-portal route (#302) -- a Blob, not JSON, so the caller turns it
 * into an object URL and drives the browser's own download, mirroring
 * engagementDetail.ts's downloadAttachment. Throws with the response
 * body text on a non-2xx response (e.g. not yet signed). */
export async function downloadClientSignedContractPdf(fetcher: Fetcher, engagementId: string): Promise<Blob> {
	return fetchBlob(fetcher, `${clientContractPath(engagementId)}/pdf`);
}

/** Downloads the Signed PDF for engagementId's Contract from the Practice
 * route (#302) -- Owner, Admin, and an employed Doula per ADR-0008's
 * money row as amended by #282, refused for a contractor. Mirrors
 * downloadClientSignedContractPdf above. Throws with the response body
 * text on a non-2xx response. */
export async function downloadSignedContractPdf(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<Blob> {
	return fetchBlob(fetcher, `${contractPath(practiceId, engagementId)}/pdf`);
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
		throw new Error(await apiErrorMessage(response));
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
		throw new Error(await apiErrorMessage(response));
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
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Signs the sent Contract for engagementId -- transitions it to signed,
 * one-way, and only while it's still sent (a non-sent Contract 409s).
 * fullLegalName and isAttestation are the only fields the caller supplies;
 * the BFF derives signed_at and the signer's IP itself. Throws with the
 * response body text on a non-2xx response. */
export async function signContract(
	fetcher: Fetcher,
	engagementId: string,
	fullLegalName: string,
	isAttestation: boolean
): Promise<Contract> {
	const response = await fetcher(`${clientContractPath(engagementId)}/sign`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ fullLegalName, attestation: isAttestation })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Transitions the Contract for engagementId from Signed to Voided --
 * one-way and terminal, and only while it's still signed (a non-signed
 * Contract 409s). Staff create a fresh Draft afterward via createContract
 * to capture updated terms; there is no amendment/addendum entity. Throws
 * with the response body text on a non-2xx response. */
export async function voidContract(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<Contract> {
	const response = await fetcher(`${contractPath(practiceId, engagementId)}/void`, { method: 'POST' });
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Asks for engagementId's Signed Contract to be voided (#971) -- what a
 * Doula does instead of voiding it herself, which #970 refuses her.
 * Only succeeds while the Contract is signed, and only once per
 * requester at a time (a second ask while the first is still open
 * 409s). Throws with the response body text on a non-2xx response. */
export async function requestContractVoid(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	reason: string
): Promise<Contract> {
	const response = await fetcher(`${contractPath(practiceId, engagementId)}/void-request`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ reason })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Declines one open void request against engagementId's Contract (#971)
 * -- an Owner or an Admin's other answer to a Doula's ask, distinct from
 * granting it (voidContract above). Only succeeds while requestId names
 * a still-open request. Throws with the response body text on a non-2xx
 * response. */
export async function declineContractVoidRequest(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	requestId: string,
	reason: string
): Promise<Contract> {
	const response = await fetcher(
		`${contractPath(practiceId, engagementId)}/void-request/${requestId}/decline`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason })
		}
	);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
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

/** One row of the Practice-wide "Contracts awaiting signature" list
 * (#273) -- a Draft or Sent Contract, the Engagement it belongs to and
 * the Client whose signature is outstanding. Mirrors the Go BFF's
 * AwaitingItem (api/internal/contracts/awaiting.go). `createdAt` is the
 * Contract's own creation instant, which is also the moment it started
 * waiting -- the endpoint's own ordering already treats it that way
 * ("the Contract that has been waiting longest ... belongs at the
 * top"), so this reuses it rather than tracking a separate per-status
 * timestamp the BFF does not keep. */
export interface AwaitingContract {
	engagementId: string;
	contractId: string;
	clientId: string;
	clientName: string;
	status: string;
	createdAt: string;
}

/** The Practice-wide "Contracts awaiting signature" list's path --
 * mirrors practiceInvoicesPath in invoice.ts. */
export function practiceAwaitingContractsPath(practiceId: string, cursor?: string): string {
	const path = `/api/practices/${practiceId}/contracts/awaiting-signature`;
	return cursor ? `${path}?cursor=${encodeURIComponent(cursor)}` : path;
}

/** Loads one page of every Contract at the Practice that is Draft or
 * Sent, oldest first -- mirrors loadWaitingOnReplyPage in
 * practiceLanding.ts. Throws with the response body text on a non-2xx
 * response; the route's `load` maps status codes to SvelteKit errors
 * before calling this, so a throw here is only ever an unexpected
 * failure. */
export async function loadPracticeAwaitingContracts(
	fetcher: Fetcher,
	practiceId: string,
	cursor?: string
): Promise<CursorPage<AwaitingContract>> {
	const response = await fetcher(practiceAwaitingContractsPath(practiceId, cursor));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** The two statuses this list ever shows, in the words a person reads --
 * "draft" is work the Practice still owes, "sent" is work the Client
 * still owes. An unknown status falls through to itself rather than to
 * a blank, mirroring invoiceStatusLabel. */
const awaitingContractStatusLabels: Record<string, string> = {
	draft: 'Draft',
	sent: 'Sent'
};

export function awaitingContractStatusLabel(status: string): string {
	return awaitingContractStatusLabels[status] ?? status;
}

/** One row of the Practice-wide "void requests waiting on you" roll-up
 * (#971) -- mirrors the Go BFF's VoidRequestAwaitingItem
 * (api/internal/contracts/void_requests_awaiting.go). Owner/Admin only,
 * the same reach Void itself carries. */
export interface AwaitingVoidRequest {
	requestId: string;
	engagementId: string;
	clientId: string;
	clientName: string;
	requestedByName: string;
	reason: string;
	createdAt: string;
}

/** The Practice-wide "void requests awaiting" list's path -- mirrors
 * practiceAwaitingContractsPath above. */
export function practiceAwaitingVoidRequestsPath(practiceId: string, cursor?: string): string {
	const path = `/api/practices/${practiceId}/contracts/void-requests`;
	return cursor ? `${path}?cursor=${encodeURIComponent(cursor)}` : path;
}

/** Loads one page of every open void request at the Practice, oldest
 * first -- mirrors loadPracticeAwaitingContracts above. Throws with the
 * response body text on a non-2xx response. */
export async function loadPracticeAwaitingVoidRequests(
	fetcher: Fetcher,
	practiceId: string,
	cursor?: string
): Promise<CursorPage<AwaitingVoidRequest>> {
	const response = await fetcher(practiceAwaitingVoidRequestsPath(practiceId, cursor));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}
