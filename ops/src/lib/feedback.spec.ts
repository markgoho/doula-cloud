import { describe, expect, it, vi } from 'vitest';
import {
	issueLabel,
	pieceFacts,
	readFeedback,
	readFeedbackPiece,
	sentAt,
	summaryFacts,
	type FeedbackPiece,
	type FeedbackSummary
} from './feedback.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

const deadLettered: FeedbackSummary = {
	id: 'f-1',
	kind: 'not_working',
	routeId: '/portal/[clientId]',
	sentAt: '2026-09-20T15:30:00Z',
	issue: { state: 'dead_lettered', attempts: 8, lastError: '401 Bad credentials', deadLetterReason: 'retries_exhausted' }
};

const piece: FeedbackPiece = {
	id: 'f-2',
	kind: 'idea_or_request',
	text: 'Line one.\nLine two.',
	pageUrl: '/practices/x/invoices',
	routeId: '/practices/[practiceId]/invoices',
	appBuild: 'abc1234',
	screenWidth: 1280,
	browser: 'Firefox 131',
	sentAt: '2026-09-20T14:30:00Z',
	role: 'owner, doula',
	practiceName: 'Rooted Birth Collective',
	sender: { kind: 'staff', name: 'Renata Alvarez', email: 'renata@example.com' },
	issue: { state: 'opened', number: 17, url: 'https://github.com/acme/feedback/issues/17', attempts: 0 }
};

describe('sentAt', () => {
	it('states the instant to the minute in New York', () => {
		expect(sentAt('2026-09-20T15:30:00Z')).toBe('September 20, 2026 at 11:30 AM EDT');
	});
});

describe('issueLabel', () => {
	it('says what became of the issue in one line', () => {
		expect(issueLabel({ state: 'opened', number: 17, attempts: 0 })).toBe('Issue #17');
		expect(issueLabel({ state: 'pending', attempts: 0 })).toBe('Waiting to open');
		expect(issueLabel({ state: 'retrying', attempts: 1 })).toBe('Not opened yet: 1 attempt failed');
		expect(issueLabel({ state: 'retrying', attempts: 2 })).toBe('Not opened yet: 2 attempts failed');
		expect(issueLabel(deadLettered.issue)).toBe('Not opened: all 8 attempts failed, and nothing will try again');
		expect(issueLabel({ state: 'dead_lettered', attempts: 1, deadLetterReason: 'own_data' })).toBe(
			'Not opened: GitHub refused what this piece would send, and nothing will try again'
		);
	});
});

describe('summaryFacts', () => {
	it('states the kind, route and issue, and the last error where an attempt failed', () => {
		expect(summaryFacts(deadLettered)).toEqual([
			{ key: 'Kind', value: 'Something is not working' },
			{ key: 'Route', value: '/portal/[clientId]' },
			{ key: 'Issue', value: 'Not opened: all 8 attempts failed, and nothing will try again' },
			{ key: 'Last error', value: '401 Bad credentials' }
		]);
		expect(summaryFacts({ ...deadLettered, kind: 'something_else', issue: { state: 'pending', attempts: 0 } })).toEqual([
			{ key: 'Kind', value: 'Something else' },
			{ key: 'Route', value: '/portal/[clientId]' },
			{ key: 'Issue', value: 'Waiting to open' }
		]);
	});

	it('prints a kind it does not know as its own value', () => {
		expect(summaryFacts({ ...deadLettered, kind: 'praise' })[0]).toEqual({ key: 'Kind', value: 'praise' });
	});
});

describe('pieceFacts', () => {
	it('states every stored field but the words, with the sender named', () => {
		expect(pieceFacts(piece)).toEqual([
			{ key: 'Kind', value: 'An idea or a request' },
			{ key: 'Sent', value: 'September 20, 2026 at 10:30 AM EDT' },
			{ key: 'Sender', value: 'Renata Alvarez' },
			{ key: 'Email address', value: 'renata@example.com' },
			{ key: 'Role', value: 'Owner, Doula' },
			{ key: 'Practice', value: 'Rooted Birth Collective' },
			{ key: 'Page', value: '/practices/x/invoices' },
			{ key: 'Route', value: '/practices/[practiceId]/invoices' },
			{ key: 'App build', value: 'abc1234' },
			{ key: 'Screen width', value: '1280px' },
			{ key: 'Browser', value: 'Firefox 131' }
		]);
	});

	it('says so in words where a field holds nothing', () => {
		const facts = pieceFacts({
			...piece,
			role: '',
			practiceName: undefined,
			sender: { kind: 'client', name: '', email: '' }
		});
		expect(facts.slice(2, 6)).toEqual([
			{ key: 'Sender', value: 'A Client with no name on record' },
			{ key: 'Email address', value: 'No address on record' },
			{ key: 'Role', value: 'No role' },
			{ key: 'Practice', value: 'None: sent from a screen outside any Practice' }
		]);
	});

	it('names a Client sender by her name, and her role as Client', () => {
		const facts = pieceFacts({ ...piece, role: 'Client', sender: { kind: 'client', name: 'Nadia Haddad', email: 'n@example.com' } });
		expect(facts[2]).toEqual({ key: 'Sender', value: 'Nadia Haddad, a Client' });
		expect(facts[4]).toEqual({ key: 'Role', value: 'Client' });
	});

	it('names a Staff sender with no name on record', () => {
		expect(pieceFacts({ ...piece, sender: { kind: 'staff', name: '', email: 'x@example.com' } })[2]).toEqual({
			key: 'Sender',
			value: 'No name on record'
		});
	});
});

describe('readFeedback', () => {
	it('reads a page of every piece, or of the unopened ones, after a cursor', async () => {
		const fetcher = answering(200, { items: [deadLettered], hasMore: false });

		expect(await readFeedback('all', undefined, fetcher)).toEqual({
			kind: 'ok',
			value: { items: [deadLettered], hasMore: false }
		});
		await readFeedback('unopened', 'c/1', fetcher);

		expect(fetcher).toHaveBeenNthCalledWith(1, '/api/feedback', { credentials: 'include' });
		expect(fetcher).toHaveBeenNthCalledWith(2, '/api/feedback?issue=unopened&cursor=c%2F1', { credentials: 'include' });
	});

	it('reads a later page of every piece', async () => {
		const fetcher = answering(200, { items: [], hasMore: false });
		await readFeedback('all', 'c1', fetcher);
		expect(fetcher).toHaveBeenCalledWith('/api/feedback?cursor=c1', { credentials: 'include' });
	});

	it('is signed out on a 401', async () => {
		expect(await readFeedback('all', undefined, answering(401, {}))).toEqual({ kind: 'signed-out' });
	});
});

describe('readFeedbackPiece', () => {
	it('reads one piece', async () => {
		const fetcher = answering(200, piece);
		expect(await readFeedbackPiece('f-2', fetcher)).toEqual({ kind: 'ok', value: piece });
		expect(fetcher).toHaveBeenCalledWith('/api/feedback/f-2', { credentials: 'include' });
	});

	it('passes on the refusal of a piece that is gone', async () => {
		const message = 'There is no piece of Feedback at this address.';
		expect(await readFeedbackPiece('gone', answering(404, { code: 'NOT_FOUND', message }))).toEqual({
			kind: 'refused',
			errors: [{ message }]
		});
	});
});
