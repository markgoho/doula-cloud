<script lang="ts">
	import SecondFactorNeeded from '#lib/components/molecules/SecondFactorNeeded.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Second factor needed" />
	<Text
		text="Shown in place of an act's control when the act needs two-factor authentication and the signed-in session has none. It names the act, and links to enrollment, which comes back to this screen."
		tone="variant"
	/>

	<SecondFactorNeeded
		message="You need two-factor authentication before you can delete this Practice."
		returnTo="/style-guide/second-factor-needed"
	/>
</stack-l>
