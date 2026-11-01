import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import OfferInbox from './OfferInbox.svelte';
import { formatInstantWithClock } from '#lib/dates.js';
import type { Offer, OfferDecision } from '#lib/offer.js';

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
const sent = formatInstantWithClock(openOffer.offeredAt);

async function setup(
	offers: Offer[] = [],
	onDecide: (offerId: string, action: 'accept' | 'decline') => Promise<OfferDecision> = vi
		.fn()
		.mockImplementation(async (offerId: string, action: 'accept' | 'decline') => ({
			offerId,
			state: action === 'accept' ? 'accepted' : 'declined'
		}))
) {
	const { container } = await render(OfferInbox, {
		offers,
		onDecide,
		practiceName: 'Rooted Birth Collective',
		engagementHref: (engagementId: string) => `/engagements/${engagementId}`
	});
	return { container, onDecide };
}

// "Accept"/"Decline" alone doesn't say which Offer they answer (#515); the
// distinguishing name is a sibling joined by aria-describedby, the same
// pattern the Edit link fix (#513) and CheckAnswers' Change links use, so
// no accessible query names it directly.
function describedByText(container: HTMLElement, button: ReturnType<typeof page.getByRole>): string {
	const describedBy = button.element().getAttribute('aria-describedby') ?? '';
	return container.querySelector(`#${describedBy}`)?.textContent ?? '';
}

describe('OfferInbox.svelte', () => {
	it('says so when she has no offers', async () => {
		await setup();

		await expect.element(page.getByText('You have no offers.')).toBeVisible();
	});

	it('shows the four decidable facts and the terms', async () => {
		await setup([openOffer]);

		await expect.element(page.getByText('R', { exact: true })).toBeVisible();
		// exact: true -- #515's hidden "North side, due Jan 4, 2027" sibling
		// (naming the Accept/Decline buttons) otherwise substring-matches too.
		await expect.element(page.getByText('North side', { exact: true })).toBeVisible();
		// #2236: the product's own date format, not the stored 2027-01-04.
		await expect.element(page.getByText('Jan 4, 2027', { exact: true })).toBeVisible();
		await expect.element(page.getByText('$450.00')).toBeVisible();
		await expect.element(page.getByText('Two prenatal visits, on call from 38 weeks.')).toBeVisible();
	});

	describe('an accepted fee (#2271)', () => {
		const accepted: Offer = {
			...openOffer,
			state: 'accepted',
			clientFirstInitial: '',
			clientArea: '',
			dueDate: '',
			decidedAt: '2026-10-27T15:00:00Z'
		};

		it('links to the Engagement while she still reaches it (#2269)', async () => {
			await setup([{ ...accepted, dueMonth: '2027-02', engagementId: 'eng-7' }]);

			await expect
				.element(page.getByRole('link', { name: 'Open the birth due Feb 2027' }))
				.toHaveAttribute('href', '/engagements/eng-7');
		});

		it('says the day the work ended, with no link (#2270)', async () => {
			await setup([{ ...accepted, dueMonth: '2027-02', endedOn: '2026-10-31' }]);

			await expect.element(page.getByText('Ended', { exact: true })).toBeVisible();
			await expect.element(page.getByText('Oct 31, 2026', { exact: true })).toBeVisible();
			await expect.element(page.getByRole('link')).not.toBeInTheDocument();
		});

		it('says what the Practice recorded paying her', async () => {
			await setup([{ ...accepted, feePaid: { amountCents: 45_000, method: 'bank_transfer', paidOn: '2027-03-03' } }]);

			await expect.element(page.getByText('$450.00 on Mar 3, 2027, by bank transfer')).toBeVisible();
			await expect.element(page.getByText('Oct 27, 2026')).toBeVisible();
		});

		it('says nothing is recorded yet', async () => {
			await setup([accepted]);

			await expect.element(page.getByText('Not recorded yet')).toBeVisible();
		});

		it('names the due month on an accepted Offer (#2383)', async () => {
			await setup([{ ...accepted, dueMonth: '2027-02' }]);

			await expect.element(page.getByText('Feb 2027')).toBeVisible();
		});

		it('shows no due month when the read carries none', async () => {
			await setup([accepted]);

			expect(page.getByText('Due', { exact: true }).all()).toHaveLength(0);
		});

		it('shows no Paid row on an accepted Offer with no fee', async () => {
			await setup([{ ...accepted, amountCents: undefined, decidedAt: undefined }]);

			expect(page.getByText('Not recorded yet').all()).toHaveLength(0);
			expect(page.getByText('Accepted', { exact: true }).all()).toHaveLength(1);
		});
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

	// #2268: declining is final, so it is confirmed first.
	it('asks before it declines, and declines on the confirm', async () => {
		const { onDecide } = await setup([openOffer]);

		await page.getByRole('button', { name: 'Decline' }).click();
		await expect.element(page.getByRole('heading', { name: 'Decline this offer?' })).toBeVisible();
		await expect
			.element(page.getByText("You can't undo this. Rooted Birth Collective can send you a new offer later."))
			.toBeVisible();
		expect(onDecide).not.toHaveBeenCalled();

		await page.getByRole('button', { name: 'Decline offer' }).click();

		expect(onDecide).toHaveBeenCalledWith('offer-1', 'decline');
		const status = page.getByRole('status');
		await expect.element(status).toHaveTextContent('You declined. Rooted Birth Collective can see that on the Client\'s page.');
		await expect.element(status).toHaveFocus();
	});

	it('keeps the offer when she backs out of declining', async () => {
		const { onDecide } = await setup([openOffer]);

		await page.getByRole('button', { name: 'Decline' }).click();
		await page.getByRole('button', { name: 'Keep the offer' }).click();

		expect(onDecide).not.toHaveBeenCalled();
		await expect.element(page.getByRole('button', { name: 'Decline', exact: true })).toBeVisible();
	});

	it('says the work is hers on an accept, and links to it', async () => {
		const onDecide = vi.fn().mockResolvedValue({
			offerId: 'offer-1',
			state: 'accepted',
			engagementId: 'eng-1',
			engagementKind: 'birth',
			clientName: 'Adaeze Nwosu'
		});
		await setup([openOffer], onDecide);

		await page.getByRole('button', { name: 'Accept' }).click();

		const status = page.getByRole('status');
		await expect.element(status).toHaveTextContent("You accepted. You're on Adaeze Nwosu's birth.");
		await expect.element(status).toHaveFocus();
		await expect
			.element(page.getByRole('link', { name: "Open Adaeze Nwosu's birth" }))
			.toHaveAttribute('href', '/engagements/eng-1');
	});

	it('names the Accept and Decline buttons by which Offer they answer', async () => {
		const other: Offer = { ...openOffer, offerId: 'offer-2', clientArea: 'South side', dueDate: '2027-02-10' };
		const { container } = await setup([openOffer, other]);

		const accepts = page.getByRole('button', { name: 'Accept' });
		expect(describedByText(container, accepts.first())).toBe(`North side, due Jan 4, 2027, sent ${sent}`);
		expect(describedByText(container, accepts.nth(1))).toBe(`South side, due Feb 10, 2027, sent ${sent}`);

		const declines = page.getByRole('button', { name: 'Decline' });
		expect(describedByText(container, declines.first())).toBe(`North side, due Jan 4, 2027, sent ${sent}`);
	});

	// #2389: two employee Offers from erased Clients have no area, due date
	// or fee to tell them apart, so the minute each was sent does.
	it('names two fee-less Offers from erased Clients apart by when each was sent', async () => {
		const erased = { ...openOffer, clientFirstInitial: '', clientArea: '', dueDate: '', amountCents: undefined };
		const { container } = await setup([
			erased,
			{ ...erased, offerId: 'offer-2', offeredAt: '2026-08-03T15:30:00Z' }
		]);

		const descriptions = [
			...page.getByRole('button', { name: 'Accept' }).all(),
			...page.getByRole('button', { name: 'Decline' }).all()
		].map((button) => describedByText(container, button));
		expect(descriptions[0]).toBe(`Sent ${sent}`);
		expect(descriptions[1]).toBe(`Sent ${formatInstantWithClock('2026-08-03T15:30:00Z')}`);
		expect(descriptions[0]).not.toBe(descriptions[1]);
		expect(descriptions[2]).not.toBe(descriptions[3]);
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

	// #2385: an erased Client's open Offer carries none of her fields.
	it('leaves out the Client rows on an open Offer whose Client is erased, and names it by when it was sent', async () => {
		const { container } = await setup([{ ...openOffer, clientFirstInitial: '', clientArea: '', dueDate: '' }]);

		expect(page.getByText('Area').elements()).toHaveLength(0);
		expect(page.getByText('Due date').elements()).toHaveLength(0);
		expect(describedByText(container, page.getByRole('button', { name: 'Accept' }))).toBe(`Sent ${sent}`);
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

		await page.getByRole('button', { name: 'Accept' }).click();

		await expect.element(page.getByText('Failed to answer this offer')).toBeVisible();
	});
});
