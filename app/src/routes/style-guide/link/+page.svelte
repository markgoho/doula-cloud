<script lang="ts">
	import Link from '#lib/components/atoms/Link.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Link</h1>

	<section>
		<h2>Variants</h2>
		<cluster-l space="var(--space-4)">
			<Link label="Primary" href="/style-guide" variant="primary" />
			<Link label="Secondary" href="/style-guide" variant="secondary" />
		</cluster-l>
	</section>

	<section>
		<h2>Internal link</h2>
		<cluster-l space="var(--space-4)">
			<Link label="Back to style guide" href="/style-guide" />
		</cluster-l>
	</section>

	<section>
		<h2>External link</h2>
		<p>Absolute and protocol-relative hrefs open in a new tab with an indicator icon.</p>
		<cluster-l space="var(--space-4)">
			<Link label="Svelte docs" href="https://svelte.dev/docs/kit" />
		</cluster-l>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any link on this page to see its focus outline.</p>
	</section>
</stack-l>
