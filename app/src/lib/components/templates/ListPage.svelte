<script lang="ts">
	/*
	 * Archetype C -- the list screen. ADR-0018.
	 *
	 * Found on #491 while giving eight routes with no page frame a Template:
	 * `practices/[practiceId]/clients`, `billing`, `staff` and `offers` are
	 * the same shape -- a heading, an optional intro sentence, one or two
	 * `DataTable`s, and a primary action -- and none of them existed as a
	 * Template before this.
	 *
	 * `OverviewHub` (archetype B) was considered first, since the shapes look
	 * close, and rejected: its `isEmpty`/`empty` pair is required because a
	 * hub's whole body is either the populated view or the empty one. A list
	 * screen does not divide that way -- `clients`' "Find or add a Client"
	 * link and the "Also show Clients with no care or request yet" toggle render whether or not the table has
	 * rows, and `DataTable` already carries its own `emptyMessage` for the
	 * zero-row case. Forcing every route here to split its body into
	 * `primary`/`empty` would recreate a state that already exists one layer
	 * down, so this Template takes a single `content` region instead.
	 *
	 * The empty list is therefore the content's own (#2015): `DataTable`
	 * draws an `EmptyState` in place of its table, and a list that is not a
	 * `DataTable` places one itself. Whatever summarizes the list (totals,
	 * a count line, filters with nothing to narrow) is the route's to leave
	 * out when there is nothing to summarize.
	 *
	 * No cap on `center-l`, matching `OverviewHub`'s own reasoning: a list is
	 * tables, not prose, and past the ramp's plateau more room buys more
	 * content (#531, #541) -- a `DataTable` stops at its own content width
	 * regardless (#542), so the extra room goes to columns that want it, not
	 * to a stretched cell.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import PageReadPending from '#lib/components/molecules/PageReadPending.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import { LOADED_PAGE, type PageReadState, type PageTitling } from '#lib/pageRead.svelte.js';

	interface Properties extends PageTitling {
		intro?: Snippet;
		/**
		 * The one primary action a list screen carries -- "Invite a Staff
		 * member", "Find or add a Client" -- on the title's line, where list
		 * screens elsewhere put it, ahead of any intro.
		 */
		actions?: Snippet;
		content: Snippet;
		/**
		 * What the act that led here did, such as "Pat Lee was removed from
		 * this Practice." (#2041): `takeOutcome`'s message, drawn the way
		 * `RecordDetail` draws its own -- a status `Notice` before the <h1>,
		 * focused on arrival -- so every page a save lands on says it the
		 * same way (`outcome.ts`).
		 */
		outcome?: string;
		/**
		 * The list's own read (#480, `pageRead.svelte.ts`): loading, with
		 * the Skeleton's accessible label; failed, with the Notice's
		 * message; or loaded, which is also what leaving it out means.
		 */
		read?: PageReadState;
	}

	let { title, serviceName, intro, actions, content, outcome, read = LOADED_PAGE }: Properties = $props();
</script>

<PageTitle page={title} {serviceName} isPending={read.status === 'loading'} />

<container-l>
	<center-l max="none" gutters="var(--page-gutter)">
		{#if read.status === 'failed'}
			<!-- data-load-error: see FormPage.svelte's own comment on this
			     attribute (#1258) -- the same fact, the same reason. -->
			<stack-l space="var(--space-section)" data-load-error>
				<PageReadPending {title} {read} skeleton="row" />
			</stack-l>
		{:else if read.status === 'loading'}
			<stack-l space="var(--space-section)">
				<PageReadPending {title} {read} skeleton="row" />
			</stack-l>
		{:else}
			<stack-l space="var(--space-group)">
				{#if outcome}
					<Notice variant="status" message={outcome} isFocusedOnAppear />
				{/if}
				<!-- The list's one action shares the title's line, beside the
				     title rather than at the far edge, since a table stops at
				     its own width (#542) and the edge can sit far from it. It
				     wraps under the title when the line runs out (ADR-0024).
				     The row holds one action's height with or without one, so
				     the title sits at the same place on every list. -->
				<cluster-l class="title-row" space="var(--space-group)" align="center">
					<Heading level={1} variant="page" text={title} />
					{#if actions}
						<div class="actions">{@render actions()}</div>
					{/if}
				</cluster-l>

				{#if intro}
					<!-- A stack, not a plain div (#2020): an intro of two
					     paragraphs ran them together. -->
					<stack-l class="intro" space="var(--space-related)">{@render intro()}</stack-l>
				{/if}

				{@render content()}
			</stack-l>
		{/if}
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		.title-row {
			min-block-size: var(--start-button-block-size);
		}

		/* The one place this Template caps a line length: the body itself is
		   uncapped for tables, but an intro sentence is prose, and prose left
		   uncapped is the exact defect #491 found on the settings screens --
		   here on the same Template rather than on the same route, since the
		   intro is the only region here that is ever prose. A `Text` placed
		   in `content` caps itself (#2020). */
		.intro {
			max-inline-size: var(--measure);
		}
	}
</style>
