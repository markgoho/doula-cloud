/**
 * The Engagement Request screen's own orchestration (#496, ADR-0017):
 * "Start new work with her" -- the ask off the Client detail hub, wired to
 * api/internal/engagementrequest.RequestHandler. Decoupled from SvelteKit
 * and the DOM, the same seam client.ts and clientDetail.ts use.
 */

import type { Fetcher } from './fetcher.js';

import { refusalError } from './formErrors.js';

/** The body a new Request submits: the kind and due date the requester
 * states as part of the ask, an optional note, and who the Doula is --
 * ADR-0017's "the requester describes the work; the approver does not
 * amend it". Mirrors the Go BFF's RequestBody.
 *
 * `doulaStaffId` is the one Doula she names (#1596): an employee, or a
 * Doula who holds Owner or Admin under each Employment type (#1625). It
 * is left out for "No Doula yet", which the BFF reads as nobody named. The
 * form refuses the submit until she has answered, so leaving it out is
 * always her answer and never a question she skipped. */
export interface NewEngagementRequest {
	kind: 'birth' | 'postpartum';
	dueDate: string;
	note: string;
	doulaStaffId?: string;
}

/** One answer the Start work form offers to "Who is the Doula?",
 * mirroring the Go BFF's engagementrequest.RequestDoula. */
export interface RequestDoula {
	staffId: string;
	name: string;
}

/** The form's whole list (engagementrequest.DoulasResponse): each person
 * this reader may name, herself first, and whether she is the only Doula
 * at the Practice -- the one case in which her answer is selected when
 * the form opens (ADR-0017's amendment on #1515). The server computes
 * that fact, because a plain Doula's list holds herself alone at a
 * Practice of any size. */
export interface RequestDoulas {
	items: RequestDoula[];
	callerIsOnlyDoula: boolean;
}

/** Loads who this reader may name as the Doula on a new Request
 * (engagementrequest.DoulasHandler). Throws with the response body text
 * on a refusal, mirroring loadApprovalDetail. */
export async function loadRequestDoulas(fetcher: Fetcher, practiceId: string): Promise<RequestDoulas> {
	const response = await fetcher(`/api/practices/${practiceId}/engagement-request-doulas`);
	if (!response.ok) {
		throw await refusalError(response);
	}
	return response.json();
}

/** The value the "No Doula yet" option carries on the form. A staff id
 * is a UUID, so this can never be one. */
export const NO_DOULA_YET = 'none';

/** The label of the "No Doula yet" answer, on the form, on the approval
 * screen, and on the Clients list's line for an Engagement with no
 * Doula on it (#1597), so an Owner reads on the list what she answered
 * on the form. GLOSSARY.md lists "Assignment" under Avoid for an
 * Attachment, so it is never "No Doula assigned". */
export const NO_DOULA_YET_LABEL = 'No Doula yet';

/** The options of "Who is the Doula?", in the order ADR-0017 gives them:
 * the person at the form first, her label ending "(you)"; then the rest
 * in the order the server sent them; "No Doula yet" last. The server
 * already puts the reader first, so this marks her and reorders nothing. */
export function doulaOptions(doulas: RequestDoula[], selfStaffId: string): { value: string; label: string }[] {
	return [
		...doulas.map(({ staffId, name }) => ({
			value: staffId,
			label: staffId === selfStaffId ? `${name} (you)` : name
		})),
		{ value: NO_DOULA_YET, label: NO_DOULA_YET_LABEL }
	];
}

/** The answer selected when the form opens: the reader herself where she
 * is the only Doula at the Practice, and nothing in every other case. */
export function initialDoulaAnswer(doulas: RequestDoulas, selfStaffId: string): string {
	return doulas.callerIsOnlyDoula ? selfStaffId : '';
}

/** RequestResponse, mirrored. `state` is "pending" for the ordinary path,
 * or "approved" with `engagementId` set when the requester already held
 * approval authority herself and ADR-0017's solo-Practice collapse fired.
 * `warning` carries the second-live-Engagement signal, ADR-0017's "warns,
 * never refuses". */
export interface EngagementRequestOutcome {
	requestId: string;
	state: 'pending' | 'approved';
	engagementId?: string;
	warning?: string;
}

/** The two shapes `requestEngagement` can settle to. A 402 -- an empty
 * balance on the collapsed Owner/Admin path, `billing.ErrNoCreditsRemaining`
 * -- is not a thrown error: it is an expected outcome this screen has a
 * real next step for (an inline Buy Credits path), the same discriminated-
 * union treatment client.ts's `EditClientResult` gives a 409 match
 * conflict. */
export type RequestEngagementResult =
	| { noCredits: false; outcome: EngagementRequestOutcome }
	| { noCredits: true };

function engagementRequestsPath(practiceId: string, clientId: string): string {
	return `/api/practices/${practiceId}/clients/${clientId}/engagement-requests`;
}

/** Opens an Engagement Request for one Client
 * (api/internal/engagementrequest.RequestHandler). Throws with the response
 * body text on any refusal but an empty balance -- a contractor's 403, a
 * validation 400, or the one-pending-per-kind 409 -- mirroring
 * loadClientDetail's error-surfacing convention. A 402 decodes into
 * `{ noCredits: true }` instead. */
export async function requestEngagement(
	fetcher: Fetcher,
	practiceId: string,
	clientId: string,
	request: NewEngagementRequest
): Promise<RequestEngagementResult> {
	const response = await fetcher(engagementRequestsPath(practiceId, clientId), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (response.status === 402) {
		return { noCredits: true };
	}
	if (!response.ok) {
		throw await refusalError(response);
	}
	return { noCredits: false, outcome: await response.json() };
}

/** The approval screen's read (#502), mirroring the Go BFF's
 * engagementrequest.DetailResponse. `balance` travels beside
 * `balanceAfter` so the screen can offer Buy Credits before an approval
 * that would fail; `balanceAfter` is honestly negative on an empty
 * balance. */
export interface ApprovalDetail {
	requestId: string;
	state: 'pending';
	kind: 'birth' | 'postpartum';
	dueDate?: string;
	note?: string;
	/** The Doula the Request names (#1596). Both are absent where the
	 * asker said "No Doula yet". */
	doulaStaffId?: string;
	doulaName?: string;
	requestedBy: string;
	requestedByName: string;
	requestedAt: string;
	client: {
		clientId: string;
		givenName: string;
		familyName: string;
		preferredName: string;
		isNewToPractice: boolean;
	};
	creditCost: number;
	balance: number;
	balanceAfter: number;
	engagements: { engagementId: string; kind: string; status: string; createdAt: string }[];
	warning?: string;
}

/** ApproveResponse, mirrored -- the Engagement approval created, and the
 * second-live-Engagement warning it carries at the approver's seat. */
export interface ApprovalOutcome {
	requestId: string;
	engagementId: string;
	state: 'approved';
	warning?: string;
}

/** The two shapes `approveRequest` can settle to, the same discriminated
 * union `requestEngagement` gives a 402: an empty balance is an expected
 * outcome with a real next step (Buy Credits), not a thrown error. */
export type ApproveRequestResult = { noCredits: false; outcome: ApprovalOutcome } | { noCredits: true };

function requestPath(practiceId: string, requestId: string): string {
	return `/api/practices/${practiceId}/engagement-requests/${requestId}`;
}

/** Loads one pending Request's full approval context
 * (engagementrequest.DetailHandler). Throws with the response body text
 * on any refusal -- a Doula's 403, an unknown id's 404, or the 409 a
 * Request somebody already decided answers with -- mirroring
 * loadClientDetail's error-surfacing convention. */
export async function loadApprovalDetail(
	fetcher: Fetcher,
	practiceId: string,
	requestId: string
): Promise<ApprovalDetail> {
	const response = await fetcher(requestPath(practiceId, requestId));
	if (!response.ok) {
		throw await refusalError(response);
	}
	return response.json();
}

/** Approves a pending Request (engagementrequest.ApproveHandler): the one
 * act that creates an Engagement and spends the Credit. A 402 decodes
 * into `{ noCredits: true }` rather than throwing. */
export async function approveRequest(
	fetcher: Fetcher,
	practiceId: string,
	requestId: string
): Promise<ApproveRequestResult> {
	const response = await fetcher(`${requestPath(practiceId, requestId)}/approve`, { method: 'POST' });
	if (response.status === 402) {
		return { noCredits: true };
	}
	if (!response.ok) {
		throw await refusalError(response);
	}
	return { noCredits: false, outcome: await response.json() };
}

/** Refuses a pending Request (engagementrequest.RefuseHandler). The
 * reason is required -- the endpoint 400s without one and
 * engagement_requests_refusal_reason refuses the row -- so the screen
 * asks for it before it ever calls this. */
export async function refuseRequest(
	fetcher: Fetcher,
	practiceId: string,
	requestId: string,
	reason: string
): Promise<void> {
	const response = await fetcher(`${requestPath(practiceId, requestId)}/refuse`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ reason })
	});
	if (!response.ok) {
		throw await refusalError(response);
	}
}

/** Withdraws the caller's own pending Request (engagementrequest.WithdrawHandler,
 * #504). ADR-0017: withdraw is the requester's own route out of a typo,
 * so the Client detail hub calls this only when she made the ask
 * herself -- the endpoint enforces that same rule server-side
 * (`requested_by = $1`) and 403s a requester mismatch it is asked to
 * trust anyway. Throws with the response body text on a refusal, the
 * same convention every other write in this module follows -- a 403 a
 * stale page still let through, or the 409 a Request somebody else
 * already decided answers with. */
export async function withdrawRequest(fetcher: Fetcher, practiceId: string, requestId: string): Promise<void> {
	const response = await fetcher(`${requestPath(practiceId, requestId)}/withdraw`, { method: 'POST' });
	if (!response.ok) {
		throw await refusalError(response);
	}
}

/*
 * Where an approver was when she ran out of Credits. Stripe's Checkout
 * success and cancel URLs are hardcoded to the Billing page
 * (billing/stripe_api_client.go), so there is no server-side way to carry
 * a return-to URL through the purchase. sessionStorage carries it
 * instead: the approval screen remembers its own address the moment an
 * empty balance is discovered, the Billing page offers the way back, and
 * the approval screen forgets it again the next time it is opened. Every
 * access is wrapped, because sessionStorage throws in a private window
 * with site data blocked and losing the shortcut is far smaller than
 * losing the screen.
 */
const APPROVAL_RETURN_KEY = 'engagement-request-approval-return';

/**
 * Remembers the approval screen to come back to after buying Credits.
 */
export function rememberApprovalReturn(href: string): void {
	try {
		sessionStorage.setItem(APPROVAL_RETURN_KEY, href);
	} catch {
		// Best effort: Buy Credits still works, she navigates back herself.
	}
}

/**
 * The approval screen waiting for a decision, if there is one.
 */
export function readApprovalReturn(): string {
	try {
		return sessionStorage.getItem(APPROVAL_RETURN_KEY) ?? '';
	} catch {
		return '';
	}
}

/** Forgets the remembered approval screen -- called by that screen itself
 * on its way in, so the way back is never offered to somebody who has
 * already taken it. */
export function forgetApprovalReturn(): void {
	try {
		sessionStorage.removeItem(APPROVAL_RETURN_KEY);
	} catch {
		// Nothing left to clean up if storage was never reachable.
	}
}

/** One row of the pending-Request inbox (#503), mirroring the Go BFF's
 * engagementrequest.ListItem. The Client's name arrives already resolved
 * -- the inbox names her, it does not print her record -- and `requestId`
 * is what the row links to, the approval screen being addressed by the
 * Request id alone. */
export interface PendingRequestItem {
	requestId: string;
	clientId: string;
	clientName: string;
	kind: 'birth' | 'postpartum';
	dueDate?: string;
	requestedByName: string;
	requestedAt: string;
}

/**
 * One page of the inbox, in docs/api-design.md section 4's envelope.
 */
export interface PendingRequestPage {
	items: PendingRequestItem[];
	nextCursor?: string;
	hasMore: boolean;
}

/** Lists every pending Request at the Practice, oldest first
 * (engagementrequest.ListHandler) -- one page at a time, `cursor`
 * resuming where the last page stopped. Throws with the response body
 * text on a refusal, mirroring loadApprovalDetail. */
export async function loadPendingRequests(
	fetcher: Fetcher,
	practiceId: string,
	cursor = ''
): Promise<PendingRequestPage> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`/api/practices/${practiceId}/engagement-requests${query}`);
	if (!response.ok) {
		throw await refusalError(response);
	}
	return response.json();
}

/*
 * The two screens either side of a Request -- the inbox and the approval
 * screen -- read a kind the same way, so the word it prints lives here
 * rather than in whichever of them was written first. The dates they
 * both print moved to `dates.ts` once the Client portal needed them too
 * (#505).
 */

/** The label a kind reads as, falling back to the raw enum value so an
 * enum this build has not met yet still prints something. */
export function kindLabel(kind: string): string {
	return { birth: 'Birth', postpartum: 'Postpartum' }[kind] ?? kind;
}
