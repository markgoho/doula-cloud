<script lang="ts">
	/*
	 * Archetype D -- the multi-section record detail. ADR-0018.
	 *
	 * `sections` is an array rather than a run of named Snippet props because
	 * the page that forced this Template has a *variable* number of them: the
	 * staff Engagement detail is Contract, Visits, then N Plan sections, then
	 * Invoices, Offers and Messages. Named regions cannot express N. It is
	 * `DataTable.rowActions`'s shape generalized -- the third confirmed use of
	 * Snippets as an API, after `LabeledField` and `DataTable`.
	 *
	 * A section with nothing in it yet keeps its heading and places an
	 * `EmptyState` in its `content` (#2015), so the contents rail still
	 * names it and the reader learns it is empty rather than missing.
	 */
	import { tick, type Snippet } from 'svelte';
	import { LOADED_PAGE, type PageReadState } from '#lib/pageRead.svelte.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';

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
		/**
		 * The route's own move for the Back link, when a plain navigation
		 * would drop something the page it returns to keeps in history state.
		 * It owns `preventDefault()`; with no script the href still works.
		 */
		onBack?: (event: MouseEvent) => void;
		/**
		 * The record will have a Back link whose target is its own data,
		 * such as the Practice Your account came from (#2251). While `read` is
		 * loading, its line is held so the title does not move when it
		 * lands.
		 */
		isBackPending?: boolean;
		summary?: Snippet;
		/**
		 * The shape of a `SummaryList` summary, known before its data
		 * (#2251), as `isContentsShown` is: one entry per row, with its
		 * label, a value as long as a usual one (with a note under it as
		 * a second line, such as "Last confirmed"), and the words of its
		 * row action if it has one.
		 * While `read` is loading, a bordered card of these rows holds the
		 * summary's space, each value drawn as bars that wrap where its
		 * words would. A summary of another shape, or none, passes nothing
		 * and gets no placeholder.
		 */
		summaryRows?: readonly { label: string; value: string | readonly string[]; action?: string }[];
		/**
		 * The words of the links a summary has under its card, such as the
		 * Engagement's "View Client" (#2258), held as a line while loading.
		 */
		summaryLinks?: readonly string[];
		actions?: Snippet;
		/**
		 * The actions to draw beside the title while `read` is loading, for a
		 * route whose actions do not wait on its record (#2251), so the
		 * title row does not grow when they land. Usually `actions` itself.
		 */
		loadingActions?: Snippet;
		sections: Section[];
		/**
		 * The contents region, ADR-0018's 2026-08-29 amendment. A boolean and
		 * not a Snippet, unlike every other region here: the list *is*
		 * `sections`, so deriving it is what keeps the promise that this is
		 * not a nav. A Snippet would let a route put routes in it.
		 *
		 * Optional because archetype D covers short records too, and a
		 * contents list above three sections is furniture. It is also
		 * consulted while `read` is loading -- a route knows whether its record has
		 * contents before the record has loaded, since it never varied by
		 * data (#480) -- so the list is drawn from the named sections at
		 * once and the page does not move when the record lands.
		 */
		isContentsShown?: boolean;
		/**
		 * The record's own read (#480, `pageRead.svelte.ts`): loading, with
		 * the Skeleton's accessible label; failed, with the Notice's
		 * message; or loaded, which is also what leaving it out means.
		 */
		read?: PageReadState;
	}

	let {
		title,
		serviceName,
		outcome,
		backHref,
		backLabel,
		onBack,
		isBackPending = false,
		summary,
		summaryRows,
		summaryLinks,
		actions,
		loadingActions,
		sections,
		isContentsShown = false,
		read = LOADED_PAGE
	}: Properties = $props();

	// The snippets below draw only while loading, but a snippet does not
	// carry the `{#if}` that renders it, so the label is read out here.
	const loadingLabel = $derived(read.status === 'loading' ? read.label : undefined);

	// The `#each` below is already keyed on the heading, so two sections
	// sharing one is a pre-existing error rather than a new one this
	// introduces -- the anchor simply inherits that same uniqueness.
	function anchorId(heading: string) {
		return heading
			.toLowerCase()
			.replaceAll(/[^a-z0-9]+/g, '-')
			.replaceAll(/^-|-$/g, '');
	}

	// #2508: a link from another screen to one section (the Overview's
	// declined Offers, to `#offers`) arrives before the record has loaded,
	// so the browser's own jump lands on the loading bars and the section
	// then moves down. Once the record is drawn, the section is scrolled
	// to again, once.
	let hasLanded = false;
	$effect(() => {
		if (hasLanded || read.status === 'loading') return;
		hasLanded = true;
		const id = location.hash.slice(1);
		if (!id) return;
		void tick().then(() => document.querySelector(`#${CSS.escape(id)}`)?.scrollIntoView());
	});
</script>

<PageTitle page={title} {serviceName} isPending={read.status === 'loading'} />

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
						<div class="back"><BackLink href={backHref} label={backLabel} onClick={onBack} /></div>
					{/if}
					{#if outcome}
						<div class="notice"><Notice variant="status" message={outcome} isFocusedOnAppear /></div>
					{/if}
					<cluster-l space="var(--space-group)" align="baseline">
						<Heading level={1} variant="page" text={title} />
						{#if actions}
							<cluster-l space="var(--space-2)" align="center">{@render actions()}</cluster-l>
						{/if}
					</cluster-l>
					{#if isContentsShown}{@render contentsStrip()}{/if}
					{#if summary}
						<div class="summary">{@render summary()}</div>
					{/if}
				</div>
		{/snippet}

		<!-- The contents sit under the title and above the summary at every
		     width, GOV.UK's contents list (ADR-0021): on a phone's first
		     screen (#2257), and on a wide one at the gutter every Staff
		     page starts at, with no second column to read as a nav
		     (ADR-0018, 2026-11-07). -->
		{#snippet contentsStrip()}
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
		{/snippet}


		<!-- What the header is before the record loads (#2248): the back
		     link and the title, which a route knows first, and a
		     heading-shaped bar where the title is the record's own data
		     (the route passes it empty until then). -->
		{#snippet loadingHeader()}
			<div class="record-header">
				{#if backHref}
					<div class="back"><BackLink href={backHref} label={backLabel} onClick={onBack} /></div>
				{:else if isBackPending}
					<div class="back back-placeholder" aria-hidden="true"></div>
				{/if}
				{#if loadingActions}
					<cluster-l space="var(--space-group)" align="baseline">
						{#if title}
							<Heading level={1} variant="page" text={title} />
						{:else}
							<!-- A name's words as a bar, so the actions keep the
							     baseline and the wrap a loaded name gives them. -->
							<span class="title-placeholder" aria-hidden="true"><span data-words="Given Family"></span></span>
						{/if}
						<cluster-l space="var(--space-2)" align="center">{@render loadingActions()}</cluster-l>
					</cluster-l>
				{:else if title}
					<Heading level={1} variant="page" text={title} />
				{:else}
					<Skeleton variant="heading" lines={1} />
				{/if}
				{#if isContentsShown}{@render contentsStrip()}{/if}
				{#if summaryRows?.length}
					<div class="summary" aria-hidden="true">
						<SummaryList
							rows={summaryRows.map((row) => ({
								label: row.label,
								value: '',
								changes: row.action,
								lines: typeof row.value === 'string' ? [row.value] : row.value
							}))}
							isBordered
						>
							{#snippet value(row)}
								{#each row.lines as line, index (index)}
									<span class="value-placeholder"><span data-words={line}></span></span>
								{/each}
							{/snippet}
							{#snippet action(row)}<span class="action-placeholder">{row.changes}</span>{/snippet}
						</SummaryList>
						{#if summaryLinks?.length}
							<div class="summary-links">
								{#each summaryLinks as link (link)}<span class="action-placeholder">{link}</span>{/each}
							</div>
						{/if}
					</div>
				{/if}
			</div>
		{/snippet}

		<!-- The sections a route already names while its record loads, each
		     heading with bars under it (#2248); one load, so only the first
		     announces. A route that names none yet gets one block of bars. -->
		{#snippet loadingSections()}
			<div class="sections">
				<stack-l space="var(--space-section)">
					{#each sections as section, index (section.heading)}
						<section id={anchorId(section.heading)}>
							<stack-l space="var(--space-4)">
								<Heading level={2} variant="section" text={section.heading} />
								<div class="loading-bars">
									<Skeleton variant="text" lines={3} label={index === 0 ? loadingLabel : undefined} />
								</div>
							</stack-l>
						</section>
					{:else}
						<div class="loading-bars"><Skeleton variant="text" lines={6} label={loadingLabel} /></div>
					{/each}
				</stack-l>
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

		{#if read.status === 'failed'}
			<Notice variant="error" message={read.message} />
		{:else if read.status === 'loading'}
			<stack-l space="var(--space-section)">
				{@render loadingHeader()}
				{@render loadingSections()}
			</stack-l>
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

		.title-placeholder {
			font-size: var(--text-heading-lg-size);
			font-weight: var(--text-heading-lg-weight);
			line-height: var(--text-heading-lg-leading);
			letter-spacing: var(--text-heading-lg-tracking);
		}

		.summary-links {
			display: flex;
			flex-wrap: wrap;
			gap: var(--space-4);
			margin-block-start: var(--space-4);
		}

		.back-placeholder {
			min-block-size: var(--hit-target-min);
		}

		/*
		 * A value's words as bars: transparent text on the Skeleton's own
		 * color, broken per line, so a placeholder wraps where its words
		 * would and holds their height at every width. Held back by
		 * `--loading-delay`, as `Skeleton` is.
		 */
		.value-placeholder {
			display: block;
		}

		.value-placeholder + .value-placeholder {
			font-size: var(--text-meta-size);
			line-height: var(--text-meta-leading);
		}

		/* The words are generated content, so no text query or page
		   search meets a placeholder's sample value. */
		.value-placeholder > span::before,
		.title-placeholder > span::before {
			content: attr(data-words);
		}

		.value-placeholder > span,
		.title-placeholder > span {
			color: transparent;
			background-color: var(--color-surface-container-high);
			border-radius: var(--radius);
			box-decoration-break: clone;
			/* motion:ignore -- Skeleton's held-back reveal: nothing moves. */
			animation: value-placeholder-appears 0s var(--loading-delay) backwards;
		}

		/* motion:ignore -- the reveal above; it changes opacity once. */
		@keyframes value-placeholder-appears {
			from {
				opacity: 0;
			}
		}

		/* Holds a row's action: its words, and its 44px target. */
		.action-placeholder {
			display: inline-flex;
			align-items: center;
			min-block-size: var(--hit-target-min);
			visibility: hidden;
		}

		/* A section is a summary list or prose, capped at a reading measure
		   once loaded, so its bars stop there too. */
		.loading-bars {
			max-inline-size: var(--measure);
		}
		.contents-heading {
			margin: 0;
			color: var(--color-on-surface-muted);
			font-family: var(--font-family-base);
			font-size: var(--text-label-size);
			font-weight: var(--font-weight-medium);
		}

		/* Proximity again: the strip is the title's, as the summary is. */
		.contents-strip {
			/* As wide as its links, so on a wide record the box ends with
			   them rather than at the window's edge. */
			inline-size: fit-content;
			margin-block-start: var(--space-4);
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
	}
</style>
