<script lang="ts">
	/*
	 * What a list's CSV download says in place, under the title row (#2871,
	 * #2954): the refusal, with the button still there to try again, and the
	 * words after a finished download.
	 */
	import Notice from '../atoms/Notice.svelte';

	interface Properties {
		refusal: string;
		downloaded: string;
	}

	let { refusal, downloaded }: Properties = $props();
</script>

{#if refusal}
	<Notice variant="error" message={refusal} />
{/if}
{#if downloaded}
	<Notice variant="status" message={downloaded} />
{/if}
