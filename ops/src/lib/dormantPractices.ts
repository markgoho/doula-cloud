import { refusal, type Outcome } from './act.ts';
import { credits } from './foundingGrant.ts';
import { newYorkDay } from './dates.ts';

type Instant = string;

export type DormantPractice = {
	id: string;
	name: string;
	signedUpAt: Instant;
	balance: number;
	lastContactAt: Instant | null;
};

export type DormantPage = { items: DormantPractice[]; nextCursor?: string; hasMore: boolean };

export function dormantFacts(row: DormantPractice): { label: string; value: string }[] {
	return [
		{ label: 'Signed up', value: newYorkDay(row.signedUpAt) },
		{ label: 'Credits held', value: credits(row.balance) },
		{ label: 'Last contact', value: row.lastContactAt ? newYorkDay(row.lastContactAt) : 'Never' }
	];
}

export async function readDormantPractices(
	cursor: string | undefined,
	fetcher: typeof fetch = fetch
): Promise<Outcome<DormantPage>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher('/api/practices/dormant' + query, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
