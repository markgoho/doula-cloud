import type { Offer } from '#lib/offer.js';

/*
 * The longest realistic value, not a representative one (ADR-0025): an
 * Offer names a place and states its terms in the Practice's own words,
 * and the amount is a four-figure birth package.
 */
export const openOffer: Offer = {
	offerId: 'of_1',
	state: 'offered',
	clientFirstInitial: 'S',
	clientArea: 'Irondequoit and the north-east suburbs of Rochester, NY',
	dueDate: '2027-11-02',
	amountCents: 425_000,
	terms:
		'Two prenatal visits at the Client\'s home, continuous support through the birth wherever it happens, and one postpartum visit in the first fortnight.',
	employmentType: 'contractor',
	offeredAt: '2026-08-01T00:00:00Z',
	expiresAt: '2026-08-08T00:00:00Z'
};
