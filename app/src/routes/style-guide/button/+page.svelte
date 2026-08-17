<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';

	const noop = () => {};
</script>

<stack-l space="var(--space-6)">
	<h1>Button</h1>

	<section>
		<h2>Variants</h2>
		<cluster-l space="var(--space-4)">
			<Button label="Primary" variant="primary" onClick={noop} />
			<Button label="Secondary" variant="secondary" onClick={noop} />
			<Button label="Destructive" variant="destructive" onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Sizes</h2>
		<cluster-l space="var(--space-4)" align="center">
			<Button label="Small" size="sm" onClick={noop} />
			<Button label="Medium" size="md" onClick={noop} />
			<Button label="Large" size="lg" onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Icon composition</h2>
		<cluster-l space="var(--space-4)" align="center">
			<Button label="Continue" icon="arrow-right" onClick={noop} />
			<Button label="Dismiss" icon="x" iconOnly onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Disabled</h2>
		<cluster-l space="var(--space-4)">
			<Button label="Save" disabled onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Loading</h2>
		<cluster-l space="var(--space-4)">
			<Button label="Saving" loading onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any button on this page to see its focus outline.</p>
	</section>
</stack-l>
