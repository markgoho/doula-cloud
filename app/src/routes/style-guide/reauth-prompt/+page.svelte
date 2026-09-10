<script lang="ts">
	/*
	 * `ReauthPrompt` talks to Identity Platform the moment either button is
	 * pressed, so this catalog page shows the two shapes it takes rather
	 * than driving one -- what the component looks like when a page owns
	 * the surrounding form, and what it looks like when it brings its own.
	 * A refusal is realized by writing the submission's `errors` directly,
	 * the same public field a client-side check writes.
	 */
	import ReauthPrompt from '#lib/components/molecules/ReauthPrompt.svelte';
	import { FormSubmission } from '#lib/formSubmission.svelte.js';

	const plain = new FormSubmission();
	const refused = new FormSubmission();
	refused.errors = [{ message: 'Password is not correct', targetId: 'style-guide-refused-password' }];

	const nested = new FormSubmission();

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
		<h2>Inside a form the page already owns</h2>
		<!-- stacked-form:ignore: #1108 -- the point of this example is the `insideForm` branch, which exists because HTML forbids a nested form. The bare `<form>` is the page's own outer form being demonstrated; `ReauthPrompt` brings a `StackedForm` in the other branch above. -->
		<form onsubmit={(event) => event.preventDefault()}>
			<ReauthPrompt
				idPrefix="style-guide-nested"
				email="anne-marie@example.test"
				prompt="Confirm your password to remove two-factor authentication."
				confirmLabel="Remove"
				confirmVariant="destructive"
				submission={nested}
				onAuthenticated={noop}
				onCancel={noop}
				insideForm
			/>
		</form>
	</section>
</stack-l>
