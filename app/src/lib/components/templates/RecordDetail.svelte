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

	/**
	 * Which of the `details` region's two copies a snippet is drawing: the
	 * aside beside the sections in two columns, or the last block in one.
	 */
	type DetailsView = 'aside' | 'section';

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
		/**
		 * The at-a-glance region (#2755): a few facts the reader wants on
		 * arriving, drawn beside the sections where the record's container
		 * is wide enough, and under "Jump to" where it is not. Present, it
		 * puts "Jump to", the facts and the sections in a `record-body`
		 * grid and moves the summary after the sections; absent, the record
		 * is laid out as it always was. A Snippet and not rows, because each
		 * fact's value is a link the route owns.
		 */
		facts?: Snippet;
		/**
		 * The `details` region (#2755): what a record is made of and how it is
		 * reached (contact, created, the acts that change it), drawn under the
		 * facts where the record is two columns, and as the last block where it
		 * is one. It is drawn twice, once for each, and exactly one is displayed
		 * (the other is `display: none`, out of the accessibility tree), as
		 * DataTable's two trees are; the snippet is told which and whether that
		 * copy is the displayed one, and puts a form, a dialog or anything with
		 * an id into the displayed copy only. Needs `facts`. The "Details" link
		 * in "Jump to" goes to the displayed copy.
		 */
		details?: Snippet<[view: DetailsView, isShown: boolean]>;
		/**
		 * `details` is coming but not here: the loading frame names it in
		 * "Jump to", so the list does not change when the record lands.
		 */
		hasDetails?: boolean;
		/**
		 * The facts' shape before their data, as `summaryRows` is for the
		 * summary: one entry per row, with a value as long as a usual one.
		 * While `read` is loading, bars hold the facts column's space.
		 */
		factsRows?: readonly { label: string; value: string }[];
		/**
		 * One line under the title, before "Jump to": the header's own
		 * facts (#2755). It holds no control.
		 */
		statusLine?: Snippet;
		/**
		 * The status line as one line of bars, while `read` is loading, so
		 * nothing below it moves when the line lands.
		 */
		isStatusLinePending?: boolean;
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
		facts,
		details,
		hasDetails = false,
		factsRows,
		statusLine,
		isStatusLinePending = false,
		actions,
		loadingActions,
		sections,
		isContentsShown = false,
		read = LOADED_PAGE
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

	// #2755: "Back to top" after each section taller than the window, and at
	// the page's end, where the record has a "Jump to" to go back to. A
	// section's own height is measured without the link it may hold.
	let tallSections = $state<Record<string, boolean>>({});

	function watchSectionHeight(key: string) {
		return (section: HTMLElement) => {
			// The state is written a frame later: the link it adds changes the
			// section's height, which would otherwise notify this observer in
			// the same frame it is delivering to.
			let frame = 0;
			const observer = new ResizeObserver(() => {
				cancelAnimationFrame(frame);
				frame = requestAnimationFrame(() => {
					const link = section.querySelector('.back-to-top');
					const linkHeight = link ? link.getBoundingClientRect().height : 0;
					tallSections[key] = section.getBoundingClientRect().height - linkHeight > innerHeight;
				});
			});
			observer.observe(section);
			return () => {
				cancelAnimationFrame(frame);
				observer.disconnect();
			};
		};
	}

	function focusJumpTo(event: MouseEvent) {
		event.preventDefault();
		document.querySelector<HTMLElement>('#jump-to')?.focus();
	}

	// With a facts region the page is a `record-body` grid, in the loading
	// frame (named by `factsRows`) as well as the loaded record.
	const isFactsShown = $derived(Boolean(facts) || Boolean(factsRows?.length));
	const isDetailsNamed = $derived(isFactsShown && (Boolean(details) || hasDetails));

	// Which copy of `details` the container query is drawing, read from the
	// aside's own `display` whenever its size changes, which it does when the
	// query swaps copies. The shown copy owns the `details` id, so a link or a
	// deep link to `#details` lands on what is drawn.
	let shownDetails = $state<DetailsView>('section');

	function trackShownDetails(aside: HTMLElement) {
		// Written a frame later, as the section heights are: the ids it moves are
		// not layout, but a write from inside the observer's own delivery is how
		// a loop notification is raised.
		let frame = 0;
		const observer = new ResizeObserver(() => {
			cancelAnimationFrame(frame);
			frame = requestAnimationFrame(() => {
				shownDetails = getComputedStyle(aside).display === 'none' ? 'section' : 'aside';
			});
		});
		observer.observe(aside);
		return () => {
			cancelAnimationFrame(frame);
			observer.disconnect();
		};
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
					{#if statusLine}
						<div class="status-line">{@render statusLine()}</div>
					{/if}
					{#if isContentsShown && !isFactsShown}{@render contentsStrip()}{/if}
					{#if summary && !isFactsShown}
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
				<p class="contents-heading" id="jump-to-label">Jump to</p>
				<ul id="jump-to" class="jump-list" tabindex="-1" aria-labelledby="jump-to-label"> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
					{#each sections as section (section.heading)}
						<li><Link href="#{anchorId(section.heading)}" label={section.heading} /></li>
					{/each}
					{#if isDetailsNamed}<li><Link href="#details" label="Details" /></li>{/if}
				</ul>
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
				{#if isStatusLinePending}
					<div class="status-line" aria-hidden="true">
						<span class="value-placeholder"><span data-words="Care phase Active    Due Dec 5, 2026, 37 weeks    Doulas Priya Raman, Bethany Kroll"></span></span>
					</div>
				{/if}
				{#if isContentsShown && !isFactsShown}{@render contentsStrip()}{/if}
				{#if summaryRows?.length && !isFactsShown}
					{@render summaryPlaceholder()}
				{/if}
			</div>
		{/snippet}

		{#snippet summaryPlaceholder()}
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
		{/snippet}

		<!-- The facts column before its data: bars where each fact's words
		     will be, so the column holds its height (#2755). -->
		{#snippet factsPlaceholder()}
			{#if factsRows?.length}
				<div class="facts" aria-hidden="true">
					<dl>
						{#each factsRows as row (row.label)}
							<div>
								<dt>{row.label}</dt>
								<dd><span class="value-placeholder"><span data-words={row.value}></span></span></dd>
							</div>
						{/each}
					</dl>
				</div>
			{/if}
		{/snippet}

		<!-- The record's body when it has a facts region (#2755): "Jump to",
		     then the facts, then the sections, in that order for every
		     reader, and in two columns only where the container is wide
		     enough for both (see the style). -->
		{#snippet recordBody(state: PageReadState)}
			<div class="record-body">
				<div class="record-grid">
					{#if isContentsShown}{@render contentsStrip()}{/if}
					{#if state.status === 'loading'}
						{@render factsPlaceholder()}
					{:else if facts}
						<div class="facts">{@render facts()}</div>
					{/if}
					{#if state.status !== 'loading' && details}
						<!-- The wide copy of Details, right after the facts in the DOM so the
						     Tab order is the drawn order. -->
						<aside class="details details-aside" id={shownDetails === 'aside' ? 'details' : undefined} aria-labelledby="details-aside-heading" {@attach trackShownDetails}> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
							<stack-l space="var(--space-3)">
								<!-- The region's label, not a heading: the sections' h2s are the page's outline, in drawn order. -->
								<p class="aside-title" id="details-aside-heading">Details</p>
								{@render details('aside', shownDetails === 'aside')}
							</stack-l>
						</aside>
					{/if}
					{#if state.status === 'loading'}
						{@render loadingSections(state.label)}
					{:else}
						{@render recordSections()}
					{/if}
				</div>
			</div>
		{/snippet}

		<!-- The sections a route already names while its record loads, each
		     heading with bars under it (#2248); one load, so only the first
		     announces. A route that names none yet gets one block of bars. -->
		{#snippet loadingSections(label: string)}
			<div class="sections">
				<stack-l space="var(--space-section)">
					{#each sections as section, index (section.heading)}
						<section id={anchorId(section.heading)}>
							<stack-l space="var(--space-4)">
								<Heading level={2} variant="section" text={section.heading} />
								<div class="loading-bars">
									<Skeleton variant="text" lines={3} label={index === 0 ? label : undefined} />
								</div>
							</stack-l>
						</section>
					{:else}
						<div class="loading-bars"><Skeleton variant="text" lines={6} {label} /></div>
					{/each}
					{#if isFactsShown}{@render summaryPlaceholder()}{/if}
				</stack-l>
			</div>
		{/snippet}

		{#snippet backToTop()}
			<div class="back-to-top"><Link href="#jump-to" label="Back to top" onClick={focusJumpTo} /></div>
		{/snippet}

		{#snippet recordSections()}
				<div class="sections">
					<stack-l space="var(--space-section)">
						{#each sections as section (section.heading)}
							<!-- v8 ignore start: Svelte-compiled attribute-diffing branch for the
							     templated aria-labelledby/id pair below isn't reachable from -- spelling:ignore: aria-labelledby is an ARIA attribute name
							     app-level interaction tests, only from Svelte's own reactivity
							     internals -->
							<section id={anchorId(section.heading)} aria-labelledby="{anchorId(section.heading)}-heading" {@attach watchSectionHeight(section.heading)}> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
								<stack-l space="var(--space-4)">
									<Heading level={2} variant="section" text={section.heading} id="{anchorId(section.heading)}-heading" />
									{@render section.content()}
									{#if isFactsShown && isContentsShown && tallSections[section.heading]}
										{@render backToTop()}
									{/if}
								</stack-l>
							</section>
							<!-- v8 ignore stop -->
						{/each}
						{#if isFactsShown && details}
							<!-- The narrow copy of Details, the last block of the page. -->
							<section class="details details-section" id={shownDetails === 'section' ? 'details' : undefined} aria-labelledby="details-section-heading"> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
								<stack-l space="var(--space-4)">
									<Heading level={2} variant="section" text="Details" id="details-section-heading" />
									{@render details('section', shownDetails === 'section')}
								</stack-l>
							</section>
						{/if}
						{#if isFactsShown && isContentsShown}{@render backToTop()}{/if}
						{#if summary && isFactsShown}
							<!-- With a facts region the summary is after the sections, so the
							     first screen holds the facts and not a card of rows to change
							     (#2755); the route turns it into a section next. -->
							<div class="summary">{@render summary()}</div>
						{/if}
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
			{#if isFactsShown}
				<stack-l space="var(--space-4)">
					{@render recordHeader()}
					{@render recordBody(read)}
				</stack-l>
			{:else}
				<stack-l space="var(--space-section)">
					{@render recordHeader()}
					{@render recordSections()}
				</stack-l>
			{/if}
		{/snippet}

		{#if read.status === 'failed'}
			<Notice variant="error" message={read.message} />
		{:else if read.status === 'loading'}
			{#if isFactsShown}
				<stack-l space="var(--space-4)">
					{@render loadingHeader()}
					{@render recordBody(read)}
				</stack-l>
			{:else}
				<stack-l space="var(--space-section)">
					{@render loadingHeader()}
					{@render loadingSections(read.label)}
				</stack-l>
			{/if}
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

		/*
		 * "Jump to": the muted label, then one wrapping run of plain links,
		 * no box and no chips (#2755). Proximity: the run is the title's.
		 * Each target is 24px (WCAG 2.2, 2.5.8), not the 44px a link on its
		 * own holds, so ten sections fit two lines at a desk's width and
		 * four on a phone; the reader moves by them in a corridor, one
		 * thumb, and each stands apart by its gap.
		 */
		.contents-strip {
			display: flex;
			align-items: baseline;
			gap: var(--space-related);
			margin-block-start: var(--space-4);
		}

		.jump-list {
			display: flex;
			flex-wrap: wrap;
			flex: 1 1 0;
			min-inline-size: 0;
			column-gap: var(--space-related);
			row-gap: var(--space-1);
			margin: 0;
			padding: 0;
			list-style: none;
		}

		.jump-list :global(a) {
			min-block-size: var(--target-min-aa);
		}

		/* The list is a focus target for "Back to top" (tabindex -1), so
		   it shows the ring when it takes focus. */
		.jump-list:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		.status-line {
			margin-block-start: var(--space-2);
		}

		/*
		 * The record's body when it has a facts region (#2755). The grid is
		 * inside the container because a container cannot be queried by its
		 * own style. Narrow, it is one column: "Jump to", the facts, the
		 * sections. The sections' own gap above them is --space-section.
		 */
		.record-body {
			container: record-body / inline-size;
		}

		.record-grid {
			/* The base size, re-declared inside the container it resolves against. */
			font-size: var(--text-body-size);
			display: grid;
			grid-template-columns: minmax(0, 1fr);
			row-gap: var(--space-4);
		}

		.record-grid > .contents-strip {
			margin-block-start: 0;
		}

		/*
		 * The facts: a muted small label and an ink value (#2755). A consumer
		 * draws a `<dl>` of `<div><dt>` and `<dd>` pairs into the region.
		 */

		.facts :global(dl) {
			display: grid;
			row-gap: var(--space-2);
			margin: 0;
		}

		/*
		 * Label beside value where the row has the room, label above it where
		 * it does not, with no query (ADR-0024): the label holds the longest
		 * label's width ("On call tonight", 9rem at the label size) and the
		 * value asks for 11rem ("Wed Nov 18, 11:00am"), so in the 15 to 17rem
		 * column beside the sections the value wraps under its label and at a
		 * phone's full width it sits beside it, the values starting at one edge.
		 */
		.facts :global(dl > div) {
			display: flex;
			flex-wrap: wrap;
			column-gap: var(--space-3);
			align-items: baseline;
		}

		.facts :global(dl > div > dt) {
			/* tokens:ignore -- the longest label's width, stated above */
			flex: 0 0 9rem;
		}

		.facts :global(dl > div > dd) {
			/* tokens:ignore -- the value's own width, stated above */
			flex: 1 1 11rem;
			min-inline-size: 0;
		}

		.facts :global(dt) {
			margin: 0;
			font-size: var(--text-meta-size);
			line-height: var(--text-meta-leading);
			color: var(--color-on-surface-muted);
		}

		.facts :global(dd) {
			margin: 0;
			min-inline-size: 0;
			overflow-wrap: anywhere;
		}

		/*
		 * The width rule (#2755, ADR-0018's 2026-11-15 amendment): rows and tables
		 * take the main column, running text keeps the reading measure. What a
		 * section holds is capped at the measure -- its lines of prose, Notices,
		 * forms and threads -- except a table (`.frame`, as wide as its columns
		 * need) and a key/value list a route marks `column` (`SummaryList`'s
		 * `width="column"`, or the block that holds one), whose value text
		 * keeps the measure itself.
		 */
		.record-grid > .sections section > stack-l > :global(*:not(.frame, .column)) {
			max-inline-size: var(--measure);
		}

		/*
		 * The related-items treatment (GOV.UK): a rule above the facts and above
		 * Details, no box and no fill. The rule's top is the column's top, level
		 * with "Jump to" (4b).
		 */
		.facts,
		.details-aside {
			padding-block-start: var(--space-3);
			border-block-start: var(--border-active) solid var(--color-outline);
		}

		.aside-title {
			margin: 0;
			font-family: var(--font-family-base);
			color: var(--color-on-surface);
			font-size: var(--text-subheading-size);
			font-weight: var(--text-subheading-weight);
			line-height: var(--text-subheading-leading);
			letter-spacing: var(--text-subheading-tracking);
		}

		.back-to-top {
			margin: 0;
		}

		.back-to-top :global(a) {
			min-block-size: var(--target-min-aa);
		}

		.record-grid > .sections {
			min-inline-size: 0;
			margin-block-start: calc(var(--space-section) - var(--space-4));
		}

		/*
		 * 73rem is the floor of two columns, not a device: 55rem for the main
		 * column (a DataTable draws cards below a 55rem container), the gap
		 * (2.5rem at this width), and 15rem for the facts ("Sent, waiting
		 * for the Client to sign" on two lines). Narrower, the facts stay
		 * under "Jump to" in one column. A container query, never the
		 * viewport (ADR-0024). The main track holds 55rem as its floor, so a
		 * window with a scrollbar still has the table, and stops at 64rem, so
		 * the facts sit beside the work and not across a void.
		 */
		/* One column: Details is the last block, and the aside is not drawn. */
		.details-aside {
			display: none;
		}

		@container record-body (min-width: 73rem) {
			.record-grid {
				/* tokens:ignore -- the two tracks' own widths, from the floors above: a
				   DataTable's 55rem, the widest table's 64rem, the facts' 15rem to 17rem. */
				grid-template-columns: minmax(55rem, 64rem) minmax(15rem, 17rem);
				column-gap: var(--space-8);
				justify-content: start;
			}

			.record-grid > .contents-strip {
				grid-column: 1;
				grid-row: 1;
			}

			/*
			 * The sections span every row from the second down, the last of them
			 * flexible, so they are sized in the flexible step and do not inflate
			 * the rows above: the facts sit level with "Jump to", and Details (row
			 * 3) starts right under the facts.
			 */
			.record-grid {
				grid-template-rows: auto auto auto 1fr;
			}

			.record-grid > .sections {
				grid-column: 1;
				grid-row: 2 / -1;
			}

			.facts {
				grid-column: 2;
				grid-row: 1 / span 2;
				align-self: start;
			}

			.details-aside {
				display: block;
				grid-column: 2;
				grid-row: 3;
				align-self: start;
				min-inline-size: 0;
			}

			.details-section {
				display: none;
			}
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
