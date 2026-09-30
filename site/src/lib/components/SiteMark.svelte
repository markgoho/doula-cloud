<!--
@component
The mark, as docs/marketing/brand.md draws it: two strokes, the outer line
in `--color-primary` and the inner arch in `--color-primary-hover`. The
site shows it at 28px tall or more, which is where the sheet asks for two
tones, so this component has no one-color size.

The app's CloudMark still draws the three-arc mark until #1486 lands.
When it does, this component can go and the teaser can use CloudMark.
Decorative by default, since the name is beside it.
-->
<script lang="ts">
	interface Properties {
		// The height in pixels. The sheet's two tones need 28 or more.
		height?: number;
	}

	let { height = 40 }: Properties = $props();

	// The viewBox is 202 by 94 units.
	const width = $derived(Math.round((height * 202) / 94));
</script>

<svg
	viewBox="48 76 202 94"
	{width}
	{height}
	fill="none"
	stroke-width="14"
	stroke-linecap="round"
	aria-hidden="true"
	focusable="false"
>
	<path class="baby" d="M100 160a32 32 0 0 1 64 0" />
	<path class="mother" d="M56 160a76 76 0 0 1 137.97-44 44 44 0 0 1 44.03 44" />
</svg>

<style>
	svg {
		display: block;
	}

	.baby {
		stroke: var(--color-primary-hover);
	}

	.mother {
		stroke: var(--color-primary);
	}
</style>
