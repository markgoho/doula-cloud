import { refusal, type Outcome } from './act.ts';
import { credits } from './foundingGrant.ts';
import { newYorkDay } from './dates.ts';

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

function owners(row: PracticeRow): { label: string; value: string } {
	const named = row.owners.map((owner) => `${owner.name}, ${owner.email}`);
	if (named.length === 0) return { label: 'Owner', value: 'No Owner' };
	return { label: named.length === 1 ? 'Owner' : 'Owners', value: named.join('; ') };
}

function stripeConnect({ stripeConnect: connect }: PracticeRow): string {
	if (!connect.connected) return 'Not connected';
	const parts = [`Card payments ${connect.cardPayments}, payouts ${connect.payouts}`];
	if (connect.requirementsDue.length > 0) parts.push(`Still due: ${connect.requirementsDue.join(', ')}`);
	if (connect.updatedAt) parts.push(`Updated ${newYorkDay(connect.updatedAt)}`);
	return parts.join('. ');
}

function foundingGrant({ foundingGrant: grant }: PracticeRow): string {
	if (!grant) return 'Not given';
	if (!grant.givenBy) return `${credits(grant.credits)}, given on ${newYorkDay(grant.givenAt)}, before ops recorded who gave it`;
	return `${credits(grant.credits)}, given by ${grant.givenBy} on ${newYorkDay(grant.givenAt)}`;
}

function deletion({ deletion: d }: PracticeRow): string {
	if (d?.deletedAt) return `Deleted ${newYorkDay(d.deletedAt)}`;
	if (d?.requestedAt && d.finalizesAt) return `Asked for ${newYorkDay(d.requestedAt)}. Finalizes ${newYorkDay(d.finalizesAt)}`;
	return 'Not asked for';
}

/**
Each field the Practice list shows for one Practice, as a summary list's rows.
*/
export function practiceFacts(row: PracticeRow): { label: string; value: string }[] {
	return [
		{ label: 'Signed up', value: newYorkDay(row.signedUpAt) },
		owners(row),
		{ label: 'Staff', value: String(row.staffCount) },
		{ label: 'Stripe Connect', value: stripeConnect(row) },
		{ label: 'Credits held', value: credits(row.creditsHeld) },
		{ label: 'Engagements started', value: String(row.engagementsStarted) },
		{ label: 'Founding grant', value: foundingGrant(row) },
		{ label: 'Deletion', value: deletion(row) },
		{ label: 'Source', value: row.source ?? 'Not recorded yet' },
		{ label: 'Open Feedback', value: String(row.openFeedback) }
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
