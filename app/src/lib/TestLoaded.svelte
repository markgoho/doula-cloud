<script lang="ts">
	/*
	 * Stands in for a route's loaded screen in a spec of the `+page.svelte`
	 * that streams its read (#2249): it says it was drawn, and with what.
	 */
	let { data }: { data: unknown } = $props();
</script>

<p title={JSON.stringify(data)}>Loaded</p>
