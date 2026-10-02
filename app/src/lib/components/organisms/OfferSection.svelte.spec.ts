import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import OfferSection from './OfferSection.svelte';
import type { NewOffer, Offer } from '#lib/offer.js';
import { SERVICE_PROBLEM } from '#lib/formErrors.js';

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
	/** `undefined`, passed by name, is the roster that could not be read
	 * (#1432) -- so the default roster is applied only when the key is
	 * absent, which a destructuring default cannot tell apart. */
	doulas?: { staffId: string; name: string; employmentType: string }[] | undefined;
	clientName?: string;
	onCreate?: (offer: NewOffer) => Promise<void>;
	onWithdraw?: (offerId: string) => Promise<void>;
}

async function setup(options: SetupOptions = {}) {
	const {
		offers = [],
		clientName = 'Rosa Martinez',
		onCreate = vi.fn().mockResolvedValue(undefined),
		onWithdraw = vi.fn().mockResolvedValue(undefined)
	} = options;
	const doulas = 'doulas' in options ? options.doulas : [contractor, employee];
	const { container } = await render(OfferSection, {
		offers,
		doulas,
		clientName,
		onCreate,
		onWithdraw
	});
	return { container, onCreate, onWithdraw };
}

// "Withdraw" alone doesn't say which Offer it withdraws (#515); the
// distinguishing name is a sibling joined by aria-describedby, the same
// pattern the Edit link fix (#513) and CheckAnswers' Change links use, so
// no accessible query names it directly.
function describedByText(container: HTMLElement, button: ReturnType<typeof page.getByRole>): string {
	const describedBy = button.element().getAttribute('aria-describedby') ?? '';
	return container.querySelector(`#${describedBy}`)?.textContent ?? '';
}

/**
 * The fragment of each error-summary link that no element on the page
 * carries as its id (#1432). querySelector, as svelte-tests.md's case 3:
 * an id is not in the accessible tree, and "every link has somewhere to
 * go" is a fact about the document, not about any one element.
 */
function danglingSummaryTargets(container: HTMLElement): string[] {
	return [...container.querySelectorAll<HTMLAnchorElement>('a[href^="#"]')]
		.map((link) => link.getAttribute('href')!)
		.filter((fragment) => container.querySelector(fragment) === null);
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

		// .first() -- #515's hidden sibling naming the Withdraw button falls
		// back to the same address and duplicates this text.
		await expect.element(page.getByText('renata@example.test').first()).toBeVisible();
	});

	it('withdraws an open Offer', async () => {
		const { onWithdraw } = await setup({ offers: [openOffer] });

		await page.getByRole('button', { name: 'Withdraw' }).click();

		expect(onWithdraw).toHaveBeenCalledWith('offer-1');
	});

	it('names the Withdraw button by who was offered the work', async () => {
		const other: Offer = { ...openOffer, offerId: 'offer-2', targetName: '', targetAddress: 'jo@example.test' };
		const { container } = await setup({ offers: [openOffer, other] });

		const withdrawButtons = page.getByRole('button', { name: 'Withdraw' });
		expect(describedByText(container, withdrawButtons.first())).toBe('Renata Alvarez');
		expect(describedByText(container, withdrawButtons.nth(1))).toBe('jo@example.test');
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

	it('says that joining by email makes a contractor, so the fee is not a surprise', async () => {
		await setup();

		await page.getByLabelText('Someone new, by email').click();

		await expect
			.element(page.getByText('A doula invited by email joins the practice as a contractor, so this offer carries a fee.'))
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
		// .first() -- #1228's ErrorSummary repeats the message as a link,
		// so it now appears twice on screen (the summary and the field).
		await expect.element(page.getByText('Enter a fee greater than zero').first()).toBeVisible();
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

	it('falls back to the service problem message when onCreate rejects with a non-Error', async () => {
		const onCreate = vi.fn().mockRejectedValue('boom');
		await setup({ onCreate });

		await page.getByLabelText('Renata Alvarez').click();
		await page.getByLabelText('Fee (USD)').fill('450');
		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		await expect.element(page.getByText(SERVICE_PROBLEM)).toBeVisible();
	});

	// #1228: novalidate takes StackedForm's browser refusal away, so this
	// form's own required check is what stops an empty submit now. The
	// Client's first initial is pre-filled from her name (#1228's setup
	// seeds it), so it carries no refusal of its own here.
	it('refuses an empty submit through the summary, without calling onCreate', async () => {
		const { onCreate } = await setup();

		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).not.toHaveBeenCalled();
		await expect.element(page.getByText('There is a problem')).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Select a Doula' })).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Enter the general area' })).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Enter the due date' })).toBeVisible();
	});

	it('refuses an emptied Client first initial', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText("Client's first initial").fill('');
		await page.getByLabelText('General area').fill('North side');
		await page.getByLabelText('Due date').fill('2027-01-04');
		await page.getByLabelText('Renata Alvarez').click();
		await page.getByLabelText('Fee (USD)').fill('450');
		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).not.toHaveBeenCalled();
		await expect.element(page.getByText("Enter the Client's first initial").first()).toBeVisible();
	});

	it('refuses an unpicked Doula, which carried no browser check of its own', async () => {
		const { onCreate } = await setup();

		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).not.toHaveBeenCalled();
		await expect.element(page.getByText('Select a Doula').first()).toBeVisible();
	});

	it('refuses an empty email address for the "someone new" target', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText('Someone new, by email').click();
		await page.getByLabelText('Fee (USD)').fill('450');
		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).not.toHaveBeenCalled();
		await expect.element(page.getByText('Enter an email address').first()).toBeVisible();
	});

	it('refuses a malformed email address, which lost its type="email" browser check', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText('Someone new, by email').click();
		await page.getByLabelText('Email address').fill('not-an-email');
		await page.getByLabelText('Fee (USD)').fill('450');
		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).not.toHaveBeenCalled();
		await expect
			.element(page.getByText('Enter an email address in the correct format, like name@example.com').first())
			.toBeVisible();
	});

	it('refuses an empty fee for a contractor Doula', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText('Renata Alvarez').click();
		await fillFacts();
		await page.getByRole('button', { name: 'Send Offer' }).click();

		expect(onCreate).not.toHaveBeenCalled();
		await expect.element(page.getByText('Enter a fee').first()).toBeVisible();
	});

	// #1432: a Practice with nobody to pick from. That is a Practice with
	// no Doula at all, and, since #1598 took the sender out of the list the
	// Engagement page hands this form, every solo Owner, whose only Doula is
	// the person at the screen.
	describe('with nobody else at the Practice to pick', () => {
		it('asks no question that has one answer, and says in words why', async () => {
			await setup({ doulas: [] });

			await expect
				.element(
					page.getByText(
						'There is no one at this practice to offer this work to, so it can only be offered to someone new, by email.'
					)
				)
				.toBeVisible();
			await expect.element(page.getByLabelText('Email address')).toBeVisible();
			expect(page.getByRole('group', { name: 'Offer this work to' }).elements()).toHaveLength(0);
			expect(page.getByRole('group', { name: 'Doula' }).elements()).toHaveLength(0);
			expect(page.getByRole('radio').elements()).toHaveLength(0);
		});

		it('sends every refusal to a control that is on the page', async () => {
			const { container, onCreate } = await setup({ doulas: [] });

			await page.getByRole('button', { name: 'Send Offer' }).click();

			expect(onCreate).not.toHaveBeenCalled();
			await expect.element(page.getByRole('link', { name: 'Enter an email address' })).toBeVisible();
			expect(page.getByRole('link', { name: 'Select a Doula' }).elements()).toHaveLength(0);
			expect(danglingSummaryTargets(container)).toEqual([]);
			// In the order the fields are on the page, which is the order
			// a reader walking the summary top to bottom is led through.
			expect(
				page
					.getByRole('link')
					.elements()
					.map((link) => link.textContent.trim())
			).toEqual(['Enter an email address', 'Enter a fee', 'Enter the general area', 'Enter the due date']);
		});

		it('still sends an Offer to an email address, with the fee a contractor carries', async () => {
			const { onCreate } = await setup({ doulas: [] });

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

		it('does not say there is no one when the roster could not be read', async () => {
			const { container } = await setup({ doulas: undefined });

			await expect
				.element(
					page.getByText(
						'We could not load who is at this practice, so this work can only be offered to someone new, by email. To pick someone already here, reload the page.'
					)
				)
				.toBeVisible();
			expect(page.getByText(/There is no one at this practice/).elements()).toHaveLength(0);
			expect(page.getByRole('radio').elements()).toHaveLength(0);

			await page.getByRole('button', { name: 'Send Offer' }).click();

			await expect.element(page.getByRole('link', { name: 'Enter an email address' })).toBeVisible();
			expect(danglingSummaryTargets(container)).toEqual([]);
		});
	});

	it('sends every refusal to a control that is on the page when there is a Doula to pick', async () => {
		const { container } = await setup();

		await page.getByRole('button', { name: 'Send Offer' }).click();

		await expect.element(page.getByRole('link', { name: 'Select a Doula' })).toBeVisible();
		expect(danglingSummaryTargets(container)).toEqual([]);
	});
});
