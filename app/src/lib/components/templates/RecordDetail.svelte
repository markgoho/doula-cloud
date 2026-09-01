<script lang="ts">
	/*
	 * Archetype D -- the multi-section record detail. ADR-0018.
	 *
	 * `sections` is an array rather than a run of named Snippet props because
	 * the page that forced this Template has a *variable* number of them: the
	 * staff Engagement detail is Visits, then N Plan sections, then Contract,
	 * Invoices, Offers and Messages. Named regions cannot express N. It is
	 * `DataTable.rowActions`'s shape generalised -- the third confirmed use of
	 * Snippets as an API, after `LabeledField` and `DataTable`.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';

	interface Section {
		heading: string;
		content: Snippet;
	}

	interface Properties {
		title: string;
		/**
		 * The Staff side leaves this unset and gets the product name; the
		 * Client portal passes its Practice's name (#431, #487).
		 */
		serviceName?: string;
		summary?: Snippet;
		actions?: Snippet;
		sections: Section[];
		/**
		 * The contents region, ADR-0018's 2026-08-29 amendment. A boolean and
		 * not a Snippet, unlike every other region here: the list *is*
		 * `sections`, so deriving it is what keeps the promise that this is
		 * not a nav. A Snippet would let a route put routes in it.
		 *
		 * Optional because archetype D covers short records too, and a
		 * contents list above three sections is furniture. It is also
		 * consulted while `loading` -- a route knows whether its record has
		 * a rail before the record has loaded, since it never varied by
		 * data (#480) -- to reserve the rail's column width so the layout
		 * does not shift width once `sections` exists to derive links from.
		 */
		isContentsShown?: boolean;
		/**
		Presence is the state, value is the Skeleton's accessible label (#480).
		*/
		loading?: string;
		/**
		Presence is the state, value is the Notice's message (#480).
		*/
		loadError?: string;
	}

	let {
		title,
		serviceName,
		summary,
		actions,
		sections,
		isContentsShown = false,
		loading,
		loadError
	}: Properties = $props();

	// The `#each` below is already keyed on the heading, so two sections
	// sharing one is a pre-existing error rather than a new one this
	// introduces -- the anchor simply inherits that same uniqueness.
	function anchorId(heading: string) {
		return heading
			.toLowerCase()
			.replaceAll(/[^a-z0-9]+/g, '-')
			.replaceAll(/^-|-$/g, '');
	}
</script>

<PageTitle page={title} {serviceName} />

<container-l>
	<!-- No cap: a record is a contents rail beside description lists and
	     tables, not prose, and past the ramp's plateau more room buys more
	     content (#531, #541). -->
	<center-l max="none" gutters="var(--page-gutter)">
		{#snippet recordContent()}
			<!--
				A plain div, not <header>: a <header> outside article/aside/main/
				nav/section maps to the `banner` landmark, and the banner is the
				shell's (#431), not a page's. A Template renders no chrome.
			-->
			<div class="record-header">
				<cluster-l space="var(--space-4)" justify="space-between" align="baseline">
					<Heading level={1} variant="page" text={title} />
					{#if actions}
						<cluster-l space="var(--space-2)" align="center">{@render actions()}</cluster-l>
					{/if}
				</cluster-l>
				{#if summary}
					<div class="summary">{@render summary()}</div>
				{/if}
			</div>

			<div class="sections">
				<stack-l space="var(--space-8)">
					{#each sections as section (section.heading)}
						<!-- v8 ignore start: Svelte-compiled attribute-diffing branch for the
						     templated aria-labelledby/id pair below isn't reachable from
						     app-level interaction tests, only from Svelte's own reactivity
						     internals -->
						<section id={anchorId(section.heading)} aria-labelledby="{anchorId(section.heading)}-heading">
							<stack-l space="var(--space-4)">
								<Heading level={2} variant="section" text={section.heading} id="{anchorId(section.heading)}-heading" />
								{@render section.content()}
							</stack-l>
						</section>
						<!-- v8 ignore stop -->
					{/each}
				</stack-l>
			</div>
		{/snippet}

		{#if loadError}
			<Notice variant="error" message={loadError} />
		{:else if loading}
			{#if isContentsShown}
				<!-- sidebar-l (#564), side="end" (like OverviewHub's own
				     secondary), same as the loaded state below. The rail
				     slot holds an empty placeholder rather than
				     placeholder links, because `sections` -- and so the
				     links -- do not exist yet, and a second "loading"
				     announcement here would double up on the Skeleton's
				     own; it still reserves the column's width so the page
				     does not reflow once the real rail arrives. -->
				<sidebar-l
					side="end"
					space="var(--space-12)"
					basis="var(--page-rail)"
					content-min="min(var(--measure), 100%)"
				>
					<Skeleton variant="text" lines={6} label={loading} />
					<div class="contents-rail"></div>
				</sidebar-l>
			{:else}
				<Skeleton variant="text" lines={6} label={loading} />
			{/if}
		{:else if isContentsShown}
			<!--
				sidebar-l, side="end" (#564): DOM order keeps the record's
				own title before its contents nav -- a screen reader or
				keyboard user meets "Ada Lovelace" before "Jump to" -- so
				`side="end"` is what OverviewHub's own comment already
				explains, not the "no reordering needed" this comment used
				to claim: that claim was wrong, caught by a test asserting
				DOM order (RecordDetail.svelte.spec.ts), and `side="start"`
				visually matched DOM order too, putting the rail on the
				RIGHT where it had always been on the LEFT. `flex-direction:
				row-reverse` (below) is what restores the left placement
				without moving the rail back to the front of the DOM --
				visual order and reading order deliberately differ here,
				each for its own reason. Wraps to a stack on its own once
				the record content cannot keep --measure, the same intent
				the removed @container threshold measured.
				content-min="min(var(--measure), 100%)", not a bare
				var(--measure) -- see OverviewHub's own comment for the
				overflow that plain form measurably caused once wrapped.
			-->
			<sidebar-l
				side="end"
				space="var(--space-12)"
				basis="var(--page-rail)"
				content-min="min(var(--measure), 100%)"
			>
				<stack-l space="var(--space-8)">
					{@render recordContent()}
				</stack-l>

				<!--
					The same list twice, and exactly one of them rendered at a
					time -- `display: none` takes the other out of the
					accessibility tree entirely, so nothing is announced twice.
					The switch is `(pointer: coarse)` now, not a width
					threshold (#564): the chip row exists for "read at arm's
					length in a hospital corridor" (PR-G5) -- a touch
					ergonomics decision, not a room one, since the rail variant
					itself reads fine at any width (`inline-size: 100%`). A
					stated input preference is exactly what ADR-0024 rule 3
					reserves a media query for.
				-->
				<div class="contents">
					<div class="contents-strip">
						<stack-l space="var(--space-2)">
							<p class="contents-heading">Jump to</p>
							<cluster-l space="var(--space-2)">
								{#each sections as section (section.heading)}
									<Link href="#{anchorId(section.heading)}" label={section.heading} variant="chip" />
								{/each}
							</cluster-l>
						</stack-l>
					</div>

					<div class="contents-rail">
						<stack-l space="var(--space-3)">
							<p class="contents-heading">On this page</p>
							<div class="contents-links">
								{#each sections as section (section.heading)}
									<Link href="#{anchorId(section.heading)}" label={section.heading} variant="rail" />
								{/each}
							</div>
						</stack-l>
					</div>
				</div>
			</sidebar-l>
		{:else}
			{@render recordContent()}
		{/if}
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		/*
		 * Law of Proximity: the summary belongs to the title, so it sits
		 * nearer to it than the first section does -- --space-4 here against
		 * the --space-8 the stack puts between sections. That difference is
		 * what groups the header, not a border.
		 */
		.summary {
			margin-block-start: var(--space-4);
		}

		.contents-heading {
			margin: 0;
			color: var(--color-on-surface-muted);
			font-family: var(--font-family-base);
			font-size: var(--text-label-size);
			font-weight: var(--font-weight-medium);
		}

		/* No gap: the entries' left hairlines meet and read as one rule
		   down the list, which is what makes it a list without a border box
		   around it. */
		.contents-links {
			display: grid;
		}

		.contents-strip {
			padding: var(--space-3) var(--space-4);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
			background-color: var(--color-surface-container);
		}

		/* An in-page anchor lands the heading at the top of the viewport,
		   where the shell's top bar will be once #452 builds it. The offset
		   is paid now so the deep link PR-G5 asked for does not arrive
		   already broken. */
		section {
			scroll-margin-block-start: calc(var(--top-bar-height) + var(--space-4));
		}

		/*
		 * No @container query, and no .body wrapper (#564): the rail split
		 * is `sidebar-l` (markup above), which wraps to a stack on its own
		 * once `.sections` cannot keep --measure -- the same tension
		 * OverviewHub's own equivalent floor hit (a measured pixel drifting
		 * by platform) side-stepped the same way, by reading --measure
		 * itself instead of a number written down for it.
		 *
		 * `flex-direction: row-reverse`, paired with `side="end"` in the
		 * markup: DOM order is content then rail (reading order), but the
		 * rail has always been the LEFT-hand column visually, and
		 * `side="end"` alone only changes which child gets which flex
		 * role, not where either one sits on the line -- reversing the
		 * line is what puts the DOM-last rail back on the left without
		 * moving it in the DOM.
		 */
		sidebar-l {
			flex-direction: row-reverse;
		}

		/*
		 * The contents block asks its own question, so it needs its own
		 * containment context -- named, so that losing this declaration
		 * cannot silently resolve the query below against the page
		 * (#533). Nothing above it can answer for it: `sidebar-l` decides
		 * whether this sits beside the record or above it, and that
		 * decision is the browser's, with no selector that reports it.
		 */
		.contents {
			container: record-contents / inline-size;
		}

		/* The base size re-resolved against the context (#544): a `cqi`
		   resolves against the nearest ANCESTOR container, so `.contents`
		   cannot answer its own. */
		.contents > * {
			font-size: var(--text-body-size);
		}

		/*
		 * Structural, and therefore a query: `.contents-strip` is a row of
		 * chips and `.contents-rail` is a vertical list of links -- two
		 * DOM trees with one rendered at a time, not one tree rearranged,
		 * and no intrinsic mechanism swaps markup. `display: none` keeps
		 * the other out of the accessibility tree so nothing is announced
		 * twice.
		 *
		 * The vertical list is the narrow state and the chip row the wide
		 * one, which is the inverse of the usual reading: a vertical list
		 * of full-width links is correct at any width, so it is the chip
		 * row that has to earn its place, and it earns it by fitting on
		 * one line. That is the whole content question here, and it is
		 * answered entirely from this block's own width -- it never has to
		 * distinguish "beside the record" from "above it", which is what
		 * makes it derivable at all. Measured 2026-09-01 on
		 * /style-guide/record-detail's own fixture: the four Jump to chips
		 * wrap onto a second line below 388px and sit on one line from
		 * 388px up. 24.25rem is that width. A fifth chip, or a longer
		 * section name, moves it.
		 */
		.contents-rail {
			display: block;
		}

		.contents-strip {
			display: none;
		}

		@container record-contents (min-width: 24.25rem) {
			.contents-rail {
				display: none;
			}

			.contents-strip {
				display: block;
			}
		}
	}
</style>
