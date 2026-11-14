/**
 * Two calendar days a reader narrows a list by, and everything a screen
 * needs to say, step and refuse them: the Schedule (`visitSchedule.ts`) and
 * On call (`onCall.ts`) both read a list "from Fri Nov 13 to Sat Dec 12".
 *
 * A calendar day is `YYYY-MM-DD` and has no zone, so counting in days is
 * arithmetic on its own parts. Only "today" is read in the Practice's zone
 * (ADR-0036). Generic over the presets a screen offers: a view names its
 * preset as a number of days counted from today.
 */
import { formatCalendarWeekdayDay } from './dates.js';
import { dayIn } from './zonedTime.js';

/**
 * Two calendar days, both included.
 */
export interface Days {
	from: string;
	to: string;
}

export interface RangeErrors {
	from?: string;
	to?: string;
}

/**
 * What an address says about its days before any day is worked out: a
 * preset of N days counted from today, or two typed days that may be empty
 * or not days.
 */
export interface DaysView {
	preset: number;
	chosen?: Days;
}

export interface RangeLimits {
	/**
	The first day a range may start on, with the words said on From when it starts before.
	*/
	earliest?: { day: string; message: string };
	/**
	The most days a range may hold, with the words said on To when it holds more.
	*/
	longest?: { days: number; message: string };
}

export const ENTER_FROM_DAY = 'Enter a From day';
export const ENTER_TO_DAY = 'Enter a To day';
export const TO_BEFORE_FROM = 'Choose a To day that is the same as the From day or later.';

const DAY_MS = 86_400_000;

function dayOfUtc(date: Date): string {
	return date.toISOString().slice(0, 10);
}

/** The calendar day `days` after the given one. A day in a zone is 25 hours
 * long when daylight saving ends and 23 when it starts, so counting hours
 * from a midnight lands a day off. */
export function addDays(day: string, days: number): string {
	const [year, month, date] = day.split('-').map(Number);
	return dayOfUtc(new Date(Date.UTC(year, month - 1, date + days)));
}

/**
Whether a string is a real calendar day in `YYYY-MM-DD` form.
*/
export function isCalendarDay(value: string): boolean {
	return /^\d{4}-\d{2}-\d{2}$/.test(value) && addDays(value, 0) === value;
}

/**
How many calendar days a range holds, both ends included.
*/
export function daysIn(days: Days): number {
	const [fromYear, fromMonth, fromDay] = days.from.split('-').map(Number);
	const [toYear, toMonth, toDay] = days.to.split('-').map(Number);
	return Math.round((Date.UTC(toYear!, toMonth! - 1, toDay!) - Date.UTC(fromYear!, fromMonth! - 1, fromDay!)) / DAY_MS) + 1;
}

/**
Why two typed days cannot be a range, in the words each field says. Only a pair that is already in order is held to `limits`.
*/
export function rangeErrors(from: string, to: string, limits: RangeLimits = {}): RangeErrors {
	const errors: RangeErrors = {};
	if (!isCalendarDay(from)) errors.from = ENTER_FROM_DAY;
	else if (limits.earliest && from < limits.earliest.day) errors.from = limits.earliest.message;
	if (!isCalendarDay(to)) errors.to = ENTER_TO_DAY;
	else if (!errors.from && to < from) errors.to = TO_BEFORE_FROM;
	else if (!errors.from && limits.longest && daysIn({ from, to }) > limits.longest.days) errors.to = limits.longest.message;
	return errors;
}

/** The days a view reads: a preset of N days is today through today plus
 * N - 1, the last day included, so a preset of 1 is today alone. Chosen days
 * that are not a range carry their errors instead. */
export function daysOf(
	view: DaysView,
	timeZone: string,
	now: Date = new Date(),
	limits?: RangeLimits
): { days: Days; errors?: undefined } | { days?: undefined; errors: RangeErrors } {
	if (view.chosen) {
		const errors = rangeErrors(view.chosen.from, view.chosen.to, limits);
		return Object.keys(errors).length > 0 ? { errors } : { days: view.chosen };
	}
	const from = dayIn(now, timeZone);
	return { days: { from, to: addDays(from, view.preset - 1) } };
}

export interface DaysStep<V extends DaysView> {
	/**
	The view a step lands on: a preset when the days are one counted from today, chosen days otherwise.
	*/
	view: V;
	days: Days;
}

/**
 * The days one step earlier (`-1`) or later (`1`): the days the reader sees
 * moved by their own length. A step lands on a preset of `presets` when its
 * days are that preset counted from today. With a `floor`, an earlier step
 * that would start before it starts on it with the same length, and there is
 * none from the floor itself. Nothing where the days are not a range.
 */
export function stepDays<V extends DaysView>(
	view: V,
	direction: -1 | 1,
	timeZone: string,
	now: Date,
	presets: readonly number[],
	floor?: string,
	limits?: RangeLimits
): DaysStep<V> | undefined {
	const shown = daysOf(view, timeZone, now, limits).days;
	if (!shown) return undefined;
	const length = daysIn(shown);
	let from = addDays(shown.from, length * direction);
	if (floor !== undefined && direction === -1) {
		if (shown.from <= floor) return undefined;
		if (from < floor) from = floor;
	}
	const days = { from, to: addDays(from, length - 1) };
	if (days.from === dayIn(now, timeZone)) {
		for (const preset of presets) {
			if (days.to === addDays(days.from, preset - 1)) return { view: { ...view, preset, chosen: undefined }, days };
		}
	}
	return { view: { ...view, chosen: days }, days };
}

/**
The days as a reader says them: "Fri Nov 6 to Thu Nov 12", or one day alone.
*/
export function daysWords(days: Days, timeZone: string, now: Date = new Date()): string {
	const from = formatCalendarWeekdayDay(days.from, timeZone, now);
	return days.from === days.to ? from : `${from} to ${formatCalendarWeekdayDay(days.to, timeZone, now)}`;
}

/**
The applied tag for chosen days: "Days: Tue Nov 10 to Thu Nov 19".
*/
export function daysTag(days: Days, timeZone: string, now: Date = new Date()): string {
	return `Days: ${daysWords(days, timeZone, now)}`;
}

/**
 * Where a sentence says the days are: "from Fri Nov 13 to Sat Dec 12", or
 * "on Mon Nov 16" when the first day is the last. A day's spaces do not
 * break, so a line wraps between days and never inside one.
 */
export function daysSpan(days: Days, timeZone: string, now: Date = new Date()): string {
	const whole = (day: string) => formatCalendarWeekdayDay(day, timeZone, now).replaceAll(' ', '\u{A0}');
	return days.from === days.to ? `on ${whole(days.from)}` : `from ${whole(days.from)} to ${whole(days.to)}`;
}
