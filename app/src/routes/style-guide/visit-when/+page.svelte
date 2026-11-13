<script lang="ts">
	import VisitWhen from '#lib/components/molecules/VisitWhen.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Visit When" />
	<Text
		text="A Visit's day and time on a Staff list: the weekday-led form with its comma, in a time that carries the exact instant. The Schedule and the Engagement page's Visits both draw the When cell with it."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<h2>In a table: every weekday starts at one place, every clock ends on one edge</h2>
		<stack-l space="var(--space-2)">
			<div><VisitWhen at="2026-11-07T09:00:00-05:00" isTracked /></div>
			<div><VisitWhen at="2026-11-25T14:00:00-05:00" isTracked /></div>
			<div><VisitWhen at="2026-12-14T10:30:00-05:00" isTracked /></div>
		</stack-l>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>On a card: one phrase</h2>
		<div><VisitWhen at="2026-11-13T14:00:00-05:00" /></div>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>No instant yet</h2>
		<div><VisitWhen /></div>
	</stack-l>
</stack-l>
