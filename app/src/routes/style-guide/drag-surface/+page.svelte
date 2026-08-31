<script lang="ts">
	import Select from '#lib/components/atoms/Select.svelte';
	import {
		atomPages,
		moleculePages,
		organismPages,
		templatePages
	} from '../components.js';
	import { toDemos, type PageModule } from './dragSurface.js';

	/*
	 * The glob lives here rather than in `dragSurface.ts` so that no test
	 * imports it: it is eager, so importing it drags every component in the
	 * app into whichever coverage report is running, at nearly zero
	 * execution. Excluding this route's own page keeps the cycle out.
	 */
	const pageModules = import.meta.glob<PageModule>(
		['../*/+page.svelte', '!../drag-surface/+page.svelte'],
		{ eager: true }
	);

	const demos = toDemos(pageModules, [
		...atomPages,
		...moleculePages,
		...organismPages,
		...templatePages
	]);

	/*
	 * The drag surface (CONTEXT.md, ADR-0025): a component watched passing
	 * through its configurations continuously, rather than inspected at
	 * chosen sizes. Nothing here offers a width to pick -- the reader
	 * sweeps the range and sees the stages nobody would have sampled.
	 */

	/*
	 * The one width this repo's verification may name: WCAG 1.4.10 at 400%
	 * zoom (ADR-0024). It is a conformance commitment, not a content floor,
	 * and it is the low end of the handle rather than a size on offer.
	 */
	const CONFORMANCE_COMMITMENT = 320;

	const names = demos.map((demo) => demo.name);

	let selectedName = $state(names[0] ?? '');
	const selected = $derived(demos.find((demo) => demo.name === selectedName));

	// The space this page itself is given, which is as wide as the frame
	// can go: a full-width page with no browser window resized.
	let availableSpace = $state(0);
	/*
	 * What the handle has been dragged to, and what the frame actually got.
	 * They are read separately so the readout reports the rendered size
	 * rather than the request. Undragged means "as wide as there is", so
	 * the handle starts at the far end rather than at a chosen number.
	 */
	let draggedTo = $state<number | undefined>();
	let frameSpace = $state(0);

	const widestSpace = $derived(Math.max(availableSpace, CONFORMANCE_COMMITMENT));
	const frameWidth = $derived(Math.min(draggedTo ?? widestSpace, widestSpace));
</script>

<stack-l space="var(--space-6)">
	<h1>Drag surface</h1>

	<p>
		Drag the handle to change the space the component below is given. The frame is a containment
		context, so what you see is the component reacting to its own available space -- not to the
		window, which never moves.
	</p>

	<label>
		Component
		<Select options={names} bind:value={selectedName} />
	</label>

	<div class="handle">
		<label for="available-space">Available space</label>
		<output for="available-space">{frameSpace}px</output>
		<!-- No range atom exists, and the block on new components (CLAUDE.md)
		     is still on, so this handle stays a raw input for now (#492). -->
		<!-- eslint-disable-next-line svelte/no-restricted-html-elements -->
		<input
			id="available-space"
			type="range"
			min={CONFORMANCE_COMMITMENT}
			max={widestSpace}
			bind:value={() => frameWidth, (value) => (draggedTo = value)}
		/>
	</div>

	<div class="run" bind:clientWidth={availableSpace}>
		<div class="frame" style:inline-size="{frameWidth}px" bind:clientWidth={frameSpace}>
			{#if selected}
				<selected.component />
			{/if}
		</div>
	</div>
</stack-l>

<style>
	@layer components {
		.handle {
			display: grid;
			grid-template-columns: 1fr auto;
			gap: var(--space-2);
			align-items: baseline;
		}

		.handle input {
			grid-column: 1 / -1;
			inline-size: 100%;
		}

		output {
			font-variant-numeric: tabular-nums;
		}

		/*
		 * The frame is allowed to be escaped: a component that needs more
		 * room than it is given crosses this border in plain sight, which
		 * is the whole point of looking. The scroll is one level up so the
		 * handle stays where it is while that happens.
		 */
		.run {
			overflow-x: auto;
		}

		/*
		 * No padding and no border, so the number in the readout is exactly
		 * the space the component was given. An outline is drawn instead of
		 * a border because it takes no room: a frame that says 320px while
		 * handing the component 286px is the lie this surface exists to
		 * stop telling.
		 */
		.frame {
			container-type: inline-size;
			background-color: var(--color-surface);
			outline: var(--border-thin) solid var(--color-outline);
		}

		/*
		 * The base size re-resolved against the frame (#544). Without it
		 * the instrument lies in the same direction every time: text that
		 * nobody sized keeps the size computed outside the frame, so the
		 * component under test is measured holding letters meant for a
		 * 1425px container while the readout says 320px.
		 */
		.frame > :global(*) {
			font-size: var(--text-body-size);
		}
	}
</style>
