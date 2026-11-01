import { refusal, type Outcome } from './act.ts';
import { credits } from './foundingGrant.ts';
import { day } from './practices.ts';

type Instant = string;

export type DormantPractice = {
	id: string;
	name: string;
	signedUpAt: Instant;
	balance: number;
	lastContactAt: Instant | null;
};

export type DormantPage = { items: DormantPractice[]; nextCursor?: string; hasMore: boolean };

/**
Each field the dormant list shows for one Practice, as a summary list's rows.
*/
export function dormantFacts(row: DormantPractice): { key: string; value: string }[] {
	return [
		{ key: 'Signed up', value: day(row.signedUpAt) },
		{ key: 'Credits held', value: credits(row.balance) },
		{ key: 'Last contact', value: row.lastContactAt ? day(row.lastContactAt) : 'Never' }
	];
}

/**
One page of the dormant list, newest signup first, after cursor.
*/
export async function readDormantPractices(
	cursor: string | undefined,
	fetcher: typeof fetch = fetch
): Promise<Outcome<DormantPage>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher('/api/practices/dormant' + query, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
