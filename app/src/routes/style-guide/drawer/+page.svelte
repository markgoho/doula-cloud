<script lang="ts">
	import Text from '#lib/components/atoms/Text.svelte';
	import Drawer from '#lib/components/organisms/Drawer.svelte';
	import FeedbackForm from '#lib/components/organisms/FeedbackForm.svelte';

	/*
	 * Open by default rather than behind a button: the continuum sweep
	 * (ADR-0025) mounts this page and measures it, but never clicks
	 * anything, so a demo that starts closed would leave the panel's own
	 * content forever unmeasured -- the same blind spot a closed
	 * `<details>` had before #710. This is also literally the ticket's
	 * own acceptance criterion: "shows it open with long content, at
	 * 320px and at full width."
	 */
	let isOpen = $state(true);
</script>

<stack-l space="var(--space-6)">
	<h1>Drawer</h1>

	<section>
		<h2>Open, with long content</h2>
		<p>
			Slides in from the inline end and covers the screen; the width the AC names is
			<code>min(28rem, 100%)</code>, so at 320px this panel takes the whole screen (and opens
			as a modal), and past 28rem it sits at a fixed 28rem beside a screen that stays
			readable and scrollable. Drag this page's own window, or the drag surface, past 28rem
			to see the second case; the continuum sweep never reaches it (ADR-0025 caps its own
			sweep at 414px), so only a real width change shows it.
		</p>
		<!--
			The real Feedback drawer (#1527), not an approximation of it:
			`organisms/FeedbackForm.svelte` is `/style-guide/feedback-form`'s
			own subject, so this page holds it inside a Drawer at its
			busiest -- the disclosure's role and Practice line included --
			the same way the panel actually appears on a Staff screen.
		-->
		<Drawer bind:open={isOpen} heading="Send feedback to Doula Cloud">
			<Text
				text="The Doula Cloud team reads every piece of feedback during the pilot. It is how we decide what to fix first."
			/>
			<FeedbackForm
				legend="What kind of feedback is it?"
				errorKind="Select what kind of feedback it is"
				textLabel="Tell us more"
				textHint="What were you trying to do, and what happened?"
				destination="This goes to the Doula Cloud team, not to your Practice."
				detailsSummary="What else we send with your feedback"
				roleAndPractice="Owner, Finger Lakes Birth Collective"
				onSend={async () => {}}
				onSent={() => {}}
			/>
		</Drawer>
	</section>
</stack-l>
