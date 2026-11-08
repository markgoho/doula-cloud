<script lang="ts">
	import PilotBanner from '#lib/components/molecules/PilotBanner.svelte';

	/*
	 * Static rather than wired to a real Drawer (#1521): this page shows the
	 * shape (#1522), not the Staff/Portal wiring, which is #1527/#1528's own
	 * ticket. `open` stays `false` -- there is no drawer here for
	 * `aria-expanded` to follow -- and `onOpenFeedback` is a no-op.
	 */
	function onOpenFeedback() {}
</script>

<stack-l space="var(--space-6)">
	<h1>Pilot banner</h1>

	<section>
		<h2>Staff</h2>
		<!-- The Staff copy is kept to one line down to 320px: the tag and a
		     short control, with no sentence between them (#1502 Q4). The
		     Portal's below is the longer pair the continuum sweep has to
		     clear at 320px. -->
		<PilotBanner
			sentence=""
			controlText="Tell us what you think"
			open={false}
			controlsId="feedback-drawer-staff"
			{onOpenFeedback}
		/>
	</section>

	<section>
		<h2>Portal</h2>
		<PilotBanner
			sentence="This care portal is new."
			controlText="Tell us what is not working or what you need"
			open={false}
			controlsId="feedback-drawer-portal"
			{onOpenFeedback}
		/>
	</section>
</stack-l>
