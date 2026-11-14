import { formatCalendarDay } from './dates.js';
import { isCalendarDay } from './visitSchedule.js';
import { dayIn } from './zonedTime.js';

export type InvoicePreset = 'this-month' | 'last-month' | 'this-year' | 'last-year';

const PRESETS: readonly InvoicePreset[] = ['this-month', 'last-month', 'this-year', 'last-year'];

export type InvoiceDays = { from: string; to: string };

export type InvoicePeriod = { preset: InvoicePreset } | { days: InvoiceDays };

export type InvoicePeriodChoice = '' | InvoicePreset | 'choose';

export const INVOICE_PERIOD_OPTIONS: readonly { value: InvoicePeriodChoice; label: string }[] = [
	{ value: '', label: 'Any time' },
	{ value: 'this-month', label: 'This month' },
	{ value: 'last-month', label: 'Last month' },
	{ value: 'this-year', label: 'This year' },
	{ value: 'last-year', label: 'Last year' },
	{ value: 'choose', label: 'Choose days' }
];

/**
 * The period an address names (#2871): `?sent=last-month`, or two days,
 * `?sentFrom=2026-10-03&sentTo=2026-10-17`. The no-script form posts its
 * Choose days pair as `sent=choose&from=&to=`. A name or a pair of days that
 * is not a period is no period, so an address never asks for what the
 * BFF would refuse.
 */
export function invoicePeriodOf(search: Pick<URLSearchParams, 'get'>): InvoicePeriod | undefined {
	const sent = search.get('sent');
	const preset = PRESETS.find((name) => name === sent);
	if (preset) return { preset };
	const from = search.get(sent === 'choose' ? 'from' : 'sentFrom') ?? '';
	const to = search.get(sent === 'choose' ? 'to' : 'sentTo') ?? '';
	return isCalendarDay(from) && isCalendarDay(to) && from <= to ? { days: { from, to } } : undefined;
}

/**
 * The period as the address says it: a preset's name, or the two days. Equal
 * for equal periods, and empty for none, so it keys a read.
 */
export function invoicePeriodKey(period: InvoicePeriod | undefined): string {
	if (!period) return '';
	return 'preset' in period ? period.preset : `${period.days.from}..${period.days.to}`;
}

export function invoicePeriodQuery(period: InvoicePeriod | undefined): [string, string][] {
	if (!period) return [];
	if ('preset' in period) return [['sent', period.preset]];
	return [
		['sentFrom', period.days.from],
		['sentTo', period.days.to]
	];
}

const pad = (value: number) => String(value).padStart(2, '0');

const lastDayOfMonth = (year: number, month: number) =>
	`${year}-${pad(month)}-${pad(new Date(Date.UTC(year, month, 0)).getUTCDate())}`;

/**
 * The two days a period covers in the Practice's zone: the BFF takes days,
 * never a name, and a preset is counted from today there.
 */
export function invoicePeriodDays(period: InvoicePeriod | undefined, timeZone: string, now: Date): InvoiceDays | undefined {
	if (!period) return undefined;
	if ('days' in period) return period.days;
	const [year, month] = dayIn(now, timeZone).split('-').map(Number) as [number, number];
	switch (period.preset) {
		case 'this-month': {
			return { from: `${year}-${pad(month)}-01`, to: lastDayOfMonth(year, month) };
		}
		case 'last-month': {
			const [priorYear, priorMonth] = month === 1 ? [year - 1, 12] : [year, month - 1];
			return { from: `${priorYear}-${pad(priorMonth)}-01`, to: lastDayOfMonth(priorYear, priorMonth) };
		}
		case 'this-year': {
			return { from: `${year}-01-01`, to: `${year}-12-31` };
		}
		case 'last-year': {
			return { from: `${year - 1}-01-01`, to: `${year - 1}-12-31` };
		}
	}
}

const MONTH_NAMES = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

const withoutYear = (day: string) => formatCalendarDay(day).replace(/, \d{4}$/, '');

/**
 * The days in fixed words, never relative ones, so a tag never reads wrong
 * next month: "in October 2026", "in 2026", "Oct 3 to Oct 17, 2026". A
 * range that is a whole month or year reads as one.
 */
export function invoicePeriodWords({ from, to }: InvoiceDays): string {
	const [fromYear, fromMonth] = from.split('-', 2);
	const isWholeYear = from === `${fromYear}-01-01` && to === `${fromYear}-12-31`;
	if (isWholeYear) return `in ${fromYear}`;
	const isWholeMonth = to.startsWith(`${fromYear}-${fromMonth}-`) && from.endsWith('-01') && to === lastDayOfMonth(Number(fromYear), Number(fromMonth));
	if (isWholeMonth) return `in ${MONTH_NAMES[Number(fromMonth) - 1]} ${fromYear}`;
	if (from === to) return `on ${formatCalendarDay(from)}`;
	const fromWords = fromYear === to.slice(0, 4) ? withoutYear(from) : formatCalendarDay(from);
	return `${fromWords} to ${formatCalendarDay(to)}`;
}

/**
 * The longest wording a period can say, for the result line's ruler.
 */
export const INVOICE_PERIOD_RULER: InvoiceDays = { from: '2026-09-30', to: '2027-12-31' };

/**
 * What the select shows for a period.
 */
export function invoicePeriodChoiceOf(period: InvoicePeriod | undefined): InvoicePeriodChoice {
	if (!period) return '';
	return 'preset' in period ? period.preset : 'choose';
}
