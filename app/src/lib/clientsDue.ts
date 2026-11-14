/*
 * The Clients list's due-month narrowing (#2871): the address, the select's
 * choices and the words the tag and the result line say.
 *
 * A due date is a calendar day with no zone, so a month is named by its
 * `YYYY-MM` and a range by its two days; nothing here reads a clock but the
 * options, which take today's day as a parameter.
 */
import { formatCalendarDay } from './dates.js';
import { isCalendarDay } from './visitSchedule.js';
import type { LabeledValue } from './roles.js';

/**
 * Either a month, `YYYY-MM`, or two days, inclusive; every part empty is "any time".
 */
export interface ClientsDue {
	month: string;
	from: string;
	to: string;
}

export const ANY_DUE: ClientsDue = { month: '', from: '', to: '' };

export const CHOOSE_DAYS = 'choose';

const MONTH_PATTERN = /^\d{4}-(0[1-9]|1[0-2])$/;
const MONTH_NAMES = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
// This month and the next nine: a pregnancy's reach.
const MONTHS_OFFERED = 10;

/**
 * What an address names. A month or a range the BFF would refuse is no narrowing at all.
 */
export function clientsDueFrom(parameters: Pick<URLSearchParams, 'get'>): ClientsDue {
	const month = parameters.get('due') ?? '';
	if (MONTH_PATTERN.test(month)) return { month, from: '', to: '' };
	const from = parameters.get('dueFrom') ?? '';
	const to = parameters.get('dueTo') ?? '';
	if (isCalendarDay(from) && isCalendarDay(to) && from <= to) return { month: '', from, to };
	return ANY_DUE;
}

export function isDueApplied({ month, from }: ClientsDue): boolean {
	return month !== '' || from !== '';
}

/**
 * The address's parts for a narrowing.
 */
export function dueAddressParts({ month, from, to }: ClientsDue): string[] {
	if (month) return [`due=${month}`];
	return from ? [`dueFrom=${from}`, `dueTo=${to}`] : [];
}

/**
 * "March 2027".
 */
export function dueMonthName(month: string): string {
	const [year, number] = month.split('-', 2);
	return `${MONTH_NAMES[Number(number) - 1]} ${year}`;
}

/**
 * The first and the last day of a month.
 */
export function dueMonthDays(month: string): { from: string; to: string } {
	const [year, number] = month.split('-').map(Number);
	const last = new Date(Date.UTC(year!, number!, 0)).getUTCDate();
	return { from: `${month}-01`, to: `${month}-${String(last).padStart(2, '0')}` };
}

/**
 * The select's months, by name, from the one `today` (`YYYY-MM-DD`) is in.
 */
export function dueMonthOptions(today: string): LabeledValue[] {
	const [year, number] = today.split('-').map(Number);
	return Array.from({ length: MONTHS_OFFERED }, (_, index) => {
		const at = new Date(Date.UTC(year!, number! - 1 + index, 1));
		const month = `${at.getUTCFullYear()}-${String(at.getUTCMonth() + 1).padStart(2, '0')}`;
		return { value: month, label: dueMonthName(month) };
	});
}

// "Mar 1 to Mar 15, 2027": the year once where both days share it.
function rangeWords(from: string, to: string): string {
	if (from === to) return formatCalendarDay(from);
	const first = formatCalendarDay(from);
	const last = formatCalendarDay(to);
	const isOneYear = from.slice(0, 4) === to.slice(0, 4);
	return `${isOneYear ? first.replace(/, \d{4}$/, '') : first} to ${last}`;
}

/**
 * When, in fixed words that never read wrong next month: "in March 2027", "Mar 1 to Mar 15, 2027".
 */
export function dueWhen({ month, from, to }: ClientsDue): string {
	return month ? `in ${dueMonthName(month)}` : rangeWords(from, to);
}

/**
 * The applied tag: "Due in March 2027".
 */
export function dueTag(due: ClientsDue): string {
	return `Due ${dueWhen(due)}`;
}
