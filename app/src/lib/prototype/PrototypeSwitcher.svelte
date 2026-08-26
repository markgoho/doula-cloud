<script lang="ts">
	// PROTOTYPE (#371) -- throwaway. Floating variant switcher. Never
	// rendered in a production build.
	import { goto } from '$app/navigation';
	import { page } from '$app/state';

	interface Properties {
		variants: { key: string; name: string }[];
		current: string;
		param?: string;
	}

	let { variants, current, param = 'variant' }: Properties = $props();

	const index = $derived(Math.max(0, variants.findIndex((v) => v.key === current)));
	const currentName = $derived(variants[index]?.name ?? '');
	const isProduction = import.meta.env.PROD;

	function go(step: number) {
		const next = variants[(index + step + variants.length) % variants.length];
		const url = new URL(page.url.href);
		url.searchParams.set(param, next.key);
		goto(url.toString(), { replaceState: true });
	}

	function onKeydown(event: KeyboardEvent) {
		const target = event.target as HTMLElement | null;
		if (target?.closest('input, textarea, select, [contenteditable]')) return;
		if (event.key === 'ArrowLeft') go(-1);
		if (event.key === 'ArrowRight') go(1);
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#if !isProduction}
	<div class="bar">
		<button type="button" onclick={() => go(-1)} aria-label="Previous variant">←</button>
		<span>{current} — {currentName}</span>
		<button type="button" onclick={() => go(1)} aria-label="Next variant">→</button>
	</div>
{/if}

<style>
	.bar {
		position: fixed;
		inset-block-end: 1rem;
		inset-inline-start: 50%;
		translate: -50% 0;
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 0.75rem;
		border-radius: 999px;
		background: #111;
		color: #fff;
		font: 600 0.8125rem/1 ui-monospace, monospace;
		box-shadow: 0 6px 24px rgb(0 0 0 / 35%);
		z-index: 999;
	}

	.bar button {
		border: 0;
		border-radius: 999px;
		width: 1.75rem;
		height: 1.75rem;
		background: #fff;
		color: #111;
		font-size: 0.875rem;
		cursor: pointer;
	}
</style>
