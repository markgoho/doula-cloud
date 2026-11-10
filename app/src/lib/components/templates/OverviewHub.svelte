<script lang="ts">
	/*
	 * Archetype B -- the overview hub. ADR-0018.
	 *
	 * `isEmpty` and `empty` are required, not optional, and that is the whole
	 * reason this layer exists. `docs/journeys/evaluator-doula.md` names the
	 * Practice landing page as Tasha Bell's abandon point -- "an empty filing
	 * cabinet, not proof" -- because nobody had to think about the zero-data
	 * case. A Template that cannot be instantiated without an empty state
	 * makes forgetting it a type error.
	 *
	 * The title is the page `Heading` every Template draws, so a hub's
	 * <h1> is the size of every other screen's. It was the display step
	 * here once (#1632), which made the Overview's title alone larger.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import PageReadPending from '#lib/components/molecules/PageReadPending.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import { LOADED_PAGE, type PageReadState, type PageTitling } from '#lib/pageRead.svelte.js';

	interface Properties extends PageTitling {
		primary: Snippet;
		/**
		 * #486's Recent-activity feed: low-prominence and low on the page,
		 * per the design brief's own #433 amendment ("it sits low on every
		 * page it appears on"). Rendered last in the column (#2020, #2063),
		 * after the work.
		 *
		 * The column is narrower than `DataTable`'s own 55rem floor at
		 * common window widths, so the caller gives the feed's table its
		 * own floor (the Practice landing hub's `activity-table`).
		 *
		 * Absent while `isEmpty`: a Practice with no Client yet has no
		 * activity to show either.
		 */
		feed?: Snippet;
		isEmpty: boolean;
		empty: Snippet;
		/**
		 * The hub's own read (#480, `pageRead.svelte.ts`): one value, not a
		 * pair of flags. Loading carries the Skeleton's accessible label,
		 * so a caller cannot ask for loading without saying what is
		 * loading; failed carries the Notice's message; loaded is also what
		 * leaving it out means. The title is drawn while loading (#2248);
		 * bars hold the rest of the hub's space.
		 */
		read?: PageReadState;
		/**
		 * What the act that led here did (#1710): `takeOutcome`'s message.
		 * A status `Notice` before the <h1>, focused on arrival, and only
		 * once the hub has loaded -- the same as `FormPage`'s and
		 * `RecordDetail`'s own `outcome`. The Practice landing page is its
		 * first reader: creating the Practice at signup (#2182) and agreeing
		 * to a changed Terms of Service (#1549) land there.
		 */
		outcome?: string;
	}

	let {
		title,
		serviceName,
		primary,
		feed,
		isEmpty,
		empty,
		read = LOADED_PAGE,
		outcome
	}: Properties = $props();
</script>

<PageTitle page={title} {serviceName} isPending={read.status === 'loading'} />

<container-l name="overview-hub">
	<!-- No cap: a hub is cards and tables rather than prose, and
	     past the ramp's plateau more room buys more content (#531, #541).
	     Anything inside that needs a measure asks for one itself. -->
	<center-l max="none" gutters="var(--page-gutter)">
		{#if read.status === 'failed'}
			<Notice variant="error" message={read.message} />
		{:else if read.status === 'loading'}
			<stack-l space="var(--space-section)">
				<PageReadPending {title} {read} skeleton="text" />
			</stack-l>
		{:else}
			<stack-l space="var(--space-section)">
				{#if outcome}
					<Notice variant="status" message={outcome} isFocusedOnAppear />
				{/if}
				<Heading level={1} variant="page" text={title} />
				{#if isEmpty}
					<div class="empty">{@render empty()}</div>
				{:else}
					<!-- The work, then the feed, last (#2063). -->
					<stack-l space="var(--space-group)">
						{@render primary()}
						{#if feed}
							{@render feed()}
						{/if}
					</stack-l>
				{/if}
			</stack-l>
		{/if}
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}
	}
</style>
