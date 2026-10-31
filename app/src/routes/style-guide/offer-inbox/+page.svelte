<script lang="ts">
	import OfferInbox from '#lib/components/organisms/OfferInbox.svelte';
	import type { Offer } from '#lib/offer.js';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): an
	 * Offer names a place and states its terms in the Practice's own words,
	 * and the amount is a four-figure birth package.
	 */
	const offers: Offer[] = [
		{
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
		},
		{
			offerId: 'of_2',
			state: 'accepted',
			clientFirstInitial: '',
			clientArea: '',
			dueDate: '',
			employmentType: 'contractor',
			offeredAt: '2026-06-01T00:00:00Z',
			expiresAt: '2026-06-08T00:00:00Z',
			decidedAt: '2026-06-02T00:00:00Z'
		},
		{
			offerId: 'of_3',
			state: 'accepted',
			clientFirstInitial: '',
			clientArea: '',
			dueDate: '',
			dueMonth: '2026-11',
			amountCents: 425_000,
			employmentType: 'contractor',
			offeredAt: '2026-04-01T00:00:00Z',
			expiresAt: '2026-04-08T00:00:00Z',
			decidedAt: '2026-04-02T00:00:00Z',
			feePaid: { amountCents: 425_000, method: 'bank_transfer', paidOn: '2026-09-30' }
		},
		{
			offerId: 'of_4',
			state: 'accepted',
			clientFirstInitial: '',
			clientArea: '',
			dueDate: '',
			dueMonth: '2027-02',
			amountCents: 45_000,
			employmentType: 'contractor',
			offeredAt: '2026-05-01T00:00:00Z',
			expiresAt: '2026-05-08T00:00:00Z',
			decidedAt: '2026-05-02T00:00:00Z'
		}
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Offer inbox</h1>

	<section>
		<h2>One still open, one already decided</h2>
		<OfferInbox {offers} onDecide={async () => {}} />
	</section>

	<section>
		<h2>Nothing offered</h2>
		<OfferInbox offers={[]} onDecide={async () => {}} />
	</section>
</stack-l>
