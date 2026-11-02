/* eslint-disable unicorn/no-null -- doula-ops answers null for a value it has not got */
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import '../../../../app/src/lib/styles/app.css';
import type { System } from '../system.ts';
import SystemSection from './System.svelte';

function answering(status: number, body: unknown) {
	return vi.fn<typeof fetch>(async () => Response.json(body, { status }));
}

const none = { signature: 0, livemode: 0, body: 0, other: 0, failed: 0 };

const system: System = {
	stripe: { mode: 'test', platformAccountId: 'acct_1Platform', undelivered: { count: 2, atLeast: false } },
	webhooks: [
		{ route: 'stripe_platform', lastArrivalAt: '2026-11-02T14:05:00Z', refusals: none },
		{ route: 'stripe_connect', lastArrivalAt: null, refusals: { ...none, signature: 2, livemode: 1 } },
		{ route: 'stripe_account', lastArrivalAt: null, refusals: none },
		{ route: 'mailgun', lastArrivalAt: '2026-11-01T20:00:00Z', refusals: none }
	],
	eventTypes: [
		{ route: 'stripe_platform', type: 'checkout.session.completed', lastArrivalAt: '2026-11-02T14:05:00Z' },
		{ route: 'stripe_connect', type: 'invoice.paid', lastArrivalAt: null }
	],
	outboxes: [
		{ name: 'portal_invite_outbox', pending: 3, deadLettered: 1, oldestDueAt: '2026-11-02T13:00:00Z' },
		{ name: 'low_credit_outbox', pending: 0, deadLettered: 0, oldestDueAt: null }
	],
	pilot: {
		gate: { openedAt: null, openedBy: null },
		allowlist: {
			total: 2,
			items: [
				{ address: 'ada@example.com', addedAt: '2026-10-30T15:00:00Z', addedBy: 'Mark Goho' },
				{ address: 'old@example.com', addedAt: '2026-10-01T15:00:00Z', addedBy: null }
			]
		}
	}
};

async function show(body: System = system) {
	const fetcher = answering(200, body);
	await render(SystemSection, { fetcher, onsignedout: vi.fn() });
	return fetcher;
}

describe('System', () => {
	it('shows the Stripe mode, the platform account and the events Stripe could not deliver', async () => {
		const fetcher = await show();

		await expect.element(page.getByRole('heading', { name: 'Stripe', level: 2, exact: true })).toBeVisible();
		await expect.element(page.getByText('Test mode')).toBeVisible();
		await expect.element(page.getByText('acct_1Platform')).toBeVisible();
		await expect.element(page.getByText('Events Stripe could not deliver in the last 7 days')).toBeVisible();
		await expect.element(page.getByText('2 events')).toBeVisible();
		expect(fetcher).toHaveBeenCalledWith('/api/system', { credentials: 'include' });
	});

	it('says so when Stripe could not be read, and when the count is a floor', async () => {
		await show({ ...system, stripe: { mode: 'live', platformAccountId: null, undelivered: null } });

		await expect.element(page.getByText('Live mode')).toBeVisible();
		await expect.element(page.getByText('Stripe did not answer').first()).toBeVisible();
		await expect.element(page.getByText('Stripe did not answer').nth(1)).toBeVisible();
	});

	it('shows a floor as “or more”', async () => {
		await show({ ...system, stripe: { ...system.stripe, undelivered: { count: 100, atLeast: true } } });

		await expect.element(page.getByText('100 or more')).toBeVisible();
	});

	it('shows each webhook route’s last arrival in New York time and its refusals by kind', async () => {
		await show();

		const table = page.getByRole('table', { name: 'Each route’s last arrival and its refusals in the last 7 days' });
		await expect.element(table.getByRole('row', { name: /Stripe platform November 2, 2026 at 9:05 AM EST 0/ })).toBeVisible();
		await expect
			.element(table.getByRole('row', { name: 'Stripe Connect Nothing yet 3 (1 livemode, 2 signature)' }))
			.toBeVisible();
		await expect.element(table.getByRole('row', { name: /Mailgun bounces and complaints November 1, 2026 at 3:00 PM EST/ })).toBeVisible();
	});

	it('lists each handled event type, with ones never seen marked', async () => {
		await show();

		const table = page.getByRole('table', { name: 'The last arrival of each event type DoulaCloud handles' });
		await expect
			.element(table.getByRole('row', { name: 'checkout.session.completed Stripe platform November 2, 2026 at 9:05 AM EST' }))
			.toBeVisible();
		await expect.element(table.getByRole('row', { name: 'invoice.paid Stripe Connect Not seen yet' })).toBeVisible();
	});

	it('counts each outbox’s pending and dead-lettered rows and dates the oldest one due', async () => {
		await show();

		const table = page.getByRole('table', { name: 'Rows waiting to be sent, and rows that failed for good' });
		await expect.element(table.getByRole('row', { name: 'Portal invite 3 1 November 2, 2026 at 8:00 AM EST' })).toBeVisible();
		await expect.element(table.getByRole('row', { name: 'Low credit 0 0 None due' })).toBeVisible();
	});

	it('says the gate is closed', async () => {
		await show();

		await expect.element(page.getByText('Closed. Only an address on the allowlist can create a Practice.')).toBeVisible();
	});

	it('says when an open gate was opened and by whom, or that nobody is on record', async () => {
		await show({ ...system, pilot: { ...system.pilot, gate: { openedAt: '2027-01-04T14:00:00Z', openedBy: 'Mark Goho' } } });

		await expect.element(page.getByText('Open since January 4, 2027 at 9:00 AM EST, opened by Mark Goho.')).toBeVisible();
	});

	it('does not name an opener the record does not have', async () => {
		await show({ ...system, pilot: { ...system.pilot, gate: { openedAt: '2027-01-04T14:00:00Z', openedBy: null } } });

		await expect.element(page.getByText('Open since January 4, 2027 at 9:00 AM EST.')).toBeVisible();
	});

	it('lists each allowlisted address with who added it and when', async () => {
		await show();

		const table = page.getByRole('table', { name: 'Addresses that can create a Practice while the gate is closed' });
		await expect.element(table.getByRole('row', { name: 'ada@example.com October 30, 2026 at 11:00 AM EDT Mark Goho' })).toBeVisible();
		await expect.element(table.getByRole('row', { name: 'old@example.com October 1, 2026 at 11:00 AM EDT Not recorded' })).toBeVisible();
	});

	it('says how many addresses there are when the list is cut short', async () => {
		await show({ ...system, pilot: { ...system.pilot, allowlist: { ...system.pilot.allowlist, total: 300 } } });

		await expect.element(page.getByText('The newest 2 of 300 addresses.')).toBeVisible();
	});

	it('says when the allowlist is empty', async () => {
		await show({ ...system, pilot: { ...system.pilot, allowlist: { total: 0, items: [] } } });

		await expect.element(page.getByText('No address is on the allowlist.')).toBeVisible();
	});

	it('sends a signed-out read to sign in', async () => {
		const onsignedout = vi.fn();

		await render(SystemSection, { fetcher: answering(401, { code: 'UNAUTHORIZED', message: 'Sign in' }), onsignedout });

		await vi.waitFor(() => expect(onsignedout).toHaveBeenCalledOnce());
	});

	it('shows a refused read in words', async () => {
		await render(SystemSection, {
			fetcher: answering(500, { code: 'INTERNAL', message: 'Something went wrong on our side.' }),
			onsignedout: vi.fn()
		});

		await expect.element(page.getByRole('alert')).toBeVisible();
		await expect.element(page.getByRole('table')).not.toBeInTheDocument();
	});
});
