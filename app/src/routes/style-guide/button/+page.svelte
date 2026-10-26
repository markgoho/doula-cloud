<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isOpenerReturned?: boolean;
	}

	/*
	 * The opener that takes focus back after its form's Cancel (#2103) is
	 * its own subject: mounting it is the moment it is drawn again, so
	 * loading this variant shows focus land on it.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Button, an opener that took focus back after Cancel', props: { isOpenerReturned: true } }
	];
</script>

<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';

	let { isOpenerReturned = false }: Properties = $props();

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

	<section>
		<h2>Taking focus when it appears</h2>
		<p>
			<code>focusOnAppearing</code> moves focus to the button the moment it renders, or the moment it
			turns true on a button already on screen. A record page's opener uses it after Cancel, so focus
			goes back to where the change was asked from. Choose the "opener that took focus back" state on
			the drag surface to see it land.
		</p>
		{#if isOpenerReturned}
			<div><Button label="Offer this work" variant="secondary" onClick={noop} focusOnAppearing /></div>
		{/if}
	</section>
</stack-l>
