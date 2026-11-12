<script lang="ts">
	/*
	 * Stands in for a route's loaded screen in a spec of the `+page.svelte`
	 * that hands it a failed read to try again (#2746): it says whether the
	 * read failed, and offers the page's retry.
	 */
	import Button from '#lib/components/atoms/Button.svelte';

	let { hasReadFailed, onRetry }: { hasReadFailed: boolean; onRetry: () => Promise<void> } = $props();
</script>

<p title={String(hasReadFailed)}>Read failed</p>
<Button label="Retry" onClick={() => onRetry()} />
