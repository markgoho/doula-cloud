<script lang="ts">
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Text</h1>

	<section>
		<h2>Sizes</h2>
		<stack-l space="var(--space-4)">
			<Text size="sm" text="Small body copy." />
			<Text size="base" text="Base body copy." />
			<Text size="lg" text="Large body copy." />
		</stack-l>
	</section>

	<section>
		<h2>Muted</h2>
		<stack-l space="var(--space-4)">
			<Text text="Default color." />
			<Text muted text="Muted color, for secondary copy." />
		</stack-l>
	</section>
</stack-l>
