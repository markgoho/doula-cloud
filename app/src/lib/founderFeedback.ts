/**
 * Reading a piece of Feedback (#1526): the founder read page's two reads
 * and the words it prints. Kept apart from the two routes the way
 * `invoice.ts` sits apart from the Invoice screens -- a domain module,
 * unit-tested with an injected `Fetcher`.
 *
 * Sending a piece is `feedback.ts`; this module is the other end of it,
 * and only one person ever reaches it (`staffauth.FounderOnly`).
 */
import { apiErrorMessage } from './apiErrorMessage.js';
import { kindOptions, type Kind } from './feedback.js';
import type { Fetcher } from './fetcher.js';
import type { CursorPage } from './paginatedList.svelte.js';

/**
 * What became of a piece's GitHub issue (`api/internal/feedback.
 * IssueStatus`). `retrying` and `dead_lettered` are the two states the
 * list prints under its own heading: something has gone wrong with the
 * open, which is how a lapsed token shows (#1500).
 */
export interface IssueStatus {
	state: 'opened' | 'pending' | 'retrying' | 'dead_lettered';
	number?: number;
	/**
	Absent where the BFF has no repository configured.
	*/
	url?: string;
	attempts: number;
	lastError?: string;
}

/**
One row of the list. No free text and nobody's name: see `feedback.Summary`.
*/
export interface FeedbackSummary {
	id: string;
	kind: Kind;
	routeId: string;
	sentAt: string;
	issue: IssueStatus;
}

export type FeedbackPage = CursorPage<FeedbackSummary>;

/**
Every stored field of one piece (`api/internal/feedback.Piece`).
*/
export interface FeedbackPiece {
	id: string;
	kind: Kind;
	text: string;
	pageUrl: string;
	routeId: string;
	appBuild: string;
	screenWidth: number;
	browser: string;
	sentAt: string;
	/**
	The role(s), "Client", or "Staff"; empty for a membership with no role.
	*/
	role: string;
	practiceName?: string;
	sender: { kind: 'staff' | 'client'; name?: string; email: string };
	issue: IssueStatus;
}

/** The list's path. `isUnopenedOnly` narrows it to the pieces whose
 * issue open has failed -- the list the page prints first. */
export function feedbackListPath(cursor?: string, isUnopenedOnly = false): string {
	const query = new URLSearchParams();
	if (isUnopenedOnly) {
		query.set('issue', 'unopened');
	}
	if (cursor) {
		query.set('cursor', cursor);
	}
	const search = query.toString();
	return search ? `/api/staff/feedback?${search}` : '/api/staff/feedback';
}

/**
The path of one piece. Reading it is what the BFF records.
*/
export function feedbackPiecePath(feedbackId: string): string {
	return `/api/staff/feedback/${feedbackId}`;
}

/** Loads one page of the list, newest first. Throws with the server's
 * sentence on a non-2xx response -- the route's `load` maps the first
 * page's status codes itself, so a throw here is a later page failing. */
export async function loadFeedbackPage(fetcher: Fetcher, cursor?: string, isUnopenedOnly = false): Promise<FeedbackPage> {
	const response = await fetcher(feedbackListPath(cursor, isUnopenedOnly));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/**
The sentence the sender chose, exactly as the form offered it. A kind
this build does not know -- the BFF gained one first -- prints as its own
value, so one new piece cannot take the whole list down.
*/
export function kindLabel(kind: Kind): string {
	return kindOptions.find((option) => option.value === kind)?.label ?? kind;
}

/**
What became of the issue, in one line.
*/
export function issueLabel(issue: IssueStatus): string {
	switch (issue.state) {
		case 'opened': {
			return `Issue #${issue.number}`;
		}
		case 'pending': {
			return 'Waiting to open';
		}
		case 'retrying': {
			return `Not opened yet: ${issue.attempts} ${issue.attempts === 1 ? 'attempt' : 'attempts'} failed`;
		}
		case 'dead_lettered': {
			return `Not opened: all ${issue.attempts} attempts failed`;
		}
	}
}

/**
The instant a piece was sent, to the minute, in the reader's zone.
*/
export function formatSentAt(value: string): string {
	return new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

/**
 * Every stored field of a piece but its free text, as `DescriptionList`
 * rows (#1526's own AC). The text is the sender's own words and the page
 * prints it on its own; the issue has its own section, because it can
 * carry a link and a list row cannot.
 *
 * A field with nothing in it says so in words. A blank value would read
 * as a field that failed to load.
 */
export function pieceFacts(piece: FeedbackPiece): { label: string; value: string }[] {
	return [
		{ label: 'Kind', value: kindLabel(piece.kind) },
		{ label: 'Sent', value: formatSentAt(piece.sentAt) },
		{
			label: 'Sender',
			value: piece.sender.kind === 'client' ? 'A Client, named by her sign-in address' : (piece.sender.name ?? 'No name on record')
		},
		{ label: 'Email', value: piece.sender.email || 'No address on record' },
		{ label: 'Role', value: piece.role || 'No role assigned' },
		{ label: 'Practice', value: piece.practiceName ?? 'None: sent from a screen outside any Practice' },
		{ label: 'Page', value: piece.pageUrl },
		{ label: 'Route', value: piece.routeId },
		{ label: 'App build', value: piece.appBuild },
		{ label: 'Screen width', value: `${piece.screenWidth}px` },
		{ label: 'Browser', value: piece.browser }
	];
}
