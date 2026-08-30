<script lang="ts">
	/*
	 * All four kinds, not just notFound. #475 walked govuk-alignment.md's
	 * Aligned rows in a browser and found that Service unavailable and
	 * There is a problem could not be walked at all -- the only place either
	 * template state renders is a real 503 or 500, which no local stack
	 * produces on demand. A style-guide entry that shows one of a
	 * component's four states hides the other three.
	 */
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import type { ErrorKind } from '#lib/errorPage.js';

	const kinds: { kind: ErrorKind; heading: string; wayOutLabel: string }[] = [
		{ kind: 'notFound', heading: 'Page not found (404)', wayOutLabel: 'Go to your Practice overview' },
		{ kind: 'refused', heading: 'Refused by role (403)', wayOutLabel: 'Go to your Practice overview' },
		{ kind: 'unavailable', heading: 'Service unavailable (503)', wayOutLabel: 'Go to your Practice overview' },
		{ kind: 'problem', heading: 'There is a problem (500)', wayOutLabel: 'Go to your Practice overview' }
	];
</script>

{#each kinds as { kind, heading, wayOutLabel } (kind)}
	<h2>{heading}</h2>
	<ErrorPage {kind} wayOutHref="/practices/practice-1" {wayOutLabel} />
{/each}
