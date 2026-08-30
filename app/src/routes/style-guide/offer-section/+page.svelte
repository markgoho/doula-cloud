<script lang="ts">
	import OfferSection from '#lib/components/organisms/OfferSection.svelte';
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
			state: 'declined',
			clientFirstInitial: 'S',
			clientArea: 'Rochester, NY',
			dueDate: '2026-11-02',
			employmentType: 'employee',
			offeredAt: '2026-07-20T00:00:00Z',
			expiresAt: '2026-07-27T00:00:00Z',
			decidedAt: '2026-07-21T00:00:00Z'
		}
	];

	const doulas = [
		{ staffId: 'st_1', name: 'Lena Ortiz', employmentType: 'contractor' },
		{ staffId: 'st_2', name: 'Priya Raman', employmentType: 'employee' }
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Offer section</h1>

	<section>
		<h2>Offers already made, and the form for another</h2>
		<OfferSection
			{offers}
			{doulas}
			clientName="Sarah Whitfield"
			onCreate={async () => {}}
			onWithdraw={async () => {}}
		/>
	</section>

	<section>
		<h2>Nobody asked yet</h2>
		<OfferSection offers={[]} {doulas} onCreate={async () => {}} onWithdraw={async () => {}} />
	</section>
</stack-l>
