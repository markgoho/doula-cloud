<script lang="ts">
	import DoulaFeeSection from '#lib/components/organisms/DoulaFeeSection.svelte';
	import type { DoulaFee } from '#lib/doulaFee.js';

	/*
	 * Two rows, every field's states between them (ADR-0025's Fixtures
	 * section): one still on the birth and not paid, one ended, paid, with
	 * the longest name, amount and note a Practice would write.
	 */
	const fees: DoulaFee[] = [
		{
			attachmentId: 'att_1',
			doulaName: 'Lena Vasquez',
			amountCents: 45_000,
			acceptedOn: '2026-10-27'
		},
		{
			attachmentId: 'att_2',
			doulaName: 'Alexandra Montgomery-Vasquez',
			amountCents: 425_000,
			acceptedOn: '2026-06-02',
			endedOn: '2026-09-14',
			paid: {
				id: 'fp_2',
				amountCents: 425_000,
				method: 'other',
				note: 'Zelle to the business account, reference DC-2026-0914-backup',
				paidOn: '2026-09-30',
				recordedBy: 'Renata Alvarez-Whitfield',
				recordedAt: '2026-09-30T15:00:00Z'
			}
		}
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Doula fee section</h1>
	<p>Each contractor's accepted fee on an Engagement, and whether the Practice has recorded paying it.</p>
	<DoulaFeeSection
		{fees}
		practiceTimezone="America/New_York"
		onRecord={async () => {}}
		onRemove={async () => {}}
	/>
</stack-l>
