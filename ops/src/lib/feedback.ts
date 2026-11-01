import { refusal, type Outcome } from './act.ts';

type Instant = string;

export type IssueStatus = {
	state: 'opened' | 'pending' | 'retrying' | 'dead_lettered';
	number?: number;
	url?: string;
	attempts: number;
	lastError?: string;
	deadLetterReason?: 'retries_exhausted' | 'own_data';
};

export type FeedbackSummary = {
	id: string;
	kind: string;
	routeId: string;
	sentAt: Instant;
	issue: IssueStatus;
};

export type FeedbackPiece = {
	id: string;
	kind: string;
	text: string;
	pageUrl: string;
	routeId: string;
	appBuild: string;
	screenWidth: number;
	browser: string;
	sentAt: Instant;
	role: string;
	practiceName?: string;
	sender: { kind: 'staff' | 'client'; name: string; email: string };
	issue: IssueStatus;
};

export type FeedbackPage = { items: FeedbackSummary[]; nextCursor?: string; hasMore: boolean };

export type FeedbackList = 'all' | 'unopened';

type Fact = { key: string; value: string };

const KIND_LABELS: Record<string, string> = {
	not_working: 'Something is not working',
	idea_or_request: 'An idea or a request',
	something_else: 'Something else'
};

const ROLE_LABELS: Record<string, string> = { owner: 'Owner', admin: 'Admin', doula: 'Doula' };

/**
The instant a piece was sent, to the minute, in New York.
*/
export function sentAt(instant: Instant): string {
	const at = new Date(instant);
	const zone = { timeZone: 'America/New_York' } as const;
	const date = at.toLocaleDateString('en-US', { ...zone, month: 'long', day: 'numeric', year: 'numeric' });
	const time = at.toLocaleTimeString('en-US', { ...zone, hour: 'numeric', minute: '2-digit', timeZoneName: 'short' });
	return `${date} at ${time}`;
}

/**
What became of a piece's GitHub issue, in one line.
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
			return issue.deadLetterReason === 'own_data'
				? 'Not opened: GitHub refused what this piece would send, and nothing will try again'
				: `Not opened: all ${issue.attempts} attempts failed, and nothing will try again`;
		}
	}
}

function kindLabel(kind: string): string {
	return KIND_LABELS[kind] ?? kind;
}

/**
What the list shows for one piece: never the words, and nobody's name.
*/
export function summaryFacts(summary: FeedbackSummary): Fact[] {
	const facts = [
		{ key: 'Kind', value: kindLabel(summary.kind) },
		{ key: 'Route', value: summary.routeId },
		{ key: 'Issue', value: issueLabel(summary.issue) }
	];
	if (summary.issue.lastError) facts.push({ key: 'Last error', value: summary.issue.lastError });
	return facts;
}

function sender({ sender: from }: FeedbackPiece): string {
	if (from.kind === 'client') return from.name ? `${from.name}, a Client` : 'A Client with no name on record';
	return from.name || 'No name on record';
}

function role(piece: FeedbackPiece): string {
	if (!piece.role) return 'No role';
	return piece.role
		.split(', ')
		.map((value) => ROLE_LABELS[value] ?? value)
		.join(', ');
}

/**
Every stored field of a piece but its words, which the page prints on their own.
*/
export function pieceFacts(piece: FeedbackPiece): Fact[] {
	return [
		{ key: 'Kind', value: kindLabel(piece.kind) },
		{ key: 'Sent', value: sentAt(piece.sentAt) },
		{ key: 'Sender', value: sender(piece) },
		{ key: 'Email address', value: piece.sender.email || 'No address on record' },
		{ key: 'Role', value: role(piece) },
		{ key: 'Practice', value: piece.practiceName ?? 'None: sent from a screen outside any Practice' },
		{ key: 'Page', value: piece.pageUrl },
		{ key: 'Route', value: piece.routeId },
		{ key: 'App build', value: piece.appBuild },
		{ key: 'Screen width', value: `${piece.screenWidth}px` },
		{ key: 'Browser', value: piece.browser }
	];
}

/**
One page of every piece, or of the pieces whose issue did not open, newest first, after cursor.
*/
export async function readFeedback(
	list: FeedbackList,
	cursor: string | undefined,
	fetcher: typeof fetch = fetch
): Promise<Outcome<FeedbackPage>> {
	const query = new URLSearchParams();
	if (list === 'unopened') query.set('issue', 'unopened');
	if (cursor) query.set('cursor', cursor);
	const search = query.toString();
	const response = await fetcher(search ? `/api/feedback?${search}` : '/api/feedback', { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

/**
One piece. doula-ops records the read before it answers.
*/
export async function readFeedbackPiece(id: string, fetcher: typeof fetch = fetch): Promise<Outcome<FeedbackPiece>> {
	const response = await fetcher(`/api/feedback/${encodeURIComponent(id)}`, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
