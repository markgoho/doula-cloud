import { describe, expect, it, vi } from 'vitest';
import { jsonResponse } from './testResponse.js';
import {
	feedbackListPath,
	issueLabel,
	kindLabel,
	loadFeedbackPage,
	pieceFacts,
	type FeedbackPiece
} from './founderFeedback.js';

const piece: FeedbackPiece = {
	id: 'feedback-1',
	kind: 'not_working',
	text: 'The invoice total is wrong.',
	pageUrl: '/practices/practice-1/invoices',
	routeId: '/practices/[practiceId]/invoices',
	appBuild: 'abc1234',
	screenWidth: 390,
	browser: 'Safari 18',
	sentAt: '2026-10-20T08:15:00Z',
	role: 'owner, admin',
	practiceName: 'Riverside Doula Collective',
	sender: { kind: 'staff', name: 'Anne-Marie Ochieng', email: 'anne-marie@example.test' },
	issue: { state: 'opened', number: 12, url: 'https://github.com/markgoho/doula-cloud-feedback/issues/12', attempts: 0 }
};

function valueOf(facts: { label: string; value: string }[], label: string): string | undefined {
	return facts.find((fact) => fact.label === label)?.value;
}

describe('feedbackListPath', () => {
	it.each([
		[undefined, false, '/api/staff/feedback'],
		['abc', false, '/api/staff/feedback?cursor=abc'],
		[undefined, true, '/api/staff/feedback?issue=unopened'],
		['abc', true, '/api/staff/feedback?issue=unopened&cursor=abc']
	])('cursor %s, unopened only %s reads %s', (cursor, isUnopenedOnly, expected) => {
		expect(feedbackListPath(cursor, isUnopenedOnly)).toBe(expected);
	});
});

describe('loadFeedbackPage', () => {
	it('asks for the page the cursor and the narrowing name', async () => {
		const page = { items: [], hasMore: false };
		const fetcher = vi.fn(async () => jsonResponse(page));

		await expect(loadFeedbackPage(fetcher, 'abc', true)).resolves.toEqual(page);
		expect(fetcher).toHaveBeenCalledWith('/api/staff/feedback?issue=unopened&cursor=abc');
	});

	it('throws the server sentence when the page does not load', async () => {
		const fetcher = vi.fn(async () => jsonResponse({ code: 'INTERNAL', message: 'internal error' }, 500));

		await expect(loadFeedbackPage(fetcher)).rejects.toThrow();
	});
});

describe('kindLabel', () => {
	it.each([
		['not_working', 'Something is not working'],
		['idea_or_request', 'An idea or a request'],
		['something_else', 'Something else']
	] as const)('reads %s as the sentence the sender chose: %s', (kind, expected) => {
		expect(kindLabel(kind)).toBe(expected);
	});

	it('prints a kind this build does not know as its own value', () => {
		expect(kindLabel('a_newer_kind' as Parameters<typeof kindLabel>[0])).toBe('a_newer_kind');
	});
});

describe('issueLabel', () => {
	it.each([
		[{ state: 'opened', number: 12, attempts: 0 }, 'Issue #12'],
		[{ state: 'pending', attempts: 0 }, 'Waiting to open'],
		[{ state: 'retrying', attempts: 1 }, 'Not opened yet: 1 attempt failed'],
		[{ state: 'retrying', attempts: 3 }, 'Not opened yet: 3 attempts failed'],
		[{ state: 'dead_lettered', attempts: 5 }, 'Not opened: all 5 attempts failed']
	] as const)('reads %o as "%s"', (issue, expected) => {
		expect(issueLabel(issue)).toBe(expected);
	});
});

describe('pieceFacts', () => {
	it('lists every stored field but the free text, which the page prints on its own', () => {
		const facts = pieceFacts(piece);

		expect(facts.map((fact) => fact.label)).toEqual([
			'Kind',
			'Sent',
			'Sender',
			'Email',
			'Role',
			'Practice',
			'Page',
			'Route',
			'App build',
			'Screen width',
			'Browser'
		]);
		expect(valueOf(facts, 'Kind')).toBe('Something is not working');
		expect(valueOf(facts, 'Sender')).toBe('Anne-Marie Ochieng');
		expect(valueOf(facts, 'Email')).toBe('anne-marie@example.test');
		expect(valueOf(facts, 'Role')).toBe('owner, admin');
		expect(valueOf(facts, 'Practice')).toBe('Riverside Doula Collective');
		expect(valueOf(facts, 'Page')).toBe('/practices/practice-1/invoices');
		expect(valueOf(facts, 'Route')).toBe('/practices/[practiceId]/invoices');
		expect(valueOf(facts, 'App build')).toBe('abc1234');
		expect(valueOf(facts, 'Screen width')).toBe('390px');
		expect(valueOf(facts, 'Browser')).toBe('Safari 18');
		expect(valueOf(facts, 'Sent')).toContain('2026');
	});

	it('names a Client by her sign-in address, and says so', () => {
		const facts = pieceFacts({
			...piece,
			role: 'Client',
			sender: { kind: 'client', email: 'priya.signin@example.test' }
		});

		expect(valueOf(facts, 'Sender')).toBe('A Client, named by her sign-in address');
		expect(valueOf(facts, 'Email')).toBe('priya.signin@example.test');
	});

	it('says so when the piece has no Practice, no role and no name', () => {
		const facts = pieceFacts({
			...piece,
			role: '',
			practiceName: undefined,
			sender: { kind: 'staff', email: '' }
		});

		expect(valueOf(facts, 'Sender')).toBe('No name on record');
		expect(valueOf(facts, 'Email')).toBe('No address on record');

		expect(valueOf(facts, 'Practice')).toBe('None: sent from a screen outside any Practice');
		expect(valueOf(facts, 'Role')).toBe('No role assigned');
	});
});
