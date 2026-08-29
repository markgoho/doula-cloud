<script lang="ts">
	import Heading from '#lib/components/atoms/Heading.svelte';
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
			<Heading level={1} text="Level 1 — defaults to page" />
			<Heading level={2} text="Level 2 — defaults to section" />
			<Heading level={3} text="Level 3 — defaults to card" />
			<Heading level={4} text="Level 4 — defaults to card" />
			<Heading level={5} text="Level 5 — defaults to card" />
			<Heading level={6} text="Level 6 — defaults to card" />
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
			<Heading level={2} variant="page" text="Page — a page title" />
			<Heading level={2} variant="section" text="Section — a section heading" />
			<Heading level={2} variant="card" text="Card — a card or group title" />
		</stack-l>
	</section>
</stack-l>
