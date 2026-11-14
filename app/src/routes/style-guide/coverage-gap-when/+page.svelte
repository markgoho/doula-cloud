<script lang="ts">
	import CoverageGapWhen from '#lib/components/molecules/CoverageGapWhen.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	const now = new Date('2026-11-14T12:00:00-05:00');
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Coverage Gap When" />
	<Text
		text="When a Doula cannot be reached: both ends with their clock, each in a time that carries the exact instant. The On call roster and the Engagement's On call section both draw a gap's times with it."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<h2>Over a night</h2>
		<div>
			<CoverageGapWhen
				startsAt="2026-11-14T18:00:00-05:00"
				endsAt="2026-11-15T09:00:00-05:00"
				timeZone="America/New_York"
				{now}
			/>
		</div>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>Within one day: the day is said once</h2>
		<div>
			<CoverageGapWhen
				startsAt="2026-11-14T18:00:00-05:00"
				endsAt="2026-11-14T21:00:00-05:00"
				timeZone="America/New_York"
				{now}
			/>
		</div>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>Into another year: the year is said</h2>
		<div>
			<CoverageGapWhen
				startsAt="2026-12-31T20:00:00-05:00"
				endsAt="2027-01-01T10:00:00-05:00"
				timeZone="America/New_York"
				{now}
			/>
		</div>
	</stack-l>
</stack-l>
