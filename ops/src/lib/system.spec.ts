/* eslint-disable unicorn/no-null -- doula-ops answers null for a value it has not got */
import { describe, expect, it, vi } from 'vitest';
import {
	arrivalLabel,
	modeLabel,
	outboxLabel,
	readSystem,
	refusalBreakdown,
	refusalTotal,
	routeLabel,
	typeArrivalLabel,
	undeliveredLabel
} from './system.ts';

describe('readSystem', () => {
	it('reads the System section with the operator’s session', async () => {
		const body = { stripe: { mode: 'test' } };
		const fetcher = vi.fn<typeof fetch>(async () => Response.json(body));

		expect(await readSystem(fetcher)).toEqual({ kind: 'ok', value: body });
		expect(fetcher).toHaveBeenCalledWith('/api/system', { credentials: 'include' });
	});

	it('sends a signed-out read to sign in', async () => {
		const fetcher = vi.fn<typeof fetch>(async () => Response.json({ code: 'UNAUTHORIZED', message: 'Sign in' }, { status: 401 }));

		expect(await readSystem(fetcher)).toEqual({ kind: 'signed-out' });
	});
});

describe('labels', () => {
	it('names each route, and passes an unknown one through', () => {
		expect(routeLabel('stripe_connect')).toBe('Stripe Connect');
		expect(routeLabel('mailgun')).toBe('Mailgun bounces and complaints');
		expect(routeLabel('other')).toBe('other');
	});

	it('names the Stripe mode, and a missing key', () => {
		expect(modeLabel('test')).toBe('Test mode');
		expect(modeLabel('live')).toBe('Live mode');
		expect(modeLabel('')).toBe('No Stripe key is set');
	});

	it('reads an outbox’s table name as words', () => {
		expect(outboxLabel('portal_invite_outbox')).toBe('Portal invite');
		expect(outboxLabel('site_build_outbox')).toBe('Site build');
		expect(outboxLabel('staff_mfa_recovery_outbox')).toBe('Staff MFA recovery');
	});

	it('shows an arrival in New York time, or that none has come', () => {
		expect(arrivalLabel('2026-11-02T14:05:00Z')).toBe('November 2, 2026 at 9:05 AM EST');
		expect(arrivalLabel(null)).toBe('Nothing yet');
		expect(typeArrivalLabel('2026-07-02T14:05:00Z')).toBe('July 2, 2026 at 10:05 AM EDT');
		expect(typeArrivalLabel(null)).toBe('Not seen yet');
	});
});

describe('refusals', () => {
	const none = { signature: 0, livemode: 0, body: 0, other: 0, failed: 0 };

	it('totals every kind', () => {
		expect(refusalTotal({ signature: 2, livemode: 1, body: 3, other: 4, failed: 5 })).toBe(15);
		expect(refusalTotal(none)).toBe(0);
	});

	it('names only the kinds that happened, livemode first', () => {
		expect(refusalBreakdown({ ...none, signature: 2, livemode: 1 })).toBe('1 livemode, 2 signature');
		expect(refusalBreakdown({ signature: 1, livemode: 0, body: 1, other: 1, failed: 1 })).toBe(
			'1 signature, 1 handler failed, 1 unreadable, 1 other'
		);
		expect(refusalBreakdown(none)).toBe('');
	});
});

describe('undeliveredLabel', () => {
	it('counts events, one or many, and says when the count is a floor', () => {
		expect(undeliveredLabel({ count: 1, atLeast: false })).toBe('1 event');
		expect(undeliveredLabel({ count: 0, atLeast: false })).toBe('0 events');
		expect(undeliveredLabel({ count: 100, atLeast: true })).toBe('100 or more');
	});
});
