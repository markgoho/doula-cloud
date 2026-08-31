<script lang="ts">
	import ContactCardFinished from './ContactCardFinished.svelte';
	import ContactCardStart from './ContactCardStart.svelte';
	import { exerciseFields } from './fields.js';

	/*
	 * The layout exercise (#534). A session arrives here from the
	 * continuum check's failure message, not from a link it was told to
	 * follow -- see `continuum.ts`.
	 *
	 * Both cards are shown at the conformance commitment rather than in a
	 * frame with a handle on it: the drag surface already exists at
	 * /style-guide/drag-surface and CONTEXT.md defines it and the
	 * continuum check as one artifact, so a second draggable frame here
	 * would be a third. 320 is the one width this repo's verification may
	 * name (ADR-0024).
	 */
	const CONFORMANCE_COMMITMENT = 320;
</script>

<stack-l space="var(--space-6)">
	<h1>Layout exercise: a card that has one configuration</h1>

	<section>
		<h2>The task</h2>
		<p>
			<code>ContactCardStart.svelte</code> has exactly one configuration at every available space: a
			label track as wide as its widest label, and a value track taking whatever is left. Give it a
			content floor so that it has more than one, and make a value with no break opportunity in it
			-- a URL somebody pasted into a free-text field -- fit at 320px anyway.
		</p>
		<p>
			You are done when the continuum check finds no break in it from 320px up:
			<code>bun run test -- continuum</code> in <code>app/</code>, or the exercise's own spec next to
			these files.
		</p>
		<p>
			<code>ContactCardFinished.svelte</code> is the answer. Diff it against START when you are done,
			not before -- the diff is the whole of what this exercise says.
		</p>
	</section>

	<section>
		<h2>START, at 320px</h2>
		<p>
			The card needs more room than it is given, and the overflow is why the whole page scrolls
			sideways.
		</p>
		<div class="frame" style:inline-size="{CONFORMANCE_COMMITMENT}px">
			<ContactCardStart fields={[...exerciseFields]} />
		</div>
	</section>

	<section>
		<h2>FINISHED, at 320px</h2>
		<p>The same content, the same space, nothing needing more room than it has.</p>
		<div class="frame" style:inline-size="{CONFORMANCE_COMMITMENT}px">
			<ContactCardFinished fields={[...exerciseFields]} />
		</div>
	</section>
</stack-l>

<style>
	@layer components {
		/*
		 * An outline rather than a border or padding, for the reason #527
		 * recorded: a frame that takes room of its own reports a size the
		 * component inside it never had. An outline takes none.
		 */
		.frame {
			container-type: inline-size;
			outline: var(--border-thin) solid var(--color-outline-variant);
			overflow-x: auto;
		}
	}
</style>
