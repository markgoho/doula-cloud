<script lang="ts">
	import Details from '#lib/components/atoms/Details.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Details</h1>

	<section>
		<h2>Closed</h2>
		<!--
			"Show what has happened" is the Client portal's own summary --
			the activity ledger DataTable's `disclosure` prop wraps on the
			Portal's Engagement page (#486).
		-->
		<Details summary="Show what has happened">
			<p>Everything that has happened, revealed once the reader opens it.</p>
		</Details>
	</section>

	<section>
		<h2>Open, with a long summary</h2>
		<!--
			The longest realistic value, not a representative one
			(ADR-0025): this is the Feedback drawer's own copy, decided on
			#1502 before the drawer itself is built -- #1520 names it as
			this atom's newest consumer. Long enough to wrap at 320px
			rather than overflow it, which is what this page and the
			continuum sweep (ADR-0025) both check.
		-->
		<Details summary="What else we send with your feedback" open>
			<ul>
				<li>The page you were on</li>
				<li>Your role and Practice</li>
				<li>The time</li>
				<li>Your screen width and browser</li>
				<li>The version of Doula Cloud</li>
			</ul>
		</Details>
	</section>

	<section>
		<h2>With a hidden name on the summary</h2>
		<!--
			HistoryDisclosure's own need (#667): every row of the Staff
			roster repeats the same summary, so `hiddenSummary` tells a
			screen reader's list of controls one row's disclosure from the
			next. The name is a real-length hyphenated one for the same
			reason HistoryDisclosure's own style-guide page picks one
			(ADR-0025, #537).
		-->
		<Details summary="Membership history" hiddenSummary="for Anne-Marie Ochieng-Whitfield">
			<p>Joined as a doula on Aug 28, 2026.</p>
		</Details>
	</section>
</stack-l>
