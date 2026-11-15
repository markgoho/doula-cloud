<script lang="ts">
	import EarlierContracts from '#lib/components/molecules/EarlierContracts.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Earlier Contracts" />
	<Text
		text="The voided Contracts an Engagement's current one replaced, kept in its Contract section as history. Each has its own anchor, which the Contracts list's row for it opens: the day it was voided, who voided it, and the reason on the request that was granted."
		tone="variant"
	/>

	<EarlierContracts
		timeZone="America/New_York"
		earlier={[
			{
				id: 'earlier-2',
				voidedAt: '2026-11-09T15:00:00Z',
				voidedBy: 'Anne-Marie Ochieng-Whitfield',
				requestedBy: 'Priyanka Venkataraman-Solberg',
				reason: 'Signed the postpartum template for a birth Client; the scope and the fee were both wrong.'
			},
			{ id: 'earlier-1', voidedAt: '2026-10-02T15:00:00Z' }
		]}
	/>
</stack-l>
