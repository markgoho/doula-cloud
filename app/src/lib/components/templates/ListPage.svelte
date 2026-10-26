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
	 * link and "See everyone" toggle render whether or not the table has
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
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';

	interface Properties {
		title: string;
		/**
		 * The Staff side leaves this unset and gets the product name; the
		 * Client portal passes its Practice's name (#431, #487).
		 */
		serviceName?: string;
		intro?: Snippet;
		/**
		 * The one primary action a list screen carries -- "Invite a Staff
		 * member", "Find or add a Client" -- positioned between the intro and
		 * the content, which is where every one of the four routes this
		 * Template was built for already put it.
		 */
		actions?: Snippet;
		content: Snippet;
		/**
		Presence is the state, value is the Skeleton's accessible label (#480).
		*/
		loading?: string;
		/**
		Presence is the state, value is the Notice's message (#480).
		*/
		loadError?: string;
	}

	let { title, serviceName, intro, actions, content, loading, loadError }: Properties = $props();
</script>

<PageTitle page={title} {serviceName} />

<container-l>
	<center-l max="none" gutters="var(--page-gutter)">
		{#if loadError}
			<!-- data-load-error: see FormPage.svelte's own comment on this
			     attribute (#1258) -- the same fact, the same reason. -->
			<stack-l space="var(--space-section)" data-load-error>
				<Heading level={1} variant="page" text={title} />
				<Notice variant="error" message={loadError} />
			</stack-l>
		{:else if loading}
			<stack-l space="var(--space-section)">
				<Heading level={1} variant="page" text={title} />
				<Skeleton variant="row" lines={6} label={loading} />
			</stack-l>
		{:else}
			<stack-l space="var(--space-section)">
				<Heading level={1} variant="page" text={title} />

				{#if intro}
					<!-- A stack, not a plain div (#2020): an intro of two
					     paragraphs ran them together. -->
					<stack-l class="intro" space="var(--space-related)">{@render intro()}</stack-l>
				{/if}

				{#if actions}
					<div class="actions">{@render actions()}</div>
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
