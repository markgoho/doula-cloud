/**
 * A date of birth as three boxes, the way a person remembers one --
 * GOV.UK's Dates pattern, and #466's replacement for the single
 * `type="date"` control intake carried before it.
 *
 * ## Why this is not a component's private business
 *
 * The composition rule is Postel's Law, written out on #466: one or two
 * digits for the month and the day, two or four for the year, all
 * accepted and normalised on the way to storage. That is a decision with
 * edge cases -- a two-digit year has to be resolved against something,
 * and 30 February parses as 2 March if nobody stops it -- so it lives in
 * a module a unit test can exercise directly, and `DateFields.svelte`
 * only renders boxes.
 *
 * ## The two-digit year
 *
 * `24` means 2024 and `76` means 1976, resolved on a sliding window: a
 * two-digit year at or below the current year's own last two digits is
 * this century, anything above it is the last one. A Client born in
 * 2030 does not exist yet, and one born in 1930 does, so the window
 * errs the only way that is ever right today.
 */

/** The three boxes, exactly as typed. Strings rather than numbers: a
 * half-typed `0` is a real intermediate state, and a control that
 * silently turns `07` into `7` under the cursor is the formatting
 * defect this pattern exists to avoid. */
export interface DateParts {
	month: string;
	day: string;
	year: string;
}

export const EMPTY_DATE_PARTS: DateParts = { month: '', day: '', year: '' };

/** Splits a stored `"YYYY-MM-DD"` back into the three boxes, so a value
 * carried in from the search that fronts intake (#498) is shown as it
 * will be edited. Anything that is not that shape -- including the empty
 * string -- comes back as three empty boxes rather than throwing: this
 * reads a value, it does not police one. */
export function splitDate(value: string): DateParts {
	const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value.trim());
	if (match === null) return { ...EMPTY_DATE_PARTS };
	return { month: match[2]!, day: match[3]!, year: match[1]! };
}

/** Which box a refusal belongs to, so the error summary's `#id` link
 * lands on the box that has to change rather than on the group. */
export type DateField = 'month' | 'day' | 'year';

export type DateResult =
	| { ok: true; value: string }
	| { ok: false; message: string; field: DateField };

const DIGITS = /^\d+$/;

function resolveYear(typed: string, today: Date): number {
	const year = Number(typed);
	if (typed.length === 4) return year;
	const currentTwoDigit = today.getFullYear() % 100;
	const century = year <= currentTwoDigit ? 2000 : 1900;
	return century + year;
}

function pad(value: number): string {
	return String(value).padStart(2, '0');
}

/**
 * Composes the three boxes into the `"YYYY-MM-DD"` string
 * `client.Record.DateOfBirth` expects.
 *
 * Three boxes left blank is not a refusal -- ADR-0017 requires only a
 * given name, and #466 made the save free -- so it composes to the empty
 * string, which is what the endpoint stores as "not asked yet".
 *
 * `label` is the field's own noun, so the message can start with it the
 * way GOV.UK's error rules ask ("Date of birth must be a real date").
 * `today` is injected rather than read, so the sliding two-digit window
 * is testable without waiting a year.
 */
export function joinDate(
	parts: DateParts,
	label = 'Date of birth',
	today: Date = new Date()
): DateResult {
	const month = parts.month.trim();
	const day = parts.day.trim();
	const year = parts.year.trim();

	if (month === '' && day === '' && year === '') return { ok: true, value: '' };

	const typedParts: [DateField, string][] = [
		['month', month],
		['day', day],
		['year', year]
	];
	for (const [field, typed] of typedParts) {
		if (typed === '') return { ok: false, message: `${label} must include a ${field}`, field };
	}

	for (const [field, typed] of typedParts) {
		const widths = field === 'year' ? [2, 4] : [1, 2];
		if (!DIGITS.test(typed) || !widths.includes(typed.length)) {
			return { ok: false, message: `${label} must be a real date`, field };
		}
	}

	const monthNumber = Number(month);
	const dayNumber = Number(day);
	const yearNumber = resolveYear(year, today);

	/*
	 * Round-tripping through Date is what catches 31 April and 29
	 * February in a common year: the constructor rolls an out-of-range
	 * day into the next month rather than refusing it, so the only
	 * reliable check is whether the parts come back out unchanged.
	 */
	const composed = new Date(yearNumber, monthNumber - 1, dayNumber);
	const isReal =
		composed.getFullYear() === yearNumber &&
		composed.getMonth() === monthNumber - 1 &&
		composed.getDate() === dayNumber;
	if (!isReal) {
		return {
			ok: false,
			message: `${label} must be a real date`,
			field: monthNumber < 1 || monthNumber > 12 ? 'month' : 'day'
		};
	}

	if (composed.getTime() > today.getTime()) {
		return { ok: false, message: `${label} must be in the past`, field: 'year' };
	}

	return {
		ok: true,
		value: `${String(yearNumber).padStart(4, '0')}-${pad(monthNumber)}-${pad(dayNumber)}`
	};
}
