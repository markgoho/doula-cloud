<script lang="ts">
	import Heading from '#lib/components/atoms/Heading.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Heading</h1>

	<section>
		<h2>Levels</h2>
		<stack-l space="var(--space-4)">
			<Heading level={1} text="Level 1 heading" />
			<Heading level={2} text="Level 2 heading" />
			<Heading level={3} text="Level 3 heading" />
			<Heading level={4} text="Level 4 heading" />
			<Heading level={5} text="Level 5 heading" />
			<Heading level={6} text="Level 6 heading" />
		</stack-l>
	</section>
</stack-l>
