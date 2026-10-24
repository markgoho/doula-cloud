<script lang="ts">
	import SecondFactorRecommendation from '#lib/components/molecules/SecondFactorRecommendation.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	// The demo never records a close; the notice stays.
	async function onClose() {}
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Second factor recommendation" />
	<Text
		text="Shown on the Practice overview to an Owner with no two-factor authentication, after First Value, the first Invitation, and Stripe connected. One link, to enrollment, and a Close."
		tone="variant"
	/>

	<SecondFactorRecommendation returnTo="/style-guide/second-factor-recommendation" {onClose} />
</stack-l>
