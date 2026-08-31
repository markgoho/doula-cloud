<script lang="ts">
	import OfferSection from '#lib/components/organisms/OfferSection.svelte';
	import type { Offer } from '#lib/offer.js';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): an
	 * Offer names a place and states its terms in the Practice's own words,
	 * and every Doula in the picker is named in full.
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
			state: 'declined',
			clientFirstInitial: 'S',
			clientArea: 'Irondequoit and the north-east suburbs of Rochester, NY',
			dueDate: '2027-11-02',
			employmentType: 'employee',
			offeredAt: '2026-07-20T00:00:00Z',
			expiresAt: '2026-07-27T00:00:00Z',
			decidedAt: '2026-07-21T00:00:00Z'
		}
	];

	const doulas = [
		{
			staffId: 'st_1',
			name: 'Persephone Adeyemi-Wollstonecraft',
			employmentType: 'contractor'
		},
		{ staffId: 'st_2', name: 'Renata Chiamaka Okonkwo-Adeyemi', employmentType: 'employee' }
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Offer section</h1>

	<section>
		<h2>Offers already made, and the form for another</h2>
		<OfferSection
			{offers}
			{doulas}
			clientName="Anne-Marie Ochieng-Whitfield"
			onCreate={async () => {}}
			onWithdraw={async () => {}}
		/>
	</section>

	<section>
		<h2>Nobody asked yet</h2>
		<OfferSection offers={[]} {doulas} onCreate={async () => {}} onWithdraw={async () => {}} />
	</section>
</stack-l>
