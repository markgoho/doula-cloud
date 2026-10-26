<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isStepShown?: boolean;
	}

	/*
	 * The step heading that takes focus when it appears (#2018) is its
	 * own subject: mounting it is the moment it appears, so loading this
	 * variant shows focus land on it, the way "Before you continue"
	 * replaces a sign-in form.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Heading, a step heading that took focus', props: { isStepShown: true } }
	];
</script>

<script lang="ts">
	import Heading from '#lib/components/atoms/Heading.svelte';

	let { isStepShown = false }: Properties = $props();
</script>

<stack-l space="var(--space-6)">
	<h1>Heading</h1>

	<section>
		<h2>Levels, with the default variant</h2>
		<p>
			<code>level</code> is document structure and sets the screen-reader outline. Left alone, it
			picks the variant a heading at that depth usually wants.
		</p>
		<stack-l space="var(--space-4)">
			<Heading level={1} text="Level 1, page — Anne-Marie Ochieng-Whitfield" />
			<Heading level={2} text="Level 2, section — Engagement with Highland Midwifery &amp; Birth Support" />
			<Heading level={3} text="Level 3, card — Contract signed Sep 14, 2027" />
			<Heading level={4} text="Level 4, card — Contract signed Sep 14, 2027" />
			<Heading level={5} text="Level 5, card — Contract signed Sep 14, 2027" />
			<Heading level={6} text="Level 6, card — Contract signed Sep 14, 2027" />
		</stack-l>
	</section>

	<section>
		<h2>Variants, chosen independently</h2>
		<p>
			Every heading below is an <code>h2</code>, so the outline is unchanged — only the visual
			role differs. The brief's <code>display</code> step is deliberately not offered here: it is
			allowed on one page title on a hub, so the OverviewHub Template owns it.
		</p>
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="page" text="Page — Anne-Marie Ochieng-Whitfield" />
			<Heading level={2} variant="section" text="Section — Engagement with Highland Midwifery &amp; Birth Support" />
			<Heading level={2} variant="card" text="Card — Contract signed Sep 14, 2027" />
		</stack-l>
	</section>

	<section>
		<h2>Taking focus when it appears</h2>
		<p>
			<code>focusOnAppearing</code> gives the heading <code>tabindex="-1"</code> and moves focus to
			it the moment it renders, the GOV.UK pattern for a step that replaces a form. The sign-in
			pages' "Before you continue" step uses it. Choose the "step heading that took focus" state
			on the drag surface to see it land.
		</p>
		{#if isStepShown}
			<Heading level={2} text="Before you continue" focusOnAppearing />
		{/if}
	</section>
</stack-l>
