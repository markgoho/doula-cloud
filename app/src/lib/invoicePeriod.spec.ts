import { describe, expect, it } from 'vitest';
import {
	invoicePeriodChoiceOf,
	invoicePeriodDays,
	invoicePeriodKey,
	invoicePeriodOf,
	invoicePeriodQuery,
	invoicePeriodWords
} from './invoicePeriod.js';
import { invoiceTotalsNote, invoicesResultSummary, invoicesViewQuery } from './invoice.js';

const parse = (query: string) => invoicePeriodOf(new URLSearchParams(query));

describe('invoicePeriodOf', () => {
	it.each([
		['sent=this-month', { preset: 'this-month' }],
		['sent=last-year', { preset: 'last-year' }],
		['sentFrom=2026-10-03&sentTo=2026-10-17', { days: { from: '2026-10-03', to: '2026-10-17' } }],
		['sentFrom=2026-10-03&sentTo=2026-10-03', { days: { from: '2026-10-03', to: '2026-10-03' } }],
		['sent=choose&from=2026-10-03&to=2026-10-17', { days: { from: '2026-10-03', to: '2026-10-17' } }]
	])('reads %s', (query, period) => {
		expect(parse(query)).toEqual(period);
	});

	it.each(['', 'sent=today', 'sent=choose', 'sentFrom=2026-10-03', 'sentTo=2026-10-17', 'sentFrom=2026-10-17&sentTo=2026-10-03', 'sentFrom=nope&sentTo=2026-10-03', 'sentFrom=2026-02-30&sentTo=2026-03-03'])(
		'reads "%s" as no period',
		(query) => {
			expect(parse(query)).toBeUndefined();
		}
	);
});

describe('invoicePeriodDays', () => {
	const noon = new Date('2026-11-14T17:00:00Z');

	it.each([
		['this-month', { from: '2026-11-01', to: '2026-11-30' }],
		['last-month', { from: '2026-10-01', to: '2026-10-31' }],
		['this-year', { from: '2026-01-01', to: '2026-12-31' }],
		['last-year', { from: '2025-01-01', to: '2025-12-31' }]
	] as const)('counts %s from Nov 14, 2026', (preset, days) => {
		expect(invoicePeriodDays({ preset }, 'America/New_York', noon)).toEqual(days);
	});

	it('counts last month across a year end and a leap February', () => {
		expect(invoicePeriodDays({ preset: 'last-month' }, 'UTC', new Date('2027-01-05T12:00:00Z'))).toEqual({ from: '2026-12-01', to: '2026-12-31' });
		expect(invoicePeriodDays({ preset: 'this-month' }, 'UTC', new Date('2028-02-10T12:00:00Z'))).toEqual({ from: '2028-02-01', to: '2028-02-29' });
	});

	it('counts today in the Practice zone, not the reader zone', () => {
		const lateOnThe31st = new Date('2026-12-01T03:00:00Z');
		expect(invoicePeriodDays({ preset: 'this-month' }, 'America/New_York', lateOnThe31st)).toEqual({ from: '2026-11-01', to: '2026-11-30' });
		expect(invoicePeriodDays({ preset: 'this-month' }, 'UTC', lateOnThe31st)).toEqual({ from: '2026-12-01', to: '2026-12-31' });
	});

	it('hands chosen days on and no period as none', () => {
		expect(invoicePeriodDays({ days: { from: '2026-10-03', to: '2026-10-17' } }, 'UTC', noon)).toEqual({ from: '2026-10-03', to: '2026-10-17' });
		expect(invoicePeriodDays(undefined, 'UTC', noon)).toBeUndefined();
	});
});

describe('the period as an address and a key', () => {
	it('names a preset and two days', () => {
		expect(invoicePeriodQuery(undefined)).toEqual([]);
		expect(invoicePeriodQuery({ preset: 'last-month' })).toEqual([['sent', 'last-month']]);
		expect(invoicePeriodQuery({ days: { from: '2026-10-03', to: '2026-10-17' } })).toEqual([
			['sentFrom', '2026-10-03'],
			['sentTo', '2026-10-17']
		]);
		expect(invoicesViewQuery('unpaid', 'sent-oldest', { preset: 'this-year' })).toBe('?show=unpaid&order=sent-oldest&sent=this-year');
	});

	it('keys equal periods alike and none as empty', () => {
		expect(invoicePeriodKey(undefined)).toBe('');
		expect(invoicePeriodKey({ preset: 'this-month' })).toBe('this-month');
		expect(invoicePeriodKey({ days: { from: '2026-10-03', to: '2026-10-17' } })).toBe('2026-10-03..2026-10-17');
	});

	it('says what the select shows', () => {
		expect(invoicePeriodChoiceOf(undefined)).toBe('');
		expect(invoicePeriodChoiceOf({ preset: 'last-year' })).toBe('last-year');
		expect(invoicePeriodChoiceOf({ days: { from: '2026-10-03', to: '2026-10-17' } })).toBe('choose');
	});
});

describe('invoicePeriodWords', () => {
	it.each([
		[{ from: '2026-10-01', to: '2026-10-31' }, 'in October 2026'],
		[{ from: '2026-02-01', to: '2026-02-28' }, 'in February 2026'],
		[{ from: '2026-01-01', to: '2026-12-31' }, 'in 2026'],
		[{ from: '2026-10-03', to: '2026-10-17' }, 'Oct 3 to Oct 17, 2026'],
		[{ from: '2026-10-03', to: '2026-10-03' }, 'on Oct 3, 2026'],
		[{ from: '2026-12-28', to: '2027-01-03' }, 'Dec 28, 2026 to Jan 3, 2027'],
		[{ from: '2026-10-01', to: '2026-11-30' }, 'Oct 1 to Nov 30, 2026']
	])('says %j as "%s"', (days, words) => {
		expect(invoicePeriodWords(days)).toBe(words);
	});
});

describe('the period in the result line and the totals note', () => {
	const counts = { invoiceCount: 35, outstandingCount: 12, overdueCount: 7 };
	const october = { from: '2026-10-01', to: '2026-10-31' };

	it.each([
		['all', 30, true, '', 'Showing the first 30 of 35 Invoices sent in October 2026, newest first.'],
		['unpaid', 4, false, '', 'Showing 4 unpaid Invoices sent in October 2026, earliest payment due first.'],
		['unpaid', 1, false, '', 'Showing 1 unpaid Invoice sent in October 2026, earliest payment due first.'],
		['all', 0, false, '', 'No Invoices sent in October 2026.'],
		['overdue', 0, false, '', 'No overdue Invoices sent in October 2026.'],
		['all', 3, false, 'Quinn', 'Showing 3 Invoices sent in October 2026 matching "Quinn", newest first.'],
		['all', 0, false, 'Quinn', 'No Invoice sent in October 2026 matches "Quinn".']
	] as const)('%s with %i shown reads "%s"', (view, shown, hasMore, search, text) => {
		expect(invoicesResultSummary(view, shown, hasMore, counts, search, '', october)).toBe(text);
	});

	it('says whose figures the totals are', () => {
		expect(invoiceTotalsNote(october)).toBe('Totals for Invoices sent in October 2026.');
	});
});
