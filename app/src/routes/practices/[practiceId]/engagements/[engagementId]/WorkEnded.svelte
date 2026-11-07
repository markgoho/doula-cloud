<script lang="ts">
	/*
	 * #2270: what a contractor reads at the address of an Engagement once
	 * her Attachment on it has ended, in place of "Page not found". The
	 * sentence is the BFF's, with the day it ended; nothing else about the
	 * Engagement reaches her. The words are from three persona consults
	 * on #2270. ErrorPage, with words of its own (#2710), since this is the
	 * page she meets in place of one.
	 */
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import Text from '#lib/components/atoms/Text.svelte';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';

	let { message }: { message: string } = $props();
</script>

{#snippet body()}
	<Text text={message} />
	<Text text="Your fee, the terms and what you were paid stay on Your offers." />
{/snippet}

<ErrorPage
	title="Your work has ended"
	{body}
	wayOutHref={resolve('/practices/[practiceId]/offers', { practiceId: page.params.practiceId! })}
	wayOutLabel="Your offers"
/>
