<script lang="ts">
	import OfferInbox from '#lib/components/organisms/OfferInbox.svelte';
	import type { Offer } from '#lib/offer.js';

	const offers: Offer[] = [
		{
			offerId: 'of_1',
			state: 'offered',
			clientFirstInitial: 'S',
			clientArea: 'Rochester, NY',
			dueDate: '2026-11-02',
			amountCents: 90_000,
			terms: 'Two prenatal visits, the birth, one postpartum visit.',
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
