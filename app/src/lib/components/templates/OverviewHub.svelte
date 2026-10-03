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
		secondary?: Snippet;
		/**
		 * #486's Recent-activity feed: low-prominence and low on the page,
		 * per the design brief's own #433 amendment ("it sits low on every
		 * page it appears on"). Rendered below the primary/secondary split
		 * at the container's own width, not squeezed into either column --
		 * a table reads better full-width than in the narrow sidebar, and
		 * the brief is explicit this is not what a person comes here for.
		 * Absent while `isEmpty`, the same as `secondary`: a Practice with
		 * no Client yet has no activity to show either.
		 */
		feed?: Snippet;
		isEmpty: boolean;
		empty: Snippet;
		/**
		 * Presence, not a paired boolean: the value is also the Skeleton's
		 * accessible label, so a caller cannot supply "loading" without
		 * saying what is loading. `title` is not known yet in every caller
		 * (the Practice landing page's welcome line is built from the data
		 * this state is standing in for), so nothing here reads it.
		 */
		loading?: string;
		/**
		Same shape as `loading`: presence is the state, value is the message.
		*/
		loadError?: string;
	}

	let { title, serviceName, primary, secondary, feed, isEmpty, empty, loading, loadError }: Properties =
		$props();
</script>

<PageTitle page={title} {serviceName} />

<container-l name="overview-hub">
	<!-- No cap: a hub is cards, tables and a rail rather than prose, and
	     past the ramp's plateau more room buys more content (#531, #541).
	     Anything inside that needs a measure asks for one itself. -->
	<center-l max="none" gutters="var(--page-gutter)">
		{#if loadError}
			<Notice variant="error" message={loadError} />
		{:else if loading}
			<Skeleton variant="text" lines={6} label={loading} />
		{:else}
			<stack-l space="var(--space-8)">
				<h1>{title}</h1>

				{#if isEmpty}
					<div class="empty">{@render empty()}</div>
				{:else if secondary}
					<!--
						sidebar-l, side="end" (#564): primary is the dominant,
						content-min-protected side; secondary is the fixed-basis
						sidebar, kept SECOND in the DOM (side="end" swaps which
						child gets which role instead of reordering). Wraps to a
						stack on its own once the primary column cannot keep
						--measure -- the same intent the removed @container
						threshold measured, now read off the primitive's own
						content quantities instead of an authored pixel.

						content-min="min(var(--measure), 100%)", not a bare
						var(--measure): --measure alone stays this template's
						min-inline-size even once wrapped onto its own row,
						which measurably overflowed a frame narrower than
						--measure (a real, checked failure, not a theoretical
						one -- primary read 521px inside a 320px frame before
						this fix). "100%" caps the floor at whatever the row
						actually has once alone.
					-->
					<sidebar-l side="end" space="var(--space-8)" content-min="min(var(--measure), 100%)">
						<stack-l space="var(--space-6)">{@render primary()}</stack-l>
						<aside><stack-l space="var(--space-6)">{@render secondary()}</stack-l></aside>
					</sidebar-l>
				{:else}
					<stack-l space="var(--space-6)">{@render primary()}</stack-l>
				{/if}

				{#if !isEmpty && feed}
					{@render feed()}
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
		 * `floor.svelte.spec.ts`): measured 2026-10-03 at 596px, where the
		 * column is 564px and `--measure` 564.6px. A wider gutter or a
		 * wider `--measure` moves it.
		 */
		@container overview-hub (min-width: 37.25rem) {
			h1 {
				font-size: var(--text-display-size);
				font-weight: var(--text-display-weight);
				line-height: var(--text-display-leading);
				letter-spacing: var(--text-display-tracking);
			}
		}

		/*
		 * No @container query for the rail split, and no .body wrapper
		 * (#564). The one query above picks a type step, never the
		 * arrangement. The rail split is `sidebar-l`, Every Layout's own
		 * Sidebar, wired in the markup above. #564's own map found the exact tension a measured
		 * threshold cannot resolve -- sufficiency (never less room than
		 * promised) and minimality (the smallest space that still works)
		 * disagreeing by exactly the check's own probe depth on this
		 * template's old floor, because the same font rasterizes to a
		 * different `--measure` on a different platform. `sidebar-l`
		 * side-steps the whole question: it reads `--measure` itself, on
		 * whichever machine is rendering, and wraps the moment that
		 * machine's own column can no longer honor it. No number is
		 * authored for the split for any environment to disagree about.
		 * The title's floor above does author one, and lives with that
		 * spread: the floor check's minimality probe is 5% of the floor,
		 * wider than any spread measured, and a title one step smaller
		 * near the floor costs no content.
		 */
	}
</style>
