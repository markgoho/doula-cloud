<script lang="ts">
	/*
	 * GOV.UK's Back link, which is a real named component rather than a
	 * link that happens to point backwards: it sits at the very top of the
	 * page above everything else including the error summary, it is quieter
	 * than prose, and its word is "Back".
	 *
	 * Extracted on #464 because `QuestionPage` and `CheckAnswers` want the
	 * identical thing, which is ADR-0018's bar -- one consumer stays a raw
	 * exception, two identical consumers earn the extraction. Without it,
	 * "the word is Back" and "it is body-sm" are copied into two
	 * stylesheets and drift the first time one of them is touched.
	 *
	 * `label` is open because a back link may name where it goes -- GOV.UK
	 * allows "Back to your practices" -- and the account page already has
	 * one of those written by hand (#474).
	 */
	import Link from '#lib/components/atoms/Link.svelte';

	interface Properties {
		href: string;
		label?: string;
	}

	let { href, label = 'Back' }: Properties = $props();
</script>

<div class="back">
	<Link {href} {label} variant="secondary" icon="arrow-left" />
</div>

<style>
	@layer components {
		/* Quieter than the question it sits above: it is a way out, not the
		   thing the page is for. */
		.back {
			font-size: var(--text-body-sm-size);
		}
	}
</style>
