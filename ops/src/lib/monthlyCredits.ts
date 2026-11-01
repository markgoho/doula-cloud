import { refusal, type Outcome } from './act.ts';

type Month = string;

export type PracticeMonth = {
	id: string;
	name: string;
	creditsBought: number;
	creditsSpent: number;
	boughtBefore: boolean;
};

export type MonthlyCredits = {
	month: Month;
	months: Month[];
	practices: PracticeMonth[];
	totals: { creditsBought: number; creditsSpent: number; practicesBoughtAgain: number };
};

export function monthName(month: Month): string {
	const [year, number] = month.split('-').map(Number);
	return new Date(Date.UTC(year, number - 1, 1)).toLocaleDateString('en-US', {
		timeZone: 'UTC',
		month: 'long',
		year: 'numeric'
	});
}

export async function readMonthlyCredits(
	month: Month | undefined,
	fetcher: typeof fetch = fetch
): Promise<Outcome<MonthlyCredits>> {
	const query = month ? `?month=${encodeURIComponent(month)}` : '';
	const response = await fetcher(`/api/credits/monthly${query}`, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
