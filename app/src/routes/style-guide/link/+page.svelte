<script lang="ts">
	import Link from '#lib/components/atoms/Link.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Link</h1>

	<section>
		<h2>Variants</h2>
		<cluster-l space="var(--space-4)">
			<Link label="Back to the engagement request you were deciding" href="/style-guide" variant="primary" />
			<Link label="Back to the engagement request you were deciding" href="/style-guide" variant="secondary" />
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
		<!--
			The longest realistic value, not a representative one (ADR-0025): a
			Practice's own site is what an external Link usually points at, and a
			Practice that has not written link text gets its URL as the label. A
			URL is the one value with no break opportunity a browser will take
			(#521), so it is what decides how narrow this component survives.
		-->
		<cluster-l space="var(--space-4)">
			<Link
				label="https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake"
				href="https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake"
			/>
		</cluster-l>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any link on this page to see its focus outline.</p>
	</section>
</stack-l>
