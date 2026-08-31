<script lang="ts">
	import CloudMark from '#lib/components/atoms/CloudMark.svelte';

	const sizes = ['sm', 'md', 'lg'] as const;
</script>

<stack-l space="var(--space-6)">
	<h1>Cloud mark</h1>

	<section>
		<h2>Three sizes, one stroke</h2>
		<p>
			Only the width and height vary. An SVG stroke is in viewBox units and scales with the frame,
			so one <code>stroke-width</code> produces the weight ramp the canvas has to state by hand —
			pen.dev's <code>strokeWidth</code> is node pixels and does not scale (#411).
		</p>
		<cluster-l space="var(--space-8)" align="center">
			{#each sizes as size (size)}
				<stack-l space="var(--space-2)">
					<CloudMark {size} />
					<span>{size}</span>
				</stack-l>
			{/each}
		</cluster-l>
	</section>

	<section>
		<h2>Named, when it stands alone</h2>
		<p>
			Decorative by default, because it almost always sits beside the wordmark and naming it there
			would make a screen reader say the product's name twice. A caller using the mark on its own
			passes a label.
		</p>
		<!--
			No hostile value exists for this component (ADR-0025): the mark takes
			no Practice content at all, and its one string is the product's own
			name, which never gets longer.
		-->
		<CloudMark size="md" label="Doula Cloud" />
	</section>
</stack-l>
