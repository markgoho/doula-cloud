<script lang="ts">
	import Details from '#lib/components/atoms/Details.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Drawer from '#lib/components/organisms/Drawer.svelte';

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
			The Feedback drawer's own decided copy (#1502 Q4), read here
			ahead of its own ticket (#1527) landing -- the same move
			/style-guide/details already made for the disclosure below,
			since #1520 named this drawer as that atom's newest consumer.
			Real content rather than lorem ipsum, and long enough that the
			sweep measures this panel at its busiest state.
		-->
		<Drawer bind:open={isOpen} heading="Send feedback to Doula Cloud">
			<Text
				text="The Doula Cloud team reads every piece of feedback during the pilot. It is how we decide what to fix first."
			/>
			<Text text="What kind of feedback is it?" step="label" />
			<Text text="Something is not working / An idea or a request / Something else" />
			<Text text="Tell us more" step="label" />
			<Text text="What were you trying to do, and what happened?" tone="muted" />
			<Details summary="What else we send with your feedback">
				<ul>
					<li>The page you were on</li>
					<li>Your role and Practice</li>
					<li>The time</li>
					<li>Your screen width and browser</li>
					<li>The version of Doula Cloud</li>
				</ul>
			</Details>
			<Text text="This goes to the Doula Cloud team, not to your Practice." tone="muted" />
		</Drawer>
	</section>
</stack-l>
