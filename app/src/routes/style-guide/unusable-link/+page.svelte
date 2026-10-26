<script lang="ts">
	import UnusableLink from '#lib/components/molecules/UnusableLink.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Unusable link" />
	<Text
		text="What a screen reached from an emailed link shows when the link cannot be used: one plain sentence saying what is wrong, then the one next step. A page to go to is a link; a step taken away from DoulaCloud is a line of text."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<Heading level={3} variant="card" text="A next step with a page" />
		<UnusableLink
			message="This link has expired or was already used."
			next={{ label: 'Ask for a new sign-in link', href: '/style-guide/unusable-link' }}
		/>
	</stack-l>

	<stack-l space="var(--space-4)">
		<Heading level={3} variant="card" text="A next step with no page" />
		<UnusableLink
			message="This link is not complete."
			next={{ text: 'Ask your Practice to send the invite again.' }}
		/>
	</stack-l>
</stack-l>
