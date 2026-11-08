<script lang="ts">
	import { PORTAL_DOOR_LINE } from '#lib/portalDoor.js';
	import DoorLink from '#lib/components/atoms/DoorLink.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Door link" />
	<Text
		text="One link whose whole area is the target, with the door's name and one short line under it. The name alone is the link's accessible name; the line is its description. The signed-out landing has two bordered doors, and lights the one this device used last."
		tone="variant"
	/>

	<!--
		The landing's own two doors, the Client door lit as the one this
		device used last. Their words are fixed product copy and take no
		Practice content, so there is no hostile value to sweep (ADR-0025);
		they sit in the same wrapping grid the landing uses, so the sweep sees
		them side by side and stacked. The primary door below has no consumer
		on the landing now; it stays as the atom's other look.
	-->
	<stack-l space="var(--space-4)">
		<grid-l min="var(--door-min)" space="var(--space-4)">
			<DoorLink
				href="/style-guide/door-link"
				label="Staff sign in"
				description="For doulas, practice owners, and office staff."
			/>
			<DoorLink
				href="/style-guide/door-link"
				label="Client sign in"
				description={PORTAL_DOOR_LINE}
				lastUsed
			/>
		</grid-l>
		<DoorLink
			href="/style-guide/door-link"
			label="Set up a Practice"
			description="New here? Your first three clients are free."
			variant="primary"
		/>
	</stack-l>
</stack-l>
