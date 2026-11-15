/**
 * A Client's Draft/Sent/Signed/Voided Contract for an Engagement -- a
 * snapshot of the Practice's Contract Template prose taken at creation,
 * plus whatever merge field values Staff has filled in. This module holds
 * the load/create/save orchestration for the Engagement view's Contract
 * section, decoupled from SvelteKit and the DOM so it can be
 * unit-tested directly -- mirrors planInstance.ts.
 */
import type { Fetcher } from './fetcher.js';
import type { Stamp } from './stamp.js';

import { DATE_MERGE_FIELDS, MERGE_FIELDS } from './contractTemplate.js';
import { dueParts, type DueParts } from './client.js';
import { formatCalendarDay } from './dates.js';
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
	/** When the Contract was sent and by whom, from the Activity ledger's
	 * own record of the send (#2104). Absent for a Draft, and on a
	 * Client-portal read. */
	sent?: Stamp;
	/** When the Contract was signed and the full legal name typed to sign
	 * it (#2104). A voided Contract keeps it. */
	signed?: Stamp;
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

const STAFF_STATUS_TEXT: Record<string, string> = {
	draft: 'Draft, not sent yet',
	sent: 'Sent, waiting for the Client to sign',
	signed: 'Signed',
	voided: 'Voided, no longer active'
};

/** A Contract's status as the Staff Engagement page's Status row says it
 * (#2004). The Client reads `clientRegister.ts`'s own words, never these.
 * An unknown status reads as itself, so a new one is visible, not blank. */
export function contractStatusText(status: string): string {
	return STAFF_STATUS_TEXT[status] ?? status;
}

/** Matches a {{merge_field_key}} placeholder in Contract prose -- mirrors
 * the Go BFF's mergeFieldPattern (api/internal/contracts/contract.go). */
const mergeFieldPattern = /\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g;

/** Substitutes every {{merge_field_key}} placeholder in prose with its
 * filled value, leaving an unfilled placeholder blank -- the read-only
 * Client-portal Contract view's "filled Contract text" is this, not the
 * raw prose. */
export function fillProse(prose: string, values: Record<string, string>): string {
	return prose.replaceAll(mergeFieldPattern, (_match, key: string) => fillValue(key, values[key] ?? ''));
}

const isoDay = /^\d{4}-\d{2}-\d{2}$/;

/** What one merge field prints as -- mirrors the BFF's fillValue
 * (api/internal/contracts/pdf.go) so the signed PDF says the same: a date
 * field holding a real YYYY-MM-DD day reads as a calendar day ("Oct 27,
 * 2026", #2222), and anything else prints as it was entered. */
function fillValue(key: string, value: string): string {
	if (!DATE_MERGE_FIELDS.has(key) || !isoDay.test(value)) return value;
	const [year, month, day] = value.split('-').map(Number);
	const date = new Date(year!, month! - 1, day!);
	if (date.getMonth() !== month! - 1 || date.getDate() !== day) return value;
	return formatCalendarDay(value);
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

async function postReason(fetcher: Fetcher, path: string, reason: string): Promise<Contract> {
	const response = await fetcher(path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ reason })
	});
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
	return postReason(fetcher, `${contractPath(practiceId, engagementId)}/void-request`, reason);
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
	return postReason(
		fetcher,
		`${contractPath(practiceId, engagementId)}/void-request/${requestId}/decline`,
		reason
	);
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

export type ContractsView = 'not-signed' | 'draft' | 'sent' | 'signed' | 'voided' | 'all';
export type ContractOrder = '' | 'oldest' | 'newest' | 'client';
type NamedContractOrder = Exclude<ContractOrder, ''>;

export interface ContractListItem {
	contractId: string;
	engagementId: string;
	clientId: string;
	clientName: string;
	engagementKind: 'birth' | 'postpartum';
	status: 'draft' | 'sent' | 'signed' | 'voided';
	steppedOn: string;
	daysWaiting?: number;
	sentByName?: string;
	dueDate?: string;
	pregnancyEndedOn?: string;
	birthOutcome?: string;
	doulaNames?: string[];
	amountCents?: number;
	hasSignedPdf: boolean;
	voidRequested: boolean;
}

export interface ContractsPage {
	items: ContractListItem[];
	nextCursor?: string;
	hasMore: boolean;
	total: number;
}

export interface ContractsListData extends ContractsPage {
	view: ContractsView;
	order: ContractOrder;
	isMine: boolean;
	doula: string;
	now: Date;
}

export interface ContractsQuery {
	view: ContractsView;
	order: ContractOrder;
	isMine: boolean;
	doula: string;
	search?: string;
}

export interface AwaitingVoidRequest {
	requestId: string;
	contractId: string;
	engagementId: string;
	clientId: string;
	clientName: string;
	requestedByName: string;
	reason: string;
	daysAgo: number;
	createdAt: string;
}

export interface VoidRequestsPage {
	items: AwaitingVoidRequest[];
	nextCursor?: string;
	hasMore: boolean;
	total: number;
}

export const CONTRACTS_READ = 'app:contracts';

const VIEW_NAMES: readonly ContractsView[] = ['draft', 'sent', 'signed', 'voided', 'all'];

export function contractsViewOf(search: Pick<URLSearchParams, 'get'>): ContractsView {
	const shown = search.get('show');
	return VIEW_NAMES.find((name) => name === shown) ?? 'not-signed';
}

const VIEW_ORDER: Record<ContractsView, 'oldest' | 'newest'> = {
	'not-signed': 'oldest',
	draft: 'oldest',
	sent: 'oldest',
	signed: 'newest',
	voided: 'newest',
	all: 'newest'
};

export function contractViewOrder(view: ContractsView): 'oldest' | 'newest' {
	return VIEW_ORDER[view];
}

export function orderForView(order: ContractOrder, view: ContractsView): ContractOrder {
	return order === VIEW_ORDER[view] ? '' : order;
}

const ORDER_NAMES = new Set(['oldest', 'newest', 'client']);

export function contractOrderOf(search: Pick<URLSearchParams, 'get'>, view: ContractsView): ContractOrder {
	const named = search.get('order');
	return ORDER_NAMES.has(named ?? '') ? orderForView(named as ContractOrder, view) : '';
}

const ORDER_LABELS: Record<ContractsView, { oldest: string; newest: string }> = {
	'not-signed': { oldest: 'Oldest first', newest: 'Newest first' },
	draft: { oldest: 'Made, oldest first', newest: 'Made, newest first' },
	sent: { oldest: 'Longest wait first', newest: 'Shortest wait first' },
	signed: { oldest: 'Signed, oldest first', newest: 'Signed, newest first' },
	voided: { oldest: 'Voided, oldest first', newest: 'Voided, newest first' },
	all: { oldest: 'Oldest first', newest: 'Newest first' }
};

export function contractOrderOptions(view: ContractsView): { value: ContractOrder; label: string }[] {
	const own = VIEW_ORDER[view];
	const other = own === 'oldest' ? 'newest' : 'oldest';
	return [
		{ value: '', label: ORDER_LABELS[view][own] },
		{ value: other, label: ORDER_LABELS[view][other] },
		{ value: 'client', label: 'Client, A to Z' }
	];
}

export function contractOrderedBy(
	order: ContractOrder,
	view: ContractsView
): { column: string; direction: 'ascending' | 'descending' } {
	const named: NamedContractOrder = order || VIEW_ORDER[view];
	if (named === 'client') return { column: 'Client', direction: 'ascending' };
	return { column: 'Status', direction: named === 'oldest' ? 'ascending' : 'descending' };
}

export interface ContractsAddress {
	view: ContractsView;
	order: ContractOrder;
	isMine?: boolean;
	doula?: string;
}

export function contractsQueryString({ view, order, isMine = false, doula = '' }: ContractsAddress): string {
	const query = new URLSearchParams();
	if (view !== 'not-signed') query.set('show', view);
	if (isMine) query.set('mine', 'true');
	if (doula) query.set('doula', doula);
	const own = orderForView(order, view);
	if (own) query.set('order', own);
	const text = query.toString();
	return text ? `?${text}` : '';
}

function contractsBase(practiceId: string): string {
	return `/api/practices/${practiceId}/contracts`;
}

export function practiceContractsPath(practiceId: string, query: ContractsQuery, cursor?: string): string {
	const search = new URLSearchParams();
	if (query.view !== 'not-signed') search.set('view', query.view);
	if (query.order) search.set('order', query.order);
	if (query.isMine) search.set('mine', 'true');
	if (query.doula) search.set('doula', query.doula);
	if (cursor) search.set('cursor', cursor);
	const text = search.toString();
	return text ? `${contractsBase(practiceId)}?${text}` : contractsBase(practiceId);
}

export async function loadPracticeContracts(
	fetcher: Fetcher,
	practiceId: string,
	query: ContractsQuery,
	cursor?: string
): Promise<ContractsPage> {
	const search = query.search ?? '';
	const response = search
		? await fetcher(`${contractsBase(practiceId)}/list`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					view: query.view === 'not-signed' ? '' : query.view,
					order: query.order,
					doula: query.doula,
					mine: query.isMine,
					search,
					cursor: cursor ?? ''
				})
			})
		: await fetcher(practiceContractsPath(practiceId, query, cursor));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

export function practiceVoidRequestsPath(practiceId: string, cursor?: string): string {
	const path = `${contractsBase(practiceId)}/void-requests`;
	return cursor ? `${path}?cursor=${encodeURIComponent(cursor)}` : path;
}

export async function loadPracticeVoidRequests(
	fetcher: Fetcher,
	practiceId: string,
	cursor?: string
): Promise<VoidRequestsPage> {
	const response = await fetcher(practiceVoidRequestsPath(practiceId, cursor));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

export function voidRequestsHeading(total: number): string {
	return `${total} void ${total === 1 ? 'request' : 'requests'} to decide`;
}

export function voidRequestAsked(daysAgo: number): string {
	if (daysAgo <= 0) return 'asked today';
	return `asked ${daysAgo} ${daysAgo === 1 ? 'day' : 'days'} ago`;
}

const STATUS_WORDS = { draft: 'Draft', sent: 'Sent', signed: 'Signed', voided: 'Voided' } as const;

export function contractStatusWord(status: ContractListItem['status']): string {
	return STATUS_WORDS[status];
}

export function signedContractFilename(clientName: string): string {
	const slug = clientName
		.normalize('NFD')
		.replaceAll(/[̀-ͯ]/g, '')
		.toLowerCase()
		.replaceAll(/[^a-z0-9]+/g, '-')
		.replaceAll(/^-+|-+$/g, '');
	return slug ? `signed-contract-${slug}.pdf` : 'signed-contract.pdf';
}

export function signedPdfDownloadedWords(clientName: string): string {
	return `Downloaded the Signed PDF for ${clientName}.`;
}

export function careWhen(line: Parameters<typeof dueParts>[0], timeZone: string, formatDay: (day: string) => string): DueParts {
	const parts = dueParts(line, timeZone, formatDay);
	return parts.postpartum ? { ...parts, lead: 'Born', postpartum: '' } : parts;
}

export function daysWaitingWords(days: number): string {
	return `${days} ${days === 1 ? 'day' : 'days'}`;
}

const NOUNS: Record<ContractsView, { plural: string; singular: string; none: string; predicate: string }> = {
	'not-signed': { plural: 'Contracts not signed', singular: 'Contract not signed', none: 'Contract', predicate: 'is waiting to be signed' },
	draft: { plural: 'Draft Contracts', singular: 'Draft Contract', none: 'Draft Contract', predicate: 'is waiting to be sent' },
	sent: { plural: 'Sent Contracts', singular: 'Sent Contract', none: 'Sent Contract', predicate: 'is waiting on a Client' },
	signed: { plural: 'Signed Contracts', singular: 'Signed Contract', none: 'Contract', predicate: 'is signed' },
	voided: { plural: 'Voided Contracts', singular: 'Voided Contract', none: 'Contract', predicate: 'has been voided' },
	all: { plural: 'Contracts', singular: 'Contract', none: 'Contract', predicate: '' }
};

export const CONTRACT_VIEW_TAGS: Partial<Record<ContractsView, string>> = {
	draft: 'Draft Contracts',
	sent: 'Sent Contracts',
	signed: 'Signed Contracts',
	voided: 'Voided Contracts'
};

export const NO_DOULA_FALLBACK = 'that Doula';

function orderWords(view: ContractsView, order: ContractOrder): string {
	const named: NamedContractOrder = order || VIEW_ORDER[view];
	if (named === 'client') return 'Client A to Z';
	if (view === 'sent') return named === 'oldest' ? 'longest wait first' : 'shortest wait first';
	return `${named} first`;
}

export interface ContractsSummaryInput {
	view: ContractsView;
	order: ContractOrder;
	shown: number;
	hasMore: boolean;
	total: number;
	isMine: boolean;
	doulaName: string;
	search: string;
}

export function contractsResultSummary(input: ContractsSummaryInput): string {
	const { view, order, shown, hasMore, total, isMine, doulaName, search } = input;
	const noun = NOUNS[view];
	const doula = doulaName ? ` on ${doulaName}'s Engagements` : '';
	const matching = search ? ` matching "${search}"` : '';
	if (shown === 0) {
		const scope = `${isMine ? ' of yours' : ''}${doula}${matching}`;
		if (view === 'all') {
			return scope ? `No Contract${scope}.` : 'No Contracts yet. One appears here when a Contract is made on an Engagement.';
		}
		return `No ${noun.none}${scope} ${noun.predicate}.`;
	}
	let count: string;
	if (hasMore) count = `the first ${shown} of ${isMine ? 'your ' : ''}${Math.max(total, shown)} ${noun.plural}`;
	else if (isMine) count = `${shown} of your ${noun.plural}`;
	else count = shown === 1 ? `1 ${noun.singular}` : `${shown} ${noun.plural}`;
	return `Showing ${count}${doula}${matching}, ${orderWords(view, order)}.`;
}

const RULER_DOULA = 'D'.repeat(30);

export const CONTRACTS_RESULT_RULER = (Object.keys(NOUNS) as ContractsView[])
	.flatMap((view) =>
		(['', 'oldest', 'newest', 'client'] as const).flatMap((order) =>
			[true, false].map((isMine) =>
				contractsResultSummary({
					view,
					order,
					shown: 99,
					hasMore: true,
					total: 9999,
					isMine,
					doulaName: RULER_DOULA,
					search: ''
				})
			)
		)
	)
	.toSorted((a, b) => b.length - a.length)[0];

