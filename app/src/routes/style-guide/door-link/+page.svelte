<script lang="ts">
	import DoorLink from '#lib/components/atoms/DoorLink.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Door link" />
	<Text
		text="One link whose whole area is the target, with the door's name and one short line under it. The name alone is the link's accessible name; the line is its description. The signed-out landing has three: one primary, two bordered."
		tone="variant"
	/>

	<!--
		The landing's own three doors. Their words are fixed product copy and
		take no Practice content, so there is no hostile value to sweep
		(ADR-0025); the bordered pair sits in the same wrapping grid the
		landing uses, so the sweep sees them side by side and stacked.
	-->
	<stack-l space="var(--space-4)">
		<DoorLink
			href="/style-guide/door-link"
			label="Staff log in"
			description="For doulas and practice owners."
			variant="primary"
		/>
		<grid-l min="var(--door-min)" space="var(--space-4)">
			<DoorLink
				href="/style-guide/door-link"
				label="Client portal log in"
				description="Your contract, your birth plan, and your invoices."
			/>
			<DoorLink
				href="/style-guide/door-link"
				label="Set up a Practice"
				description="New here? Your first three clients are free."
			/>
		</grid-l>
	</stack-l>
</stack-l>
