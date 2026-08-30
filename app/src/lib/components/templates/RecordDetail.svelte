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
	<center-l max="var(--page-max)" gutters="var(--page-gutter)">
		{#if loadError}
			<Notice variant="error" message={loadError} />
		{:else if loading}
			<div class="body" class:has-contents={isContentsShown}>
				{#if isContentsShown}
					<!-- Reserves the rail's column width; empty rather than
					     filled with placeholder links, because `sections` --
					     and so the links -- do not exist yet, and a second
					     "loading" announcement here would double up on the
					     Skeleton's own. -->
					<div class="contents-rail"></div>
				{/if}
				<Skeleton variant="text" lines={6} label={loading} />
			</div>
		{:else}
		<div class="body" class:has-contents={isContentsShown}>
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

			{#if isContentsShown}
				<!--
					The same list twice, and exactly one of them rendered at any
					width -- `display: none` takes the other out of the
					accessibility tree entirely, so nothing is announced twice.
					The alternative was one list restyled by a container query,
					which cannot work: the two looks are `Link` variants, and an
					atom does not get to know how wide its page frame is.
				-->
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
			{/if}

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
		</div>
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

		.body {
			display: grid;
			gap: var(--space-8);
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

		.contents-rail {
			display: none;
		}

		/* An in-page anchor lands the heading at the top of the viewport,
		   where the shell's top bar will be once #452 builds it. The offset
		   is paid now so the deep link PR-G5 asked for does not arrive
		   already broken. */
		section {
			scroll-margin-block-start: calc(var(--top-bar-height) + var(--space-4));
		}

		/*
		 * Container query, not a media query, per ADR-0003 -- the rail
		 * depends on how wide the page frame is, not the window. The same
		 * 60rem threshold OverviewHub uses, so the two archetypes change
		 * shape together rather than at two widths a person would notice.
		 */
		@container (min-width: 60rem) {
			.body.has-contents {
				grid-template-columns: var(--page-rail) minmax(0, 1fr);
				column-gap: var(--space-12);
			}

			.body.has-contents .record-header {
				grid-column: 2;
				grid-row: 1;
			}

			.body.has-contents .sections {
				grid-column: 2;
				grid-row: 2;
			}

			.body.has-contents .contents-rail {
				display: block;
				grid-column: 1;
				grid-row: 1 / span 2;
				align-self: start;
			}

			.body.has-contents .contents-strip {
				display: none;
			}
		}
	}
</style>
