import { describe, expect, it } from 'vitest';
import { joinDate, splitDate, type DateParts } from './intakeDate';

const today = new Date(2026, 8, 5);

function parts(month: string, day: string, year: string): DateParts {
	return { month, day, year };
}

describe('splitDate', () => {
	it('splits a stored date into month, day and year', () => {
		expect(splitDate('1988-02-09')).toEqual({ month: '02', day: '09', year: '1988' });
	});

	it('reads a blank value as three blank boxes', () => {
		expect(splitDate('')).toEqual({ month: '', day: '', year: '' });
	});

	it('reads anything that is not a stored date as three blank boxes', () => {
		expect(splitDate('9 Feb 1988')).toEqual({ month: '', day: '', year: '' });
	});
});

describe('joinDate', () => {
	it('composes three boxes into the stored shape', () => {
		expect(joinDate(parts('02', '09', '1988'), 'Date of birth', today)).toEqual({
			ok: true,
			value: '1988-02-09'
		});
	});

	// Postel's Law, #466: a field that refuses a real answer over
	// formatting is a defect.
	it('accepts one-digit month and day and pads them', () => {
		expect(joinDate(parts('2', '9', '1988'), 'Date of birth', today)).toEqual({
			ok: true,
			value: '1988-02-09'
		});
	});

	it('reads a two-digit year at or below this year as this century', () => {
		expect(joinDate(parts('2', '9', '26'), 'Date of birth', today)).toEqual({
			ok: true,
			value: '2026-02-09'
		});
	});

	it('reads a two-digit year above this year as the last century', () => {
		expect(joinDate(parts('2', '9', '88'), 'Date of birth', today)).toEqual({
			ok: true,
			value: '1988-02-09'
		});
	});

	// ADR-0017: only a given name is required, so an unanswered date of
	// birth is an empty value rather than a refusal.
	it('composes three blank boxes to the empty string', () => {
		expect(joinDate(parts('', '', ''), 'Date of birth', today)).toEqual({ ok: true, value: '' });
	});

	it('trims what was typed before reading it', () => {
		expect(joinDate(parts(' 2 ', ' 9 ', ' 1988 '), 'Date of birth', today)).toEqual({
			ok: true,
			value: '1988-02-09'
		});
	});

	it.each([
		['month', parts('', '9', '1988')],
		['day', parts('2', '', '1988')],
		['year', parts('2', '9', '')]
	])('names the one box left blank: %s', (field, typed) => {
		expect(joinDate(typed, 'Date of birth', today)).toEqual({
			ok: false,
			message: `Date of birth must include a ${field}`,
			field
		});
	});

	it.each([
		['a non-numeric month', parts('Feb', '9', '1988'), 'month'],
		['a three-digit year', parts('2', '9', '198'), 'year'],
		['a three-digit day', parts('2', '009', '1988'), 'day']
	])('refuses %s', (_label, typed, field) => {
		expect(joinDate(typed, 'Date of birth', today)).toEqual({
			ok: false,
			message: 'Date of birth must be a real date',
			field
		});
	});

	it('refuses a day that does not exist in that month', () => {
		expect(joinDate(parts('2', '30', '1988'), 'Date of birth', today)).toEqual({
			ok: false,
			message: 'Date of birth must be a real date',
			field: 'day'
		});
	});

	it('blames the month when the month is the impossible part', () => {
		expect(joinDate(parts('13', '9', '1988'), 'Date of birth', today)).toEqual({
			ok: false,
			message: 'Date of birth must be a real date',
			field: 'month'
		});
	});

	it('accepts 29 February in a leap year', () => {
		expect(joinDate(parts('2', '29', '1988'), 'Date of birth', today)).toEqual({
			ok: true,
			value: '1988-02-29'
		});
	});

	it('refuses a date that has not happened yet', () => {
		expect(joinDate(parts('12', '25', '2026'), 'Date of birth', today)).toEqual({
			ok: false,
			message: 'Date of birth must be in the past',
			field: 'year'
		});
	});

	it('names the field it was given', () => {
		const refused = joinDate(parts('', '9', '1988'), 'Due date', today);
		expect(refused).toEqual({
			ok: false,
			message: 'Due date must include a month',
			field: 'month'
		});
	});

	it('reads today as a date in the past rather than the future', () => {
		expect(joinDate(parts('9', '5', '2026'), 'Date of birth', today)).toEqual({
			ok: true,
			value: '2026-09-05'
		});
	});

	// The default `today` is the real clock -- exercised so the parameter's
	// default is not an untested branch.
	it('reads the clock when no date is handed in', () => {
		expect(joinDate(parts('2', '9', '1988'))).toEqual({ ok: true, value: '1988-02-09' });
	});

	// GOV.UK's error wording rules, asserted here because the banned-word
	// scan reads only .svelte files.
	it('says none of the banned words GOV.UK names', () => {
		const messages = [
			joinDate(parts('', '9', '1988'), 'Date of birth', today),
			joinDate(parts('13', '9', '1988'), 'Date of birth', today),
			joinDate(parts('12', '25', '2026'), 'Date of birth', today)
		]
			.filter((result) => !result.ok)
			.map((result) => result.message);

		expect(messages).toHaveLength(3);
		for (const message of messages) {
			expect(message).not.toMatch(/\b(please|valid|invalid|required)\b/i);
		}
	});
});
