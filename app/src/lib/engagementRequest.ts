/**
 * The Engagement Request screen's own orchestration (#496, ADR-0017):
 * "Start new work with her" -- the ask off the Client detail hub, wired to
 * api/internal/engagementrequest.RequestHandler. Decoupled from SvelteKit
 * and the DOM, the same seam client.ts and clientDetail.ts use.
 */

/** The body a new Request submits: the kind and due date the requester
 * states as part of the ask, and an optional note -- ADR-0017's "the
 * requester describes the work; the approver does not amend it". Mirrors
 * the Go BFF's RequestBody. */
export interface NewEngagementRequest {
	kind: 'birth' | 'postpartum';
	dueDate: string;
	note: string;
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

/** A minimal fetch-shaped function, injected rather than imported --
 * mirrors client.ts's Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

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
		throw new Error(await response.text());
	}
	return { noCredits: false, outcome: await response.json() };
}
