/**
 * Reading an instant in a named IANA zone (ADR-0036): a Practice's days and
 * times are its own, never the reader's, so a screen that shows a Visit's day
 * asks which zone to read it in. Nothing here reads a global zone; the
 * reader's own is `readerTimeZone`, named only where a caller compares it to
 * the Practice's.
 */

/**
 * The zone this browser is in, as an IANA name.
 */
export function readerTimeZone(): string {
	return new Intl.DateTimeFormat().resolvedOptions().timeZone;
}

export interface ZonedParts {
	weekday: string;
	/**
	The short month name, "Nov".
	*/
	month: string;
	day: number;
	year: number;
	/**
	On a 24-hour clock, 0 to 23.
	*/
	hour: number;
	minute: number;
	second: number;
}

const formatters = new Map<string, Intl.DateTimeFormat>();

function formatterIn(timeZone: string): Intl.DateTimeFormat {
	let formatter = formatters.get(timeZone);
	if (!formatter) {
		formatter = new Intl.DateTimeFormat('en-US', {
			timeZone,
			weekday: 'short',
			month: 'short',
			day: 'numeric',
			year: 'numeric',
			hour: 'numeric',
			minute: 'numeric',
			second: 'numeric',
			hourCycle: 'h23'
		});
		formatters.set(timeZone, formatter);
	}
	return formatter;
}

function partsOfWall(at: Date, timeZone: string): Record<string, string> {
	const parts: Record<string, string> = {};
	for (const part of formatterIn(timeZone).formatToParts(at)) parts[part.type] = part.value;
	return parts;
}

/**
 * The wall clock `at` shows in `timeZone`.
 */
export function partsIn(at: Date, timeZone: string): ZonedParts {
	const parts = partsOfWall(at, timeZone);
	return {
		weekday: parts.weekday!,
		month: parts.month!,
		day: Number(parts.day),
		year: Number(parts.year),
		hour: Number(parts.hour),
		minute: Number(parts.minute),
		second: Number(parts.second)
	};
}

/**
 * How far `timeZone`'s wall clock is ahead of UTC at `at`, in milliseconds.
 */
function offsetIn(at: Date, timeZone: string): number {
	const parts = partsOfWall(at, timeZone);
	const wall = Date.UTC(
		Number(parts.year),
		MONTHS.indexOf(parts.month!),
		Number(parts.day),
		Number(parts.hour),
		Number(parts.minute),
		Number(parts.second)
	);
	return wall - Math.floor(at.getTime() / 1000) * 1000;
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

const padTwoDigits = (n: number) => n.toString().padStart(2, '0');

/**
 * The calendar day `at` falls on in `timeZone`, `YYYY-MM-DD`.
 */
export function dayIn(at: Date, timeZone: string): string {
	const { year, month, day } = partsOfWall(at, timeZone);
	return `${year}-${padTwoDigits(MONTHS.indexOf(month!) + 1)}-${padTwoDigits(Number(day))}`;
}

/**
 * The instant a `YYYY-MM-DD` day begins in `timeZone`. A day there is 23 or 25
 * hours long twice a year, so the start is found from the zone's offset at
 * that start, not by adding a fixed 24 hours to the day before.
 */
export function startOfDayIn(day: string, timeZone: string): Date {
	const [year, month, date] = day.split('-').map(Number);
	const wall = Date.UTC(year!, month! - 1, date!);
	let start = wall - offsetIn(new Date(wall), timeZone);
	start = wall - offsetIn(new Date(start), timeZone);
	return new Date(start);
}

/**
 * The instant a wall clock, `YYYY-MM-DDTHH:mm` as a datetime-local control
 * reports it, shows in `timeZone`. A wall time that does not exist (the hour
 * a spring-forward day skips) reads as the offset before the change, so it
 * lands an hour later on the clock; one that happens twice (the hour a
 * fall-back day repeats) is the first of the two. An unparseable value is an
 * Invalid Date, as `new Date` would give.
 */
export function instantFromWall(local: string, timeZone: string): Date {
	const wall = Date.parse(`${local}Z`);
	if (Number.isNaN(wall)) return new Date(NaN);
	const before = offsetIn(new Date(wall - DAY_MS), timeZone);
	const after = offsetIn(new Date(wall + DAY_MS), timeZone);
	const candidates = [wall - before, wall - after].filter(
		(at, index) => offsetIn(new Date(at), timeZone) === (index === 0 ? before : after)
	);
	return new Date(candidates.length === 0 ? wall - before : Math.min(...candidates));
}

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * Whether two zones read `at` differently: another UTC offset then, or another
 * name for the zone.
 */
export function areZonesApart(a: string, b: string, at: Date): boolean {
	return offsetIn(at, a) !== offsetIn(at, b) || zoneName(a, at) !== zoneName(b, at);
}

/**
 * The zone as a person says it, "Eastern Time"; the IANA name when the
 * runtime has no such name.
 */
export function zoneName(timeZone: string, at: Date): string {
	const name = new Intl.DateTimeFormat('en-US', { timeZone, timeZoneName: 'longGeneric' })
		.formatToParts(at)
		.find((part) => part.type === 'timeZoneName')?.value;
	return name ?? timeZone;
}

/**
 * The one line that says which zone the times are in, for a reader whose own
 * is another: "Times are in this Practice's time zone, Eastern Time." Nothing
 * for a reader in the Practice's zone.
 */
export function zoneNote(practiceZone: string, readerZone: string, now: Date): string | undefined {
	if (!areZonesApart(practiceZone, readerZone, now)) return undefined;
	return `Times are in this Practice's time zone, ${zoneName(practiceZone, now)}.`;
}
