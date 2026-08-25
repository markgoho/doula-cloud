import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import OfferInbox from './OfferInbox.svelte';
import type { Offer } from './offer.js';

const openOffer: Offer = {
	offerId: 'offer-1',
	state: 'offered',
	clientFirstInitial: 'R',
	clientArea: 'North side',
	dueDate: '2027-01-04',
	amountCents: 45_000,
	terms: 'Two prenatal visits, on call from 38 weeks.',
	employmentType: 'contractor',
	offeredAt: '2026-08-01T00:00:00Z',
	expiresAt: '2026-08-08T00:00:00Z'
};

async function setup(
	offers: Offer[] = [],
	onDecide: (offerId: string, action: 'accept' | 'decline') => Promise<void> = vi
		.fn()
		.mockResolvedValue(undefined)
) {
	await render(OfferInbox, { offers, onDecide });
	return { onDecide };
}

describe('OfferInbox.svelte', () => {
	it('says so when she has no offers', async () => {
		await setup();

		await expect.element(page.getByText('You have no offers.')).toBeVisible();
	});

	it('shows the four decidable facts and the terms', async () => {
		await setup([openOffer]);

		await expect.element(page.getByText('R', { exact: true })).toBeVisible();
		await expect.element(page.getByText('North side')).toBeVisible();
		await expect.element(page.getByText('2027-01-04')).toBeVisible();
		await expect.element(page.getByText('$450.00')).toBeVisible();
		await expect.element(page.getByText('Two prenatal visits, on call from 38 weeks.')).toBeVisible();
	});

	it('leaves the terms row out when the Offer carries none', async () => {
		await setup([{ ...openOffer, terms: undefined }]);

		expect(page.getByText('Terms').elements()).toHaveLength(0);
	});

	it('says an employee Offer carries no fee', async () => {
		await setup([{ ...openOffer, amountCents: undefined, employmentType: 'employee' }]);

		await expect.element(page.getByText('No per-Engagement fee')).toBeVisible();
	});

	it('calls onDecide with accept', async () => {
		const { onDecide } = await setup([openOffer]);

		await page.getByRole('button', { name: 'Accept' }).click();

		expect(onDecide).toHaveBeenCalledWith('offer-1', 'accept');
	});

	it('calls onDecide with decline', async () => {
		const { onDecide } = await setup([openOffer]);

		await page.getByRole('button', { name: 'Decline' }).click();

		expect(onDecide).toHaveBeenCalledWith('offer-1', 'decline');
	});

	it('offers no decision on an Offer that is already closed, and names its state', async () => {
		await setup([{ ...openOffer, state: 'superseded', decidedAt: '2026-08-02T00:00:00Z' }]);

		await expect.element(page.getByText('Taken by someone else')).toBeVisible();
		expect(page.getByRole('button', { name: 'Accept' }).elements()).toHaveLength(0);
	});

	// #230: the Client's own fields stop being served once an Offer goes
	// terminal, so a past Offer reads as the fact of the asking -- her
	// fee and what she answered -- rather than as three blank rows.
	it('leaves out the Client rows on a closed Offer, and keeps the fee', async () => {
		await setup([
			{ ...openOffer, state: 'declined', clientFirstInitial: '', clientArea: '', dueDate: '' }
		]);

		expect(page.getByText('Area').elements()).toHaveLength(0);
		expect(page.getByText('Due date').elements()).toHaveLength(0);
		await expect.element(page.getByText('$450.00')).toBeVisible();
	});

	it('shows the error when onDecide throws', async () => {
		const onDecide = vi.fn().mockRejectedValue(new Error('that offer is no longer open'));
		await setup([openOffer], onDecide);

		await page.getByRole('button', { name: 'Accept' }).click();

		await expect.element(page.getByText('that offer is no longer open')).toBeVisible();
	});

	it('falls back to a generic message when onDecide rejects with a non-Error', async () => {
		const onDecide = vi.fn().mockRejectedValue('boom');
		await setup([openOffer], onDecide);

		await page.getByRole('button', { name: 'Decline' }).click();

		await expect.element(page.getByText('Failed to answer this offer')).toBeVisible();
	});
});
