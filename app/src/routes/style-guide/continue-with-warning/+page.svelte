<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		hasWarning?: boolean;
		isBusy?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Continue with warning, after the BFF has refused the first press', props: { hasWarning: true } },
		{ name: 'Continue with warning, busy', props: { isBusy: true } }
	];
</script>

<script lang="ts">
	import ContinueWithWarning from '#lib/components/molecules/ContinueWithWarning.svelte';

	let { hasWarning = false, isBusy = false }: Properties = $props();

	/*
	 * The longest realistic warning (ADR-0025): the whole sentence the BFF sends.
	 */
	const warning =
		'Continuing signs you out of Finger Lakes Birth Collective on this browser. A browser holds one DoulaCloud session at a time.';
</script>

<stack-l space="var(--space-4)">
	<h1>Continue with warning</h1>
	<ContinueWithWarning warning={hasWarning ? warning : undefined} loading={isBusy} onClick={() => {}} />
</stack-l>
