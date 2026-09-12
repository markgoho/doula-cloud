import { describe, expect, it } from 'vitest';
import { billingPeriodMonth, GCP_INVOICE_TIMEZONE, startOfBillingPeriod } from './billingPeriod.ts';

describe('GCP_INVOICE_TIMEZONE', () => {
	it('is the zone GCP assigns invoice.month in, not UTC', () => {
		expect(GCP_INVOICE_TIMEZONE).toBe('America/Los_Angeles');
	});
});

describe('startOfBillingPeriod', () => {
	it('is midnight Pacific of the calendar month `now` falls in', () => {
		expect(startOfBillingPeriod(new Date('2026-09-08T04:30:24Z')).toISOString()).toBe(
			'2026-09-01T07:00:00.000Z'
		);
	});

	it('reads August for an instant seven hours into the UTC month, since Pacific is still August then', () => {
		expect(startOfBillingPeriod(new Date('2026-09-01T03:00:00Z')).toISOString()).toBe(
			'2026-08-01T07:00:00.000Z'
		);
	});

	it('uses the winter offset when the period starts in standard time', () => {
		expect(startOfBillingPeriod(new Date('2026-01-15T12:00:00Z')).toISOString()).toBe(
			'2026-01-01T08:00:00.000Z'
		);
	});

	it('uses the summer offset when the period starts in daylight time, so a fixed offset cannot pass both this and the winter case', () => {
		expect(startOfBillingPeriod(new Date('2026-07-15T12:00:00Z')).toISOString()).toBe(
			'2026-07-01T07:00:00.000Z'
		);
	});

	it('reads daylight time for a period that starts the same Sunday the clocks fall back, since 2 a.m. is still ahead of midnight', () => {
		// US DST ends 2026-11-01, 2 a.m. Pacific. Midnight that day is still PDT.
		expect(startOfBillingPeriod(new Date('2026-11-15T12:00:00Z')).toISOString()).toBe(
			'2026-11-01T07:00:00.000Z'
		);
	});
});

describe('billingPeriodMonth', () => {
	it('names the same month startOfBillingPeriod opens', () => {
		expect(billingPeriodMonth(new Date('2026-09-08T04:30:24Z'))).toBe('202609');
	});

	it('names August for the same stale-window instant startOfBillingPeriod reads as August', () => {
		expect(billingPeriodMonth(new Date('2026-09-01T03:00:00Z'))).toBe('202608');
	});
});
