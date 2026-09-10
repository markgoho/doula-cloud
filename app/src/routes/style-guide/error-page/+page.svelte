<script lang="ts">
	/*
	 * Every kind, not just notFound. #475 walked govuk-alignment.md's
	 * Aligned rows in a browser and found that Service unavailable and
	 * There is a problem could not be walked at all -- the only place either
	 * template state renders is a real 503 or 500, which no local stack
	 * produces on demand. A style-guide entry that shows one of a
	 * component's states hides the rest. #918 added two more of them --
	 * the two 403 reasons that are not a role refusal, neither of which
	 * a local stack produces on demand either.
	 */
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import type { ErrorKind } from '#lib/errorPage.js';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): the
	 * body of each state is the product's own wording and cannot grow, so
	 * the only field a Practice's data reaches is the way out -- given here
	 * as the longest label the app puts on one.
	 */
	const kinds: { kind: ErrorKind; heading: string; wayOutLabel: string }[] = [
		{
			kind: 'notFound',
			heading: 'Page not found (404)',
			wayOutLabel: 'Go to the Highland Midwifery Practice overview'
		},
		{
			kind: 'refused',
			heading: 'Refused by role (403)',
			wayOutLabel: 'Go to the Highland Midwifery Practice overview'
		},
		{
			kind: 'practiceLocked',
			heading: 'Practice locked (403 PRACTICE_PENDING_DELETION)',
			wayOutLabel: 'Go to the Highland Midwifery Practice overview'
		},
		{
			kind: 'secondFactor',
			heading: 'Second sign-in factor (403 MFA_REQUIRED)',
			wayOutLabel: 'Go to the Highland Midwifery Practice overview'
		},
		{
			kind: 'unavailable',
			heading: 'Service unavailable (503)',
			wayOutLabel: 'Go to the Highland Midwifery Practice overview'
		},
		{
			kind: 'problem',
			heading: 'There is a problem (500)',
			wayOutLabel: 'Go to the Highland Midwifery Practice overview'
		}
	];
</script>

{#each kinds as { kind, heading, wayOutLabel } (kind)}
	<h2>{heading}</h2>
	<ErrorPage {kind} wayOutHref="/practices/practice-1" {wayOutLabel} />
{/each}
