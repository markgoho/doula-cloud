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
	 * The title is set at the brief's `display` type step where its column
	 * holds a reading measure, and at `heading-lg` below that (#1632). The
	 * display step is why `Heading` deliberately cannot reach it (#417): a
	 * rule that depends on everyone remembering it is not a rule, so the
	 * step lives in Templates and nowhere a route can pass it. This title
	 * and `LandingPage`'s greeting (#1645) are the two places that spend it.
	 */
	import type { Snippet } from 'svelte';
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
		 * Presence, not a paired boolean: the value is also the Skeleton's
		 * accessible label, so a caller cannot supply "loading" without
		 * saying what is loading. The title is drawn while loading (#2248);
		 * bars hold the rest of the hub's space.
		 */
		loading?: string;
		/**
		Same shape as `loading`: presence is the state, value is the message.
		*/
		loadError?: string;
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
		loading,
		loadError,
		outcome
	}: Properties = $props();
</script>

<PageTitle page={title} {serviceName} isPending={Boolean(loading) && !loadError} />

<container-l name="overview-hub">
	<!-- No cap: a hub is cards and tables rather than prose, and
	     past the ramp's plateau more room buys more content (#531, #541).
	     Anything inside that needs a measure asks for one itself. -->
	<center-l max="none" gutters="var(--page-gutter)">
		{#if loadError}
			<Notice variant="error" message={loadError} />
		{:else if loading}
			<stack-l space="var(--space-section)">
				<h1>{title}</h1>
				<Skeleton variant="text" lines={6} label={loading} />
			</stack-l>
		{:else}
			<stack-l space="var(--space-section)">
				{#if outcome}
					<Notice variant="status" message={outcome} isFocusedOnAppear />
				{/if}
				<h1>{title}</h1>
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

		/*
		 * The title is the heading-lg step until the column holds a reading
		 * measure, and the display step from there up (#1632). A Practice
		 * name runs to 75 characters, and at the display step on a phone it
		 * wraps to seven lines and pushes the empty Practice's one link
		 * below the first screen (ADR-0048 asks that screen for one act).
		 * The same words, the same tree: only the type step changes, so
		 * this query picks a step, not a DOM tree -- the second kind the
		 * floor check admits (`floor.svelte.spec.ts`).
		 */
		h1 {
			margin: 0;
			font-family: var(--font-family-base);
			font-size: var(--text-heading-lg-size);
			font-weight: var(--text-heading-lg-weight);
			line-height: var(--text-heading-lg-leading);
			letter-spacing: var(--text-heading-lg-tracking);
			color: var(--color-on-surface);
		}

		/*
		 * The floor is the container width at which the column inside
		 * `center-l`'s gutters first reaches `--measure`, measured on
		 * /style-guide/overview-hub (the measure criterion in
		 * `floor.svelte.spec.ts`), measured 2026-10-03. It is the larger of
		 * two environments' fixed points, because a floor has to be
		 * sufficient in every one: on macOS the column reaches `--measure`
		 * at 596px (564px against 564.6px), and on CI's Linux runner
		 * `--measure` is 585px there, so the column reaches it only at
		 * about 620px. A wider gutter or a wider `--measure` moves it.
		 *
		 * Re-measured 2026-10-25 (#1943): `--measure` is 65ch, so the 16px
		 * body floor (ADR-0060) widened it. On macOS at 620px the column
		 * reached 588px against a 604.2px measure; 40rem (640px) holds on
		 * macOS and in CI's Linux/Chromium.
		 */
		@container overview-hub (min-width: 40rem) {
			h1 {
				font-size: var(--text-display-size);
				font-weight: var(--text-display-weight);
				line-height: var(--text-display-leading);
				letter-spacing: var(--text-display-tracking);
			}
		}
	}
</style>
