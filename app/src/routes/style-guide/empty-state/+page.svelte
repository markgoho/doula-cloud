<script lang="ts">
	import EmptyState from '#lib/components/molecules/EmptyState.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Empty state" />
	<Text
		text="What an empty list or section shows: the duotone icon, one line that says what is not here yet, and the one action that makes the first thing where there is one. A destination is a link; a change on the same screen is a button."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<h2>A line only</h2>
		<EmptyState message="No messages yet. Write to your doula below." />
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>A line and a link</h2>
		<EmptyState
			message="No Practice-defined fields yet."
			action={{ label: 'Add a question', href: '/style-guide/empty-state' }}
		/>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>A line and a button</h2>
		<EmptyState message="No extra questions yet." action={{ label: 'Add a question', onClick: () => {} }} />
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>A long line</h2>
		<EmptyState
			message="No visits are scheduled in this range. Widen the dates, or choose every doula, to see more."
		/>
	</stack-l>
</stack-l>
