import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { formatCalendarDay, formatInstant } from './dates.js';
import { invoiceFacts, type ClientInvoice } from './clientInvoice.js';
import { dueLabel } from './invoice.js';

/*
 * #1958's and #1965's proof, with the zone set by the spec itself rather
 * than by whoever runs it: CI runs in UTC, where a day read as UTC
 * midnight never moves, so a spec that trusted the runner's zone would
 * pass there and say nothing. The BFF sends an Invoice's `paidOn` and
 * `dueOn` as `YYYY-MM-DD` days it has already read in the Practice's own
 * timezone; the reader's zone, behind UTC or ahead of it, must not move
 * them again.
 *
 * Node reads `process.env.TZ` again on every assignment, and Vitest runs
 * each spec file in its own fork, so the zone set here reaches no other
 * file; it is put back after each block anyway.
 */

const runnerZone = process.env.TZ;

/**
The portal's printed Sent day for an Invoice raised at `createdAt`.
*/
function sentDay(days: Pick<ClientInvoice, 'createdAt' | 'raisedOn'>): string | undefined {
	const invoice: ClientInvoice = {
		id: 'i',
		status: 'open',
		amountCents: 1000,
		currency: 'usd',
		reference: 'INV-1',
		billingMode: 'stripe',
		refundedCents: 0,
		...days
	};
	return invoiceFacts(invoice).find((fact) => fact.label === 'Sent')?.value;
}

function inZone(zone: string): void {
	beforeAll(() => {
		process.env.TZ = zone;
	});
	afterAll(() => {
		process.env.TZ = runnerZone;
	});
}

describe('a date in a zone behind UTC (America/New_York)', () => {
	inZone('America/New_York');

	// Without this the assertions below would pass vacuously in UTC.
	it('is in a zone where UTC midnight is still the day before', () => {
		expect(new Date('2026-10-25T00:00:00Z').getDate()).toBe(24);
	});

	it('shows a paid day of 2026-10-25 as Oct 25, 2026', () => {
		expect(formatCalendarDay('2026-10-25')).toBe('Oct 25, 2026');
	});

	it('shows the due day the BFF sent, with the lateness it counted', () => {
		expect(dueLabel({ dueOn: '2026-11-23', daysOverdue: 0 })).toBe('Nov 23, 2026');
		expect(dueLabel({ dueOn: '2026-11-23', daysOverdue: 1 })).toBe('Nov 23, 2026 — 1 day overdue');
	});

	it('still reads a real instant in the reader zone', () => {
		expect(formatInstant('2026-10-25T00:00:00Z')).toBe('Oct 24, 2026');
	});

	it("prints an Invoice's Sent day as the Practice's day the BFF sent (#2205)", () => {
		// 20:00Z on Oct 25 is Oct 25 here, but already Oct 26 for a Practice in Tokyo.
		expect(sentDay({ createdAt: '2026-10-25T20:00:00Z', raisedOn: '2026-10-26' })).toBe('Oct 26, 2026');
	});
});

describe('a date in a zone ahead of UTC (Asia/Tokyo)', () => {
	inZone('Asia/Tokyo');

	// Without this the assertions below would pass vacuously in UTC.
	it('is in a zone where a UTC evening is already the next day', () => {
		expect(new Date('2026-10-25T20:00:00Z').getDate()).toBe(26);
	});

	it('shows a paid day of 2026-10-25 as Oct 25, 2026', () => {
		expect(formatCalendarDay('2026-10-25')).toBe('Oct 25, 2026');
	});

	it('shows the due day the BFF sent, with the lateness it counted', () => {
		expect(dueLabel({ dueOn: '2026-11-23', daysOverdue: 3 })).toBe('Nov 23, 2026 — 3 days overdue');
	});

	it('still reads a real instant in the reader zone', () => {
		expect(formatInstant('2026-10-25T20:00:00Z')).toBe('Oct 26, 2026');
	});

	it("prints an Invoice's Sent day as the Practice's day the BFF sent (#2205)", () => {
		// 00:03Z on Oct 26 is Oct 26 here, but still Oct 25 for a Practice in New York.
		expect(sentDay({ createdAt: '2026-10-26T00:03:00Z', raisedOn: '2026-10-25' })).toBe('Oct 25, 2026');
	});
});
