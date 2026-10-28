<script lang="ts">
	/*
	 * What a Practice screen draws when its read fails: the page
	 * `practices/+error.svelte` draws, with the same way out. A read a
	 * `load` hands over unawaited (#2249) fails after the screen has
	 * changed, so the screen draws this itself.
	 */
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import type { ErrorKind } from '#lib/errorPage.js';

	let { kind }: { kind: ErrorKind } = $props();
</script>

<ErrorPage
	{kind}
	wayOutHref={resolve('/practices/[practiceId]', { practiceId: page.params.practiceId! })}
	wayOutLabel="Go to your Practice overview"
/>
