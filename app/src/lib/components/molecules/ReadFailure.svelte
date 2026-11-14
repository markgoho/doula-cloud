<script lang="ts">
	/*
	 * What stands in place of a list whose read failed: the sentence, and "Try
	 * again" to run the read that failed. Consumers: the Schedule and the On
	 * call roster.
	 */
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';

	let {
		message,
		onRetry
	}: {
		// What a person reads: no status code and no server text.
		message: string;
		// Runs the read that failed, under the same narrowing.
		onRetry: () => Promise<void>;
	} = $props();

	let isRetrying = $state(false);
	async function tryAgain() {
		isRetrying = true;
		try {
			await onRetry();
		} finally {
			isRetrying = false;
		}
	}
</script>

<stack-l space="var(--space-related)">
	<Notice {message} variant="error" />
	<Button label="Try again" onClick={tryAgain} loading={isRetrying} />
</stack-l>
