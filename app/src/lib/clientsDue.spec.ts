import { describe, expect, it } from 'vitest';
import {
	ANY_DUE,
	clientsDueFrom,
	dueAddressParts,
	dueMonthDays,
	dueMonthName,
	dueMonthOptions,
	dueTag,
	dueWhen,
	isDueApplied
} from './clientsDue.js';

const parameters = (query: string) => new URLSearchParams(query);

describe('clientsDueFrom (#2871)', () => {
	it('reads a month', () => {
		expect(clientsDueFrom(parameters('due=2027-03'))).toEqual({ month: '2027-03', from: '', to: '' });
	});

	it('reads two days, both included', () => {
		expect(clientsDueFrom(parameters('dueFrom=2027-03-01&dueTo=2027-03-15'))).toEqual({ month: '', from: '2027-03-01', to: '2027-03-15' });
		expect(clientsDueFrom(parameters('dueFrom=2027-03-15&dueTo=2027-03-15'))).toEqual({ month: '', from: '2027-03-15', to: '2027-03-15' });
	});

	it('prefers the month to days beside it, which the BFF would refuse together', () => {
		expect(clientsDueFrom(parameters('due=2027-03&dueFrom=2027-03-01&dueTo=2027-03-15')).month).toBe('2027-03');
	});

	it.each([
		['no month or days', ''],
		['a month with no number', 'due=March'],
		['a month 13', 'due=2027-13'],
		['a one-digit month', 'due=2027-3'],
		['From without To', 'dueFrom=2027-03-01'],
		['To without From', 'dueTo=2027-03-01'],
		['a day that is not a day', 'dueFrom=2027-02-30&dueTo=2027-03-01'],
		['To before From', 'dueFrom=2027-03-15&dueTo=2027-03-01']
	])('reads %s as no narrowing, which the BFF would refuse', (_name, query) => {
		const read = clientsDueFrom(parameters(query));
		expect(read).toBe(ANY_DUE);
		expect(isDueApplied(read)).toBe(false);
	});
});

describe('the due address and words', () => {
	it('writes a month, or both days, or nothing', () => {
		expect(dueAddressParts({ month: '2027-03', from: '', to: '' })).toEqual(['due=2027-03']);
		expect(dueAddressParts({ month: '', from: '2027-03-01', to: '2027-03-15' })).toEqual(['dueFrom=2027-03-01', 'dueTo=2027-03-15']);
		expect(dueAddressParts(ANY_DUE)).toEqual([]);
	});

	it('names a month and finds its first and last day, in a leap year too', () => {
		expect(dueMonthName('2027-03')).toBe('March 2027');
		expect(dueMonthDays('2027-03')).toEqual({ from: '2027-03-01', to: '2027-03-31' });
		expect(dueMonthDays('2027-02').to).toBe('2027-02-28');
		expect(dueMonthDays('2028-02').to).toBe('2028-02-29');
	});

	it('offers this month and the next nine, across the year end', () => {
		const options = dueMonthOptions('2026-11-14');
		expect(options).toHaveLength(10);
		expect(options[0]).toEqual({ value: '2026-11', label: 'November 2026' });
		expect(options[2]).toEqual({ value: '2027-01', label: 'January 2027' });
		expect(options[9]).toEqual({ value: '2027-08', label: 'August 2027' });
	});

	it('says when in fixed words: a month, the days of one year, days across two, one day', () => {
		expect(dueWhen({ month: '2027-03', from: '', to: '' })).toBe('in March 2027');
		expect(dueTag({ month: '2027-03', from: '', to: '' })).toBe('Due in March 2027');
		expect(dueTag({ month: '', from: '2027-03-01', to: '2027-03-15' })).toBe('Due Mar 1 to Mar 15, 2027');
		expect(dueTag({ month: '', from: '2026-12-20', to: '2027-01-05' })).toBe('Due Dec 20, 2026 to Jan 5, 2027');
		expect(dueTag({ month: '', from: '2027-03-01', to: '2027-03-01' })).toBe('Due Mar 1, 2027');
	});
});
