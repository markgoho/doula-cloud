<!--
	Test support, not a component anybody ships.

	Three Links or three Buttons in a row with too little room for them
	(#1851): the shape of the Staff bar's nav, the Client portal's top bar
	and a cluster of actions, where each control is a flex item that may
	shrink. `inlineSize` is the room the row is given, so a spec can make it
	too small without this file choosing a width.
-->
<script lang="ts">
	import Button from '../Button.svelte';
	import Link from '../Link.svelte';

	let {
		kind,
		label,
		inlineSize
	}: { kind: 'link' | 'button'; label: string; inlineSize: string } = $props();
</script>

<div class="row" style:inline-size={inlineSize}>
	{#each ['first', 'second', 'third'] as key (key)}
		{#if kind === 'link'}
			<Link href="#{key}" {label} variant="nav" />
		{:else}
			<Button {label} variant="secondary" />
		{/if}
	{/each}
</div>

<style>
	.row {
		display: flex;
	}
</style>
