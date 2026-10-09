<script lang="ts">
	/*
	 * `ReauthPrompt` talks to Identity Platform the moment either button is
	 * pressed, so this catalog page shows the two shapes it takes rather
	 * than driving one -- its default and its destructive variant.
	 * A refusal is realized by writing the submission's `errors` directly,
	 * the same public field a client-side check writes.
	 */
	import ReauthPrompt from '#lib/components/molecules/ReauthPrompt.svelte';
	import { FormSubmission } from '#lib/formSubmission.svelte.js';

	const plain = new FormSubmission();
	const refused = new FormSubmission();
	refused.errors = [{ message: 'Password is not correct', targetId: 'style-guide-refused-password' }];

	const destructive = new FormSubmission();

	function noop() {}
</script>

<stack-l space="var(--space-6)">
	<h1>Reauth prompt</h1>

	<section>
		<h2>Default</h2>
		<ReauthPrompt
			idPrefix="style-guide-plain"
			email="anne-marie@example.test"
			prompt="Confirm your password to send the code."
			confirmLabel="Send the code"
			submission={plain}
			onAuthenticated={noop}
			onCancel={noop}
		/>
	</section>

	<section>
		<h2>Refused</h2>
		<ReauthPrompt
			idPrefix="style-guide-refused"
			email="anne-marie@example.test"
			prompt="Confirm your password to send the code."
			confirmLabel="Send the code"
			submission={refused}
			onAuthenticated={noop}
		/>
	</section>

	<section>
		<h2>Destructive</h2>
		<ReauthPrompt
			idPrefix="style-guide-destructive"
			email="anne-marie@example.test"
			prompt="Confirm your password to remove two-factor authentication."
			confirmLabel="Remove"
			confirmVariant="destructive"
			submission={destructive}
			onAuthenticated={noop}
			onCancel={noop}
		/>
	</section>
</stack-l>
