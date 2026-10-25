<script lang="ts">
	/*
	 * PROTOTYPE -- throwaway, ticket #1926, for the type-scale question on
	 * #1722. Six candidate scales, each with body at 16px and the same 1.2
	 * fluid growth, rendered on real screens built from the real components
	 * in a narrow and a wide column side by side. The candidate switches in
	 * page state with the floating bar or the arrow keys; `?variant=` is read
	 * only as the start value, because the artifact viewer passes no query.
	 *
	 * Run: `bun run dev` in app/, then open /prototype-type-scale.
	 */
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Frame from './Frame.svelte';
	import ScaleReadout from './ScaleReadout.svelte';
	import type { ScreenKey } from './fixtures.js';
	import { OPENING_KEY, SCALES, tokenStyle } from './scales.js';

	const SCREENS: { key: ScreenKey | 'all'; name: string }[] = [
		{ key: 'clients', name: 'Clients list' },
		{ key: 'question', name: 'Question page' },
		{ key: 'record', name: 'Client record' },
		{ key: 'overview', name: 'Practice overview' },
		{ key: 'all', name: 'Every screen' }
	];
	const NARROW_WIDTHS = [320, 360, 400];

	const initial = new URLSearchParams(globalThis.location?.search ?? '');
	let scaleKey = $state(initial.get('variant') ?? OPENING_KEY);
	let screenKey = $state<ScreenKey | 'all'>('clients');
	let narrowPx = $state(360);
	let narrowWidth = $state(0);
	let wideWidth = $state(0);

	const scale = $derived(SCALES.find((each) => each.key === scaleKey) ?? SCALES[0]!);
	const scaleIndex = $derived(SCALES.indexOf(scale));
	const screens = $derived<readonly ScreenKey[]>(
		screenKey === 'all' ? ['clients', 'question', 'record', 'overview'] : [screenKey]
	);

	function cycle(step: number) {
		scaleKey = SCALES[(scaleIndex + step + SCALES.length) % SCALES.length]!.key;
	}

	function onKeydown(event: KeyboardEvent) {
		const target = event.target as HTMLElement | null;
		if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return;
		if (target?.isContentEditable) return;
		if (event.key === 'ArrowLeft') cycle(-1);
		if (event.key === 'ArrowRight') cycle(1);
	}
</script>

<svelte:window onkeydown={onKeydown} />
<PageTitle page="PROTOTYPE #1926 type scale" />

<main class="page">
	<header>
		<h1>Pick a type scale</h1>
		<p>
			Six candidate scales, each with body text at 16px and the same 1.2 growth from a 320px
			column to a 1920px one. The screens below are the real components with a Practice's content
			in them. Use the bar at the bottom or the left and right arrow keys to switch scale.
		</p>
	</header>

	<ScaleReadout {scale} {narrowWidth} {wideWidth} />

	<!-- The one place the prototype overrides the type tokens: scales.ts writes the eight sizes. -->
	<div class="frames" style={tokenStyle(scale)}>
		<Frame
			name="Narrow column"
			prefix="narrow"
			kind="narrow"
			{narrowPx}
			{screens}
			bind:width={narrowWidth}
		/>
		<Frame name="Wide column" prefix="wide" kind="wide" {screens} bind:width={wideWidth} />
	</div>
</main>

<div class="switcher">
	<button type="button" onclick={() => cycle(-1)} aria-label="Previous scale">←</button>
	<span class="label">{scaleIndex + 1} of {SCALES.length}: {scale.name}</span>
	<button type="button" onclick={() => cycle(1)} aria-label="Next scale">→</button>
	<label>
		<span>Screen</span>
		<select bind:value={screenKey}>
			{#each SCREENS as option (option.key)}
				<option value={option.key}>{option.name}</option>
			{/each}
		</select>
	</label>
	<label>
		<span>Narrow column</span>
		<select bind:value={narrowPx}>
			{#each NARROW_WIDTHS as option (option)}
				<option value={option}>{option}px</option>
			{/each}
		</select>
	</label>
</div>

<style>
	/*
	 * The artifact viewer wraps the page in its own reset, unlayered, which
	 * sets a system font and a light body over the app's layered base.css.
	 * Restated here, unlayered too, so the screens read in the app's own
	 * face and colors in both themes.
	 */
	:global(body) {
		font-family: var(--font-family-base);
		line-height: var(--text-body-leading);
		color: var(--color-on-surface);
		background-color: var(--color-surface);
	}

	.page {
		display: grid;
		gap: var(--space-6);
		padding: var(--space-5) var(--page-gutter) var(--space-12);
	}

	header {
		display: grid;
		gap: var(--space-2);
		max-inline-size: var(--measure);
	}

	h1 {
		margin: 0;
		font-size: var(--text-heading-size);
		font-weight: var(--text-heading-weight);
	}

	p {
		margin: 0;
	}

	.frames {
		display: flex;
		flex-wrap: wrap;
		align-items: start;
		gap: var(--space-6);
	}

	.switcher {
		position: fixed;
		inset-block-end: var(--space-3);
		inset-inline: var(--space-2);
		z-index: 20;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: var(--space-1) var(--space-3);
		margin-inline: auto;
		inline-size: fit-content;
		max-inline-size: 100%;
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius);
		background: var(--color-on-surface);
		color: var(--color-surface);
		font-size: var(--text-meta-size);
	}

	.switcher button {
		border: 0;
		padding: var(--space-1) var(--space-2);
		background: none;
		color: inherit;
		font: inherit;
		cursor: pointer;
	}

	label {
		display: flex;
		align-items: center;
		gap: var(--space-1);
	}
</style>
