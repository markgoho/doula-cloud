const LONG = { month: 'long', day: 'numeric', year: 'numeric' } as const;

/**
A calendar day, YYYY-MM-DD, as a person reads it: October 30, 2026. No zone moves it.
*/
export function longDate(isoDate: string): string {
	const [year, month, day] = isoDate.split('-').map(Number);
	return new Date(Date.UTC(year, month - 1, day)).toLocaleDateString('en-US', { ...LONG, timeZone: 'UTC' });
}

/**
The day an instant falls on in New York, where the business keeps its books.
*/
export function newYorkDay(instant: string): string {
	return new Date(instant).toLocaleDateString('en-US', { ...LONG, timeZone: 'America/New_York' });
}
