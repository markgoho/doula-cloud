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
	 * same shape as the other optional props beside it. A hub's empty state
	 * is a whole body; a document's is a line. That line is drawn as an
	 * `EmptyState` (#2015), with `emptyAction` beside it where the reader has
	 * one.
	 *
	 * The back link is `backHref`/`backLabel`, `QuestionPage`'s shape, not a
	 * Snippet: this Template owns `BackLink`, so it can also own hiding it on
	 * paper. A printed Birth Plan is the document, not the way out of it.
	 */
	import type { Snippet } from 'svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import PageReadPending from '#lib/components/molecules/PageReadPending.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import EmptyState, { type EmptyStateAction } from '#lib/components/molecules/EmptyState.svelte';
	import { LOADED_PAGE, type PageReadState, type PageTitling } from '#lib/pageRead.svelte.js';

	/**
	A route whose title depends on its data passes a generic name until it arrives.
	*/
	interface Properties extends PageTitling {
		backHref: string;
		backLabel?: string;
		content: Snippet;
		/**
		 * The document's own read (#480, `pageRead.svelte.ts`): loading,
		 * with the Skeleton's accessible label; failed, with the Notice's
		 * message; or loaded, which is also what leaving it out means.
		 */
		read?: PageReadState;
		/**
		Presence is the state, value is the "none yet" sentence. Drawn only
		once `read` is loaded.
		*/
		empty?: string;
		/**
		 * The one action that makes the document, shown with `empty`.
		 */
		emptyAction?: EmptyStateAction;
	}

	let {
		title,
		serviceName,
		backHref,
		backLabel,
		content,
		read = LOADED_PAGE,
		empty,
		emptyAction
	}: Properties = $props();
</script>

<PageTitle page={title} {serviceName} isPending={read.status === 'loading'} />

<container-l>
	<center-l max="var(--measure)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-5)">
			<div class="no-print"><BackLink href={backHref} label={backLabel} /></div>

			{#if read.status === 'failed'}
				<!-- data-load-error: see FormPage.svelte's own comment on this
				     attribute (#1258) -- the same fact, the same reason. -->
				<stack-l space="var(--space-group)" data-load-error>
					<PageReadPending {title} {read} skeleton="text" />
				</stack-l>
			{:else if read.status === 'loading'}
				<stack-l space="var(--space-group)">
					<!-- `text`, not ListPage's `row`: what is coming is prose and
					     a list of facts, not table rows. -->
					<PageReadPending {title} {read} skeleton="text" />
				</stack-l>
			{:else if empty}
				<stack-l space="var(--space-group)">
					<Heading level={1} variant="page" text={title} />
					<EmptyState message={empty} action={emptyAction} />
				</stack-l>
			{:else}
				<stack-l space="var(--space-group)">
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

		/* Centered in spare room by default (#543); a shell that holds
		   every page to one edge sets the property (ADR-0018). */
		center-l {
			margin-inline: var(--page-column-margin-inline, auto);
		}

		/* A print query, not a width one (ADR-0024 allows it): on paper the
		   back link is a way out of nothing. */
		@media print {
			.no-print {
				display: none;
			}
		}
	}
</style>
