const LONG = { month: 'long', day: 'numeric', year: 'numeric' } as const;

export function longDate(isoDate: string): string {
	const [year, month, day] = isoDate.split('-').map(Number);
	return new Date(Date.UTC(year, month - 1, day)).toLocaleDateString('en-US', { ...LONG, timeZone: 'UTC' });
}

export function newYorkDay(instant: string): string {
	return new Date(instant).toLocaleDateString('en-US', { ...LONG, timeZone: 'America/New_York' });
}
