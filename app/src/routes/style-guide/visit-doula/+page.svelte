<script lang="ts">
	import VisitDoula from '#lib/components/molecules/VisitDoula.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Visit Doula" />
	<Text
		text="A Visit's Doula on a Staff list: the name, and an amber badge where a booked Visit still stands under a Doula who was taken off its Engagement. The Schedule and the Engagement page's Visits both draw the Doula cell with it."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<h2>In a table: one line, so a row with a badge is as tall as one without</h2>
		<stack-l space="var(--space-2)">
			<div><VisitDoula name="Priya Raman" isOffEngagement={false} isTracked /></div>
			<div><VisitDoula name="Priya Raman" isOffEngagement isTracked /></div>
		</stack-l>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>On a card: the badge wraps under a long name</h2>
		<stack-l space="var(--space-2)">
			<div><VisitDoula name="Priya Raman" isOffEngagement /></div>
			<div><VisitDoula name="Persephone Vandermeulen-Achterberg, CD(DONA)" isOffEngagement /></div>
		</stack-l>
	</stack-l>
</stack-l>
