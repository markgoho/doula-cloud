<script lang="ts">
	import FeedbackForm from '#lib/components/organisms/FeedbackForm.svelte';

	/*
	 * The Staff copy (#1502 Q4) -- the longer of the two shells' wording
	 * (ADR-0025), and real content rather than lorem ipsum, matching the
	 * Drawer style-guide page's own Feedback demo. No real send: the
	 * "posted" line below reads back exactly what `onSend` received,
	 * which is what a person clicking Send here should see happen.
	 */
	let posted = $state('');
</script>

<stack-l space="var(--space-6)">
	<h1>Feedback form</h1>

	<section>
		<h2>Staff copy, with a role and Practice in the disclosure</h2>
		<FeedbackForm
			legend="What kind of feedback is it?"
			errorKind="Select what kind of feedback it is"
			textLabel="Tell us more"
			textHint="What were you trying to do, and what happened?"
			destination="This goes to the Doula Cloud team, not to your Practice."
			detailsSummary="What else we send with your feedback"
			roleAndPractice="Owner, Finger Lakes Birth Collective"
			onSend={async (input) => {
				posted = JSON.stringify(input, undefined, 2);
			}}
			onSent={() => {}}
		/>
		{#if posted}
			<pre>{posted}</pre>
		{/if}
	</section>
</stack-l>

<style>
	@layer components {
		pre {
			padding: var(--space-3);
			background-color: var(--color-surface-container);
			border-radius: var(--radius);
			font-size: var(--text-body-sm-size);
			white-space: pre-wrap;
		}
	}
</style>
