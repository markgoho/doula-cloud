<script lang="ts">
	import { fillProse } from '#lib/contract.js';

	let { prose, values }: { prose: string; values: Record<string, string> } = $props();
</script>

<div class="contract-prose">{fillProse(prose, values)}</div>

<style>
	.contract-prose {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
</style>
