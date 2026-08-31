<script lang="ts">
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Back link" />
	<Text
		text="GOV.UK's Back link. It sits at the very top of a page, above the error summary and the question, and its word is 'Back' unless it names where it goes."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<BackLink href="/style-guide" />
		<BackLink href="/style-guide" label="Back to the engagement request you were deciding" />
	</stack-l>
</stack-l>
