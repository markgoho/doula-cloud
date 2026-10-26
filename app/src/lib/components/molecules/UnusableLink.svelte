<script module lang="ts">
	/**
	 * The one next step (#2025): a page to go to is a `Link`; a step the
	 * person takes away from DoulaCloud -- asking a Practice to send an
	 * invite again -- has no page to link to, so it is a line of text. Its
	 * shape picks which, so a caller never invents an href.
	 */
	export type UnusableLinkNext = { label: string; href: string } | { text: string };
</script>

<script lang="ts">
	/*
	 * What a screen reached from an emailed link shows when the link cannot
	 * be used (#2025, the #2001 audit's root cause 12): it is not complete,
	 * it has expired, or it was used already. Before it, five such screens
	 * showed the heading and a red box and nothing else, so a person had
	 * nothing to do next. This is the one shape `reset-password` already
	 * had: one plain sentence saying what is wrong, then the one next step.
	 *
	 * Named by its one domain thing, the link that cannot be used
	 * (ADR-0018). GOV.UK's "Confirm an email address" pattern and its
	 * "There is a problem with the service" page both end on what to do
	 * next; see the two rows in docs/design/govuk-alignment.md.
	 *
	 * It renders no error summary and does not mark the title as an error
	 * (#1576): a link that cannot be used is not a refused form.
	 */
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	interface Properties {
		message: string;
		next: UnusableLinkNext;
	}

	let { message, next }: Properties = $props();
</script>

<!--
	One group (#2020), so the sentence and its next step read together
	rather than as two of the page's groups.
-->
<stack-l space="var(--space-related)">
	<Notice variant="error" {message} />
	{#if 'href' in next}
		<Link href={next.href} label={next.label} />
	{:else}
		<Text text={next.text} />
	{/if}
</stack-l>
