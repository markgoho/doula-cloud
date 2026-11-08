<script lang="ts">
	import PilotFeedback from '#lib/components/organisms/PilotFeedback.svelte';

	/*
	 * Static wiring, no real send: `onSend` never reaches `#lib/api.js`, so a
	 * click on Send here is harmless, as on the Staff feedback page.
	 */
</script>

<stack-l space="var(--space-6)">
	<h1>Pilot feedback</h1>

	<section>
		<h2>With a role and Practice line</h2>
		<PilotFeedback
			email="jordan@fingerlakesbirth.example"
			bannerSentence="This care portal is new."
			controlText="Tell us what is not working or what you need"
			drawerHeading="Send feedback about this portal"
			introText="Finger Lakes Birth Collective uses DoulaCloud to run this portal. The DoulaCloud team reads every piece of feedback."
			destination="This goes to the DoulaCloud team, not to Finger Lakes Birth Collective."
			sentBy="the DoulaCloud team"
			roleAndPractice="Client, Finger Lakes Birth Collective"
			onSend={async () => {}}
		/>
	</section>

	<section>
		<h2>With no banner sentence and no role line</h2>
		<PilotFeedback
			email="jordan@fingerlakesbirth.example"
			bannerSentence=""
			controlText="Tell us what you think"
			drawerHeading="Send feedback to DoulaCloud"
			introText="The DoulaCloud team reads every piece of feedback during the pilot."
			destination="This goes to the DoulaCloud team, not to your Practice."
			sentBy="Mark Goho, who builds DoulaCloud,"
			onSend={async () => {}}
		/>
	</section>
</stack-l>
