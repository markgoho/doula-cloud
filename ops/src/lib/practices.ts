import { refusal, type Outcome } from './act.ts';
import { credits } from './foundingGrant.ts';
import { longDate } from './refund.ts';

type Instant = string;

export type PracticeRow = {
	id: string;
	name: string;
	signedUpAt: Instant;
	owners: { name: string; email: string }[];
	staffCount: number;
	stripeConnect: {
		connected: boolean;
		cardPayments: string;
		payouts: string;
		requirementsDue: string[];
		updatedAt: Instant | null;
	};
	creditsHeld: number;
	engagementsStarted: number;
	foundingGrant: { credits: number; givenBy: string | null; givenAt: Instant } | null;
	deletion: { requestedAt: Instant | null; finalizesAt: Instant | null; deletedAt: Instant | null } | null;
	source: string | null;
	openFeedback: number;
};

export type PracticePage = { items: PracticeRow[]; nextCursor?: string; hasMore: boolean };

export type PracticeList = 'live' | 'deleted';

type Fetch = typeof fetch;

export function day(instant: Instant): string {
	return longDate(instant.slice(0, 10));
}

function owners(row: PracticeRow): { key: string; value: string } {
	const named = row.owners.map((owner) => `${owner.name}, ${owner.email}`);
	if (named.length === 0) return { key: 'Owner', value: 'No Owner' };
	return { key: named.length === 1 ? 'Owner' : 'Owners', value: named.join('; ') };
}

function stripeConnect({ stripeConnect: connect }: PracticeRow): string {
	if (!connect.connected) return 'Not connected';
	const parts = [`Card payments ${connect.cardPayments}, payouts ${connect.payouts}`];
	if (connect.requirementsDue.length > 0) parts.push(`Still due: ${connect.requirementsDue.join(', ')}`);
	if (connect.updatedAt) parts.push(`Updated ${day(connect.updatedAt)}`);
	return parts.join('. ');
}

function foundingGrant({ foundingGrant: grant }: PracticeRow): string {
	if (!grant) return 'Not given';
	if (!grant.givenBy) return `${credits(grant.credits)}, given on ${day(grant.givenAt)}, before ops recorded who gave it`;
	return `${credits(grant.credits)}, given by ${grant.givenBy} on ${day(grant.givenAt)}`;
}

function deletion({ deletion: d }: PracticeRow): string {
	if (d?.deletedAt) return `Deleted ${day(d.deletedAt)}`;
	if (d?.requestedAt && d.finalizesAt) return `Asked for ${day(d.requestedAt)}. Finalizes ${day(d.finalizesAt)}`;
	return 'Not asked for';
}

/**
Each field the Practice list shows for one Practice, as a summary list's rows.
*/
export function practiceFacts(row: PracticeRow): { key: string; value: string }[] {
	return [
		{ key: 'Signed up', value: day(row.signedUpAt) },
		owners(row),
		{ key: 'Staff', value: String(row.staffCount) },
		{ key: 'Stripe Connect', value: stripeConnect(row) },
		{ key: 'Credits held', value: credits(row.creditsHeld) },
		{ key: 'Engagements started', value: String(row.engagementsStarted) },
		{ key: 'Founding grant', value: foundingGrant(row) },
		{ key: 'Deletion', value: deletion(row) },
		{ key: 'Source', value: row.source ?? 'Not recorded yet' },
		{ key: 'Open Feedback', value: String(row.openFeedback) }
	];
}

/**
One page of the live list or the Deleted list, newest signup first, after cursor.
*/
export async function readPractices(
	list: PracticeList,
	cursor: string | undefined,
	fetcher: Fetch = fetch
): Promise<Outcome<PracticePage>> {
	const path = list === 'deleted' ? '/api/practices/deleted' : '/api/practices';
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(path + query, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
