import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import OfferSection from './OfferSection.svelte';
import type { NewOffer, Offer } from '#lib/offer.js';

const contractor = { staffId: 'staff-1', name: 'Renata Alvarez', employmentType: 'contractor' };
const employee = { staffId: 'staff-2', name: 'Dana Okafor', employmentType: 'employee' };

const openOffer: Offer = {
	offerId: 'offer-1',
	state: 'offered',
	clientFirstInitial: 'R',
	clientArea: 'North side',
	dueDate: '2027-01-04',
	amountCents: 45_000,
	terms: 'Two prenatal visits.',
	employmentType: 'contractor',
	offeredAt: '2026-08-01T00:00:00Z',
	expiresAt: '2026-08-08T00:00:00Z',
	targetName: 'Renata Alvarez',
	targetAddress: 'renata@example.test'
};

interface SetupOptions {
	offers?: Offer[];
	doulas?: { staffId: string; name: string; employmentType: string }[];
	clientName?: string;
	onCreate?: (offer: NewOffer) => Promise<void>;
	onWithdraw?: (offerId: string) => Promise<void>;
}

async function setup({
	offers = [],
	doulas = [contractor, employee],
	clientName = 'Rosa Martinez',
	onCreate = vi.fn().mockResolvedValue(undefined),
	onWithdraw = vi.fn().mockResolvedValue(undefined)
}: SetupOptions = {}) {
	await render(OfferSection, { offers, doulas, clientName, onCreate, onWithdraw });
	return { onCreate, onWithdraw };
}

/**
 * Fills the three always-present facts, which every send needs.
 */
async function fillFacts() {
	await page.getByLabelText("Client's first initial").fill('R');
	await page.getByLabelText('General area').fill('North side');
	await page.getByLabelText('Due date').fill('2027-01-04');
}

describe('OfferSection.svelte', () => {
	it('says so when nobody has been offered the work yet', async () => {
		await setup();

		await expect.element(page.getByText('Nobody has been offered this work yet.')).toBeVisible();
	});

	it('lists who was asked, what state their Offer is in, and the fee', async () => {
		await setup({ offers: [openOffer] });

		await expect.element(page.getByText('Renata Alvarez').first()).toBeVisible();
		await expect.element(page.getByText('Awaiting a decision')).toBeVisible();
		await expect.element(page.getByText('$450.00')).toBeVisible();
	});

	it('falls back to the invited address for an Offer nobody has accepted yet', async () => {
		await setup({ offers: [{ ...openOffer, targetName: '' }], doulas: [] });

		await expect.element(page.getByText('renata@example.test')).toBeVisible();
	});

	it('withdraws an open Offer', async () => {
		const { onWithdraw } = await setup({ offers: [openOffer] });

		await page.getByRole('button', { name: 'Withdraw' }).click();

		expect(onWithdraw).toHaveBeenCalledWith('offer-1');
	});

	it('offers no withdrawal on an Offer that is already closed', async () => {
		await setup({ offers: [{ ...openOffer, state: 'declined' }] });

		await expect.element(page.getByText('Declined')).toBeVisible();
		expect(page.getByRole('button', { name: 'Withdraw' }).elements()).toHaveLength(0);
	});

	it('shows the error when onWithdraw throws', async () => {
		const onWithdraw = vi.fn().mockRejectedValue(new Error('no open offer found at this practice'));
		await setup({ offers: [openOffer], onWithdraw });

		await page.getByRole('button', { name: 'Withdraw' }).click();

		await expect.element(page.getByText('no open offer found at this practice')).toBeVisible();
	});

	it('falls back to a generic message when onWithdraw rejects with a non-Error', async () => {
		const onWithdraw = vi.fn().mockRejectedValue('boom');
		await setup({ offers: [openOffer], onWithdraw });

		await page.getByRole('button', { name: 'Withdraw' }).click();

		await expect.element(page.getByText('Failed to withdraw offer')).toBeVisible();
	});

	it("pre-fills the Client's first initial from her name", async () => {
		await setup();

		await expect.element(page.getByLabelText("Client's first initial")).toHaveValue('R');
	});

	it('asks for a fee once a contractor is chosen, and sends it in cents', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText('Renata Alvarez').click();
		await page.getByLabelText('Fee (USD)').fill('450');
		await fillFacts();
		await page.getByLabelText('Terms').fill('Two prenatal visits.');
		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).toHaveBeenCalledWith({
			staffId: 'staff-1',
			amountCents: 45_000,
			terms: 'Two prenatal visits.',
			clientFirstInitial: 'R',
			clientArea: 'North side',
			dueDate: '2027-01-04'
		});
	});

	it('asks for no fee when the chosen Doula is an employee', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText('Dana Okafor').click();
		expect(page.getByLabelText('Fee (USD)').elements()).toHaveLength(0);

		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).toHaveBeenCalledWith({
			staffId: 'staff-2',
			terms: undefined,
			clientFirstInitial: 'R',
			clientArea: 'North side',
			dueDate: '2027-01-04'
		});
	});

	it('offers work to an email address, which always joins her as a contractor and so carries a fee', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText('Someone new, by email').click();
		await page.getByLabelText('Email address').fill('new@example.test');
		await page.getByLabelText('Fee (USD)').fill('520');
		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).toHaveBeenCalledWith({
			email: 'new@example.test',
			amountCents: 52_000,
			terms: undefined,
			clientFirstInitial: 'R',
			clientArea: 'North side',
			dueDate: '2027-01-04'
		});
	});

	it('says what joining by email makes her, so the fee is not a surprise', async () => {
		await setup();

		await page.getByLabelText('Someone new, by email').click();

		await expect
			.element(page.getByText('She joins the practice as a contractor doula, so this offer carries a fee.'))
			.toBeVisible();
		await expect.element(page.getByLabelText('Fee (USD)')).toBeVisible();
	});

	it('refuses a zero fee without calling onCreate', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText('Renata Alvarez').click();
		await page.getByLabelText('Fee (USD)').fill('0');
		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).not.toHaveBeenCalled();
		await expect.element(page.getByText('Enter a fee greater than zero')).toBeVisible();
	});

	it('clears the typed fields once the Offer is away', async () => {
		await setup();

		await page.getByLabelText('Someone new, by email').click();
		await page.getByLabelText('Email address').fill('new@example.test');
		await page.getByLabelText('Fee (USD)').fill('520');
		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		await expect.element(page.getByLabelText('Email address')).toHaveValue('');
		await expect.element(page.getByLabelText('General area')).toHaveValue('');
	});

	it('shows the error when onCreate throws', async () => {
		const onCreate = vi.fn().mockRejectedValue(new Error('that address already holds a membership'));
		await setup({ onCreate });

		await page.getByLabelText('Renata Alvarez').click();
		await page.getByLabelText('Fee (USD)').fill('450');
		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		await expect.element(page.getByText('that address already holds a membership')).toBeVisible();
	});

	it('falls back to a generic message when onCreate rejects with a non-Error', async () => {
		const onCreate = vi.fn().mockRejectedValue('boom');
		await setup({ onCreate });

		await page.getByLabelText('Renata Alvarez').click();
		await page.getByLabelText('Fee (USD)').fill('450');
		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		await expect.element(page.getByText('Failed to send offer')).toBeVisible();
	});
});
