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
	 * The title is the one place in the app the brief's `display` type step is
	 * allowed, which is why `Heading` deliberately cannot reach it (#417): a
	 * one-per-app rule that depends on everyone remembering it is not a rule,
	 * so the step lives here and nowhere a route can pass it.
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

	let { title, serviceName, primary, secondary, isEmpty, empty, loading, loadError }: Properties =
		$props();
</script>

<PageTitle page={title} {serviceName} />

<container-l>
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
				{:else}
					<div class="body" class:has-secondary={secondary}>
						<stack-l space="var(--space-6)">{@render primary()}</stack-l>
						{#if secondary}
							<aside><stack-l space="var(--space-6)">{@render secondary()}</stack-l></aside>
						{/if}
					</div>
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

		h1 {
			margin: 0;
			font-family: var(--font-family-base);
			font-size: var(--text-display-size);
			font-weight: var(--text-display-weight);
			line-height: var(--text-display-leading);
			letter-spacing: var(--text-display-tracking);
			color: var(--color-on-surface);
		}

		.body {
			display: grid;
			gap: var(--space-8);
		}

		/*
		 * Container query, not a media query: ADR-0003 makes container
		 * queries the default and the rail depends on how wide the page
		 * frame is, not how wide the window is. The threshold is the
		 * narrowest width at which a 20rem rail still leaves the primary
		 * column above its own comfortable measure -- --measure, since
		 * this column holds cards and tables rather than a cap of its own
		 * (see the no-cap note above).
		 *
		 * This is a "measure criterion" floor, not an overflow one -- the
		 * primary column is `minmax(0, 1fr)`, so it never overflows, but
		 * below this width the 20rem rail leaves it narrower than
		 * --measure, which is not what the wide configuration is for. The
		 * previous 60rem (960px) was part of the shared 60rem set (#523)
		 * rather than measured against this column at all.
		 *
		 * Re-checked 2026-09-01 against the canonical environment (#564):
		 * a first sweep on 2026-08-31, on /style-guide/overview-hub's own
		 * demo with the query forced live, read the primary column as
		 * first reaching --measure at 1004px (62.75rem) on that machine.
		 * CI's own Linux/Chromium reaches it 8px SOONER, at 996px -- the
		 * opposite direction from every overflow floor this ticket also
		 * re-measured, because the canonical rasterizer renders ordinary
		 * mixed text wider (what an overflow floor watches) but its `0`
		 * glyph narrower (`--measure` is `65ch`, and 1ch is the width of
		 * `0`), so the cap itself is smaller there and the column crosses
		 * it sooner.
		 *
		 * The literal stays 1004px/62.75rem rather than moving to 996px:
		 * sufficiency now runs in every environment (#564), and 996px
		 * measurably leaves the column short of --measure on the machine
		 * that needs the full 1004px -- a real shortfall for a real
		 * reader there, not only a failing assertion. 1004px is exactly
		 * 8px above where CI's own runner crosses, though, which is this
		 * check's own minimality probe depth -- an unresolved tension
		 * between "sufficient in every environment" and "minimal in the
		 * canonical one" recorded on #564 rather than silently picked one
		 * way, since no single number here satisfies both as things
		 * stand.
		 */
		@container (min-width: 62.75rem) {
			.body.has-secondary {
				grid-template-columns: minmax(0, 1fr) 20rem;
			}
		}
	}
</style>
