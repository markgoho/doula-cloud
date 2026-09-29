<script lang="ts">
	/*
	 * Archetype G -- the document view. ADR-0018.
	 *
	 * #405 named G ("Document / print view -- portal `birth-plan`,
	 * `contract`") and left it without a Template. #1574 found three routes
	 * of that shape -- the portal Birth Plan, Contract and Invoice -- and
	 * none of them had a frame that stood in every state: Birth Plan and
	 * Contract had none at all, and the Invoice hand-wrote one on the route.
	 * Three consumers is past the extraction bar, so G gets its own Template.
	 *
	 * `empty` is a string, not `OverviewHub`'s `isEmpty`/`empty` Snippet
	 * pair: a document that does not exist yet is one sentence ("No Contract
	 * has been sent for your care yet."), and presence-is-state keeps it the
	 * same shape as `loading` and `loadError` beside it. A hub's empty state
	 * is a whole body; a document's is a line.
	 *
	 * The back link is `backHref`/`backLabel`, `QuestionPage`'s shape, not a
	 * Snippet: this Template owns `BackLink`, so it can also own hiding it on
	 * paper. A printed Birth Plan is the document, not the way out of it.
	 */
	import type { Snippet } from 'svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	interface Properties {
		/**
		 * The one `<h1>` and the tab title, in every state. A route whose
		 * title depends on its data passes a generic name until it arrives.
		 */
		title: string;
		/**
		 * The Staff side leaves this unset and gets the product name; the
		 * Client portal passes its Practice's name (#431, #487).
		 */
		serviceName?: string;
		backHref: string;
		backLabel?: string;
		content: Snippet;
		/**
		Presence is the state, value is the Skeleton's accessible label (#480).
		*/
		loading?: string;
		/**
		Presence is the state, value is the Notice's message (#480).
		*/
		loadError?: string;
		/**
		Presence is the state, value is the "none yet" sentence.
		*/
		empty?: string;
	}

	let { title, serviceName, backHref, backLabel, content, loading, loadError, empty }: Properties = $props();
</script>

<PageTitle page={title} {serviceName} />

<container-l>
	<center-l max="var(--measure)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-5)">
			<div class="no-print"><BackLink href={backHref} label={backLabel} /></div>

			{#if loadError}
				<!-- data-load-error: see FormPage.svelte's own comment on this
				     attribute (#1258) -- the same fact, the same reason. -->
				<stack-l space="var(--space-7)" data-load-error>
					<Heading level={1} variant="page" text={title} />
					<Notice variant="error" message={loadError} />
				</stack-l>
			{:else if loading}
				<stack-l space="var(--space-7)">
					<Heading level={1} variant="page" text={title} />
					<Skeleton variant="text" lines={6} label={loading} />
				</stack-l>
			{:else if empty}
				<stack-l space="var(--space-7)">
					<Heading level={1} variant="page" text={title} />
					<Text text={empty} />
				</stack-l>
			{:else}
				<stack-l space="var(--space-7)">
					<Heading level={1} variant="page" text={title} />
					{@render content()}
				</stack-l>
			{/if}
		</stack-l>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		@media print {
			.no-print {
				display: none;
			}
		}
	}
</style>
