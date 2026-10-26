<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';

	const noop = () => {};
</script>

<stack-l space="var(--space-6)">
	<h1>Button</h1>

	<section>
		<h2>Variants</h2>
		<cluster-l space="var(--space-4)">
			<Button label="I agree to sign electronically, continue" variant="primary" onClick={noop} />
			<Button label="Back to the engagement request you were deciding" variant="secondary" onClick={noop} />
			<Button label="Remove this Doula from the Practice" variant="destructive" onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Sizes</h2>
		<cluster-l space="var(--space-4)" align="center">
			<Button label="Approve and start the work" size="sm" onClick={noop} />
			<Button label="Approve and start the work" size="md" onClick={noop} />
			<Button label="Approve and start the work" size="lg" onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Icon composition</h2>
		<cluster-l space="var(--space-4)" align="center">
			<Button label="Answer the website question" icon="arrow-right" onClick={noop} />
			<Button label="Dismiss" icon="x" iconOnly onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Disabled</h2>
		<p>
			A dashed outline in the muted color with no fill, whatever the variant. A button that is unavailable but keeps focus
			(the last Move down) draws the same.
		</p>
		<cluster-l space="var(--space-4)">
			<Button label="Send this invoice to the Client" disabled onClick={noop} />
			<Button label="Move up" variant="secondary" disabled onClick={noop} />
			<Button label="Remove this Doula from the Practice" variant="destructive" disabled onClick={noop} />
			<Button label="Move down" variant="secondary" ariaDisabled onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Loading</h2>
		<cluster-l space="var(--space-4)">
			<Button label="Sending this invoice to the Client" loading onClick={noop} />
		</cluster-l>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any button on this page to see its focus outline.</p>
	</section>
</stack-l>
