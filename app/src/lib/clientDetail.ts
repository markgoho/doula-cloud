/**
 * A Client's detail read (ADR-0017): her twelve structural columns, her
 * Practice-defined layer resolved live against the current Client Field
 * Template, her Engagements, and her merged history -- mirrors the Go
 * BFF's client.DetailResponse (api/internal/client/detail.go). Decoupled
 * from SvelteKit and the DOM so it can be unit-tested directly, the same
 * seam client.ts's list/create orchestration uses.
 */

/**
A Client's structural core, Record-shaped -- mirrors client.Record.
*/
export interface ClientRecord {
	id: string;
	givenName: string;
	familyName: string;
	preferredName: string;
	email: string;
	phone: string;
	addressLine1: string;
	addressLine2: string;
	addressLocality: string;
	addressRegion: string;
	addressPostalCode: string;
	dateOfBirth: string;
}

/** One Practice-defined field, resolved live against the current Client
 * Field Template -- mirrors client.ResolvedField. `value` is absent for
 * a blank active field; an archived field a Client still holds a value
 * under carries `note`. */
export interface ResolvedField {
	fieldId: string;
	label: string;
	type: string;
	options?: string[];
	value?: unknown;
	note?: string;
}

/** One of a Client's Engagements, past or present -- mirrors
 * client.EngagementSummary. */
export interface EngagementSummary {
	engagementId: string;
	kind: string;
	status: string;
	createdAt: string;
}

/** One activity row on a Client's record -- mirrors client.Event.
 * `actorName` is absent only when `actorKind` is "system". */
export interface ClientEvent {
	eventType: string;
	diff: unknown;
	actorKind: string;
	actorStaffId?: string;
	actorName?: string;
	createdAt: string;
}

/** One engagement_requests row -- mirrors client.RequestSummary.
 * `decidedBy`/`decidedByName` are absent while the Request is pending. */
export interface EngagementRequestSummary {
	requestId: string;
	kind: string;
	state: string;
	requestedBy: string;
	requestedByName: string;
	requestedAt: string;
	decidedBy?: string;
	decidedByName?: string;
	decidedAt?: string;
	reason?: string;
	engagementId?: string;
}

/** One row of a Client's merged history -- mirrors client.HistoryEntry. A
 * discriminated union on `type`, matching the Go BFF's own "exactly one
 * of ClientEvent/EngagementRequest is set" contract, so a reader
 * switches on `type` instead of null-checking either payload. */
export type HistoryEntry =
	| { type: 'client_event'; at: string; clientEvent: ClientEvent }
	| { type: 'engagement_request'; at: string; engagementRequest: EngagementRequestSummary };

/**
A Client's full detail read -- mirrors client.DetailResponse.
*/
export interface ClientDetail extends ClientRecord {
	resolvedFields: ResolvedField[];
	engagements: EngagementSummary[];
	history: HistoryEntry[];
}

/** A minimal fetch-shaped function, injected rather than imported --
 * mirrors client.ts's Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

/** Loads one Client's full detail read. Throws with the response body
 * text on a non-2xx response, mirroring loadClients' error-surfacing
 * convention. */
export async function loadClientDetail(fetcher: Fetcher, practiceId: string, clientId: string): Promise<ClientDetail> {
	const response = await fetcher(`/api/practices/${practiceId}/clients/${clientId}`);
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** The name every screen reads (ADR-0017's read table): preferred name
 * when she has one, otherwise given plus family name. */
export function displayName(record: ClientRecord): string {
	if (record.preferredName) {
		return record.preferredName;
	}
	return [record.givenName, record.familyName].filter(Boolean).join(' ');
}

/** Renders a resolved Practice-defined field's stored value as text, per
 * ADR-0001's field-type palette -- a `section_header` carries no value
 * and is never passed here. Undefined/null (a blank active field) reads
 * as an empty string, so the row still renders, blank, per ADR-0017's
 * "every active field appears even when blank". */
export function resolvedFieldValueText(field: ResolvedField): string {
	if (field.value === undefined || field.value === null) {
		return '';
	}
	if (field.type === 'checkbox') {
		return field.value === true ? 'Yes' : 'No';
	}
	if (field.type === 'multi_select') {
		return Array.isArray(field.value) ? field.value.join(', ') : String(field.value);
	}
	return String(field.value);
}

/** The Requests still open for this Client, one per kind -- the block
 * ADR-0017 names "the Client's record shows a pending-request block
 * naming who asked and when". */
export function pendingRequests(history: HistoryEntry[]): EngagementRequestSummary[] {
	return history
		.filter((entry) => entry.type === 'engagement_request')
		.map((entry) => entry.engagementRequest)
		.filter((request) => request.state === 'pending');
}
