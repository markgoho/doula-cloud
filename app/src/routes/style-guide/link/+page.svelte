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

	<!-- The floor check measures `#start` against a reading measure (#2145). -->
	<section id="start">
		<h2>Start button</h2>
		<p>The one navigation that starts a screen's task, drawn as the primary button. Once on a screen at most.</p>
		<stack-l space="var(--space-4)">
			<Link label="Invite a Staff member" href="/style-guide" variant="start" />
			<Link label="Answer the website question so Stripe can see your Practice online" href="/style-guide" variant="start" />
		</stack-l>
	</section>

	<section>
		<h2>Filter chips</h2>
		<p>The applied filter is drawn apart from the others.</p>
		<cluster-l space="var(--space-3)">
			<Link label="All invoices" href="/style-guide" variant="chip" current />
			<Link label="Overdue (1)" href="/style-guide" variant="chip" />
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
