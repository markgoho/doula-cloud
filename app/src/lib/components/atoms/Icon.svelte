<script module lang="ts">
	// Duotone reads well at the sizes it was designed for; below that the
	// finer single-layer `light` weight stays legible (issue #96's 20px
	// threshold decision). Exported so callers retuning it (e.g. the style
	// guide's demo) stay in sync with the actual cutoff.
	export const SIZE_THRESHOLD_FOR_DUOTONE = 20;
</script>

<script lang="ts">
	import type { IconName } from './Icon/manifest.js';

	// Self-hosted per issue #96 -- eagerly bundles the generated icon
	// modules produced by `bun run sync-icons`, keyed by file path.
	const iconModules = import.meta.glob<{ duotone: string; light: string }>('./Icon/generated/*.ts', {
		eager: true
	});

	interface Properties {
		name: IconName;
		size?: number;
		label?: string;
		weight?: 'light' | 'duotone';
	}

	let { name, size = 24, label, weight: weightOverride }: Properties = $props();

	const weight = $derived(
		weightOverride ?? (size < SIZE_THRESHOLD_FOR_DUOTONE ? 'light' : 'duotone')
	);
	const innerMarkup = $derived(iconModules[`./Icon/generated/${name}.ts`][weight]);
</script>

<svg
	viewBox="0 0 256 256"
	width={size}
	height={size}
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
	focusable="false"
	><!-- eslint-disable-line svelte/no-at-html-tags -- innerMarkup is repo-committed SVG markup bundled at build time via import.meta.glob, never user input -->{@html innerMarkup}</svg
>

<style>
	@layer components {
		svg {
			display: inline-block;
			vertical-align: middle;
		}

		svg :global(.icon-fg) {
			fill: var(--icon-fg, currentColor);
		}

		svg :global(.icon-bg) {
			fill: var(--icon-bg, currentColor);
		}
	}
</style>
