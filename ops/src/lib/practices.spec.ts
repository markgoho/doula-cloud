/* eslint-disable unicorn/no-null -- doula-ops answers null for a field with nothing in it */
import { describe, expect, it, vi } from 'vitest';
import { practiceFacts, readPractices, type PracticeRow } from './practices.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

const rooted: PracticeRow = {
	id: 'p-1',
	name: 'Rooted Birth Collective',
	signedUpAt: '2026-09-14T15:00:00Z',
	owners: [
		{ name: 'Renata Alvarez', email: 'renata@example.com' },
		{ name: 'Ada Bell', email: 'ada@example.com' }
	],
	staffCount: 3,
	stripeConnect: {
		connected: true,
		cardPayments: 'active',
		payouts: 'pending',
		requirementsDue: ['external_account', 'business_profile.url'],
		updatedAt: '2026-09-15T10:00:00Z'
	},
	creditsHeld: 12,
	engagementsStarted: 2,
	foundingGrant: { credits: 9, givenBy: 'Mark Goho', givenAt: '2026-09-15T10:00:00Z' },
	deletion: null,
	source: null,
	openFeedback: 2
};

const brandNew: PracticeRow = {
	...rooted,
	owners: [],
	staffCount: 0,
	stripeConnect: { connected: false, cardPayments: 'unsupported', payouts: 'unsupported', requirementsDue: [], updatedAt: null },
	creditsHeld: 1,
	engagementsStarted: 0,
	foundingGrant: null,
	openFeedback: 0
};

describe('practiceFacts', () => {
	it('states every field the list shows, with the source marked as not yet recorded', () => {
		expect(practiceFacts(rooted)).toEqual([
			{ label: 'Signed up', value: 'September 14, 2026' },
			{ label: 'Owners', value: 'Renata Alvarez, renata@example.com; Ada Bell, ada@example.com' },
			{ label: 'Staff', value: '3' },
			{
				label: 'Stripe Connect',
				value: 'Card payments active, payouts pending. Still due: external_account, business_profile.url. Updated September 15, 2026'
			},
			{ label: 'Credits held', value: '12 Credits' },
			{ label: 'Engagements started', value: '2' },
			{ label: 'Founding grant', value: '9 Credits, given by Mark Goho on September 15, 2026' },
			{ label: 'Deletion', value: 'Not asked for' },
			{ label: 'Source', value: 'Not recorded yet' },
			{ label: 'Open Feedback', value: '2' }
		]);
	});

	it('says what a new Practice does not have yet', () => {
		const facts = Object.fromEntries(practiceFacts(brandNew).map((fact) => [fact.label, fact.value]));

		expect(facts['Owner']).toBe('No Owner');
		expect(facts['Stripe Connect']).toBe('Not connected');
		expect(facts['Credits held']).toBe('1 Credit');
		expect(facts['Founding grant']).toBe('Not given');
	});

	it('names one Owner, a connected account with nothing due, and a grant given before ops', () => {
		const facts = Object.fromEntries(
			practiceFacts({
				...rooted,
				owners: [rooted.owners[0]],
				stripeConnect: { ...rooted.stripeConnect, requirementsDue: [], updatedAt: null },
				foundingGrant: { credits: 6, givenBy: null, givenAt: '2026-09-15T10:00:00Z' }
			}).map((fact) => [fact.label, fact.value])
		);

		expect(facts['Owner']).toBe('Renata Alvarez, renata@example.com');
		expect(facts['Stripe Connect']).toBe('Card payments active, payouts pending');
		expect(facts['Founding grant']).toBe('6 Credits, given on September 15, 2026, before ops recorded who gave it');
	});

	it('states a Deletion in its window and a finalized one', () => {
		const pending = practiceFacts({
			...rooted,
			deletion: { requestedAt: '2026-09-15T10:00:00Z', finalizesAt: '2026-10-15T10:00:00Z', deletedAt: null }
		});
		const deleted = practiceFacts({
			...rooted,
			deletion: { requestedAt: '2026-09-15T10:00:00Z', finalizesAt: '2026-10-15T10:00:00Z', deletedAt: '2026-10-15T10:05:00Z' }
		});

		expect(pending).toContainEqual({ label: 'Deletion', value: 'Asked for September 15, 2026. Finalizes October 15, 2026' });
		expect(deleted).toContainEqual({ label: 'Deletion', value: 'Deleted October 15, 2026' });
	});
});

describe('readPractices', () => {
	const page = { items: [rooted], hasMore: true, nextCursor: 'c-2' };

	it('reads the first page of the live list', async () => {
		const fetcher = answering(200, page);

		expect(await readPractices('live', undefined, fetcher)).toEqual({ kind: 'ok', value: page });
		expect(fetcher).toHaveBeenCalledWith('/api/practices', { credentials: 'include' });
	});

	it('reads a later page of the Deleted list', async () => {
		const fetcher = answering(200, { items: [], hasMore: false });

		await readPractices('deleted', 'c 2', fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/deleted?cursor=c%202', { credentials: 'include' });
	});

	it('names an ended session', async () => {
		expect(await readPractices('live', undefined, answering(401, { message: 'ended' }))).toEqual({ kind: 'signed-out' });
	});
});
