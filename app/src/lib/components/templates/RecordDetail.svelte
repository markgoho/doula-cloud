<script lang="ts">
	/*
	 * Archetype D -- the multi-section record detail. ADR-0018.
	 *
	 * `sections` is an array rather than a run of named Snippet props because
	 * the page that forced this Template has a *variable* number of them: the
	 * staff Engagement detail is Visits, then N Plan sections, then Contract,
	 * Invoices, Offers and Messages. Named regions cannot express N. It is
	 * `DataTable.rowActions`'s shape generalized -- the third confirmed use of
	 * Snippets as an API, after `LabeledField` and `DataTable`.
	 *
	 * A section with nothing in it yet keeps its heading and places an
	 * `EmptyState` in its `content` (#2015), so the contents rail still
	 * names it and the reader learns it is empty rather than missing.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
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
		/**
		 * What the save that led here did, such as "Work with Pat
		 * started." (#1611, #1710): `takeOutcome`'s message. Rendered as a
		 * status `Notice` before the <h1>, which is GOV.UK's position for a
		 * notification banner, and focused on arrival, as GOV.UK asks of a
		 * success banner on the page a form leads to. A string and not a
		 * Snippet: every page a save leads to says it the same way.
		 */
		outcome?: string;
		/**
		 * GOV.UK's Back link, above everything else on the record, the
		 * outcome banner included (#1998): the Engagement's drawing has
		 * "← Clients" there. Page arrangement is the Template's, so the
		 * place is here and the words are the route's.
		 */
		backHref?: string;
		backLabel?: string;
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
		outcome,
		backHref,
		backLabel,
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
		{#snippet recordHeader()}
				<!--
					A plain div, not <header>: a <header> outside article/aside/main/
					nav/section maps to the `banner` landmark, and the banner is the
					shell's (#431), not a page's. A Template renders no chrome.
				-->
				<div class="record-header">
					{#if backHref}
						<div class="back"><BackLink href={backHref} label={backLabel} /></div>
					{/if}
					{#if outcome}
						<div class="notice"><Notice variant="status" message={outcome} isFocusedOnAppear /></div>
					{/if}
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
		{/snippet}

		{#snippet recordSections()}
				<div class="sections">
					<stack-l space="var(--space-section)">
						{#each sections as section (section.heading)}
							<!-- v8 ignore start: Svelte-compiled attribute-diffing branch for the
							     templated aria-labelledby/id pair below isn't reachable from -- spelling:ignore: aria-labelledby is an ARIA attribute name
							     app-level interaction tests, only from Svelte's own reactivity
							     internals -->
							<section id={anchorId(section.heading)} aria-labelledby="{anchorId(section.heading)}-heading"> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
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

		{#snippet recordContent()}
			<!--
				The stack is the snippet's own, not each caller's (#1711): a
				record with no contents rail used to render the header and the
				sections as bare siblings, so the summary's last block touched
				the first section heading. Every record now puts the same
				--space-section between its header and its first section as between
				two sections.
			-->
			<stack-l space="var(--space-section)">
				{@render recordHeader()}
				{@render recordSections()}
			</stack-l>
		{/snippet}

		{#if loadError}
			<Notice variant="error" message={loadError} />
		{:else if loading}
			{#if isContentsShown}
				<!-- sidebar-l (#564), side="end" (like OverviewHub's own
				     secondary), as the loaded state's wide arrangement. The rail
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
				#2004: the contents sit between the record's header and its
				sections, in reading order as on the page: a screen reader or
				keyboard user meets the title, then what is on the page, then
				the sections -- GOV.UK's contents list, after the title. Where
				the sections column can hold a reading measure beside it, the
				contents are the drawing's "On this page" rail to the left of
				the header and the sections; below that, the drawing's narrow
				"Jump to" strip under the header, where a Doula reading at
				arm's length in a corridor finds it (PR-G5). Before #2004 the
				narrow page stacked the rail under the last section, the full
				height of the page away from the title.

				One query decides both the arrangement and which list
				renders, so the two cannot disagree: a strip beside the
				sections, or a rail above them, is a state no width reaches.
				The same list twice, and exactly one of them rendered at a
				time -- `display: none` takes the other out of the
				accessibility tree entirely, so nothing is announced twice.
			-->
			<div class="record">
				<div class="record-grid">
					{@render recordHeader()}

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

					{@render recordSections()}
				</div>
			</div>
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
		 * the --space-section the stack puts between sections. That difference is
		 * what groups the header, not a border.
		 */
		.summary {
			margin-block-start: var(--space-4);
		}

		.notice {
			margin-block-end: var(--space-6);
		}

		/* The drawing's --space-4 between the Back link and the title row. */
		.back {
			margin-block-end: var(--space-4);
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
		 * The loading state's reserved rail (#480, #564): `sidebar-l`,
		 * `side="end"` with the line reversed, so the DOM-last placeholder
		 * sits on the left where the loaded rail does.
		 */
		sidebar-l {
			flex-direction: row-reverse;
		}

		/* The placeholder holds its column whatever the width: the query
		   below that hides a rail asks about the loaded record. */
		sidebar-l > .contents-rail {
			display: block;
		}

		/*
		 * The record asks one question of its own width, so it is its own
		 * containment context -- named, so that losing this declaration
		 * cannot silently resolve the query below against the page (#533).
		 */
		.record {
			container: record-detail / inline-size;
		}

		/* The base size re-resolved against the context (#544): a `cqi`
		   resolves against the nearest ANCESTOR container. */
		.record > * {
			font-size: var(--text-body-size);
		}

		/* Narrow: one column -- the header, the Jump to strip, the
		   sections -- with the gap between sections between each. */
		.record-grid {
			display: grid;
			grid-template-columns: minmax(0, 1fr);
			row-gap: var(--space-section);
		}

		.contents-rail {
			display: none;
		}

		/*
		 * #2004: wide, the rail beside the header and the sections, from the
		 * width at which the sections column holds a reading measure beside
		 * it -- the content question `sidebar-l` answered here before, asked
		 * once so the arrangement and the list that renders flip together.
		 * Structural, and therefore a query: the strip and the rail are two
		 * DOM trees, and no intrinsic mechanism swaps markup. The rail
		 * column is `--page-rail` (16.25rem) and the gap `--space-12`
		 * (3rem), so the floor is those plus the measure, checked by the
		 * floor check's measure criterion.
		 */
		@container record-detail (min-width: 59.5rem) {
			.record-grid {
				grid-template-columns: var(--page-rail) minmax(0, 1fr);
				grid-template-areas:
					'contents header'
					'contents sections';
				column-gap: var(--space-12);
			}

			.record-grid > .record-header {
				grid-area: header;
			}

			.record-grid > .sections {
				grid-area: sections;
			}

			.contents {
				grid-area: contents;
				align-self: start;
			}

			.contents-rail {
				display: block;
			}

			.contents-strip {
				display: none;
			}
		}
	}
</style>
