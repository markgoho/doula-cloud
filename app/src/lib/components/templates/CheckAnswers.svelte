<script module lang="ts">
	export interface Answer {
		label: string;
		value: string;
		changeHref: string;
		/**
		 * What the Change link changes, in words -- "given name", "email
		 * address". Every row's visible text is the same word, so without
		 * this a screen reader user listing the page's links hears "Change"
		 * nineteen times; GOV.UK's own answer is visually-hidden text after
		 * it, which is what this becomes.
		 */
		changes: string;
	}

	export interface AnswerSection {
		heading: string;
		answers: Answer[];
	}
</script>

<script lang="ts">
	/*
	 * Archetype E, second half -- the summary page that ends a question
	 * sequence. ADR-0018, amended on #464.
	 *
	 * ## Why the rows are not `DescriptionList`
	 *
	 * A check-answers row is a label, a value and an action, and
	 * `DescriptionList` has no action column. Growing it would be growing a
	 * molecule for exactly one consumer, and ADR-0018 names the bar this
	 * repo already uses: one consumer stays a raw exception, two identical
	 * consumers earn the extraction. So the row markup lives here until a
	 * second page wants it. `DescriptionList`'s own defect -- keying its
	 * `#each` on the label -- is fixed on this ticket regardless, because
	 * that is a bug rather than a shape.
	 *
	 * ## Two widths
	 *
	 * GOV.UK's Check answers pattern says a long list of answers may take a
	 * wider column than a form normally would. The intake summary is
	 * nineteen rows on a Practice that has added fields, so the wide case is
	 * real rather than hypothetical -- `isWide` drops the --form-max cap
	 * and lets the column take whatever is left beside the rail.
	 */
	import type { Snippet } from 'svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import StepRail, { type JourneyStep } from '#lib/components/organisms/StepRail.svelte';

	interface Properties {
		journey: string;
		steps: JourneyStep[];
		backHref: string;
		title: string;
		/**
		 * The Staff side leaves this unset and gets the product name; the
		 * Client portal passes its Practice's name (#431, #487).
		 */
		serviceName?: string;
		caption?: string;
		/**
		 * Positioned here, built by the route (#467). Same rule as
		 * `QuestionPage`.
		 */
		errorSummary?: Snippet;
		sections: AnswerSection[];
		/**
		 * GOV.UK's wider column for a long answer list.
		 */
		isWide?: boolean;
		actions: Snippet;
	}

	let {
		journey,
		steps,
		backHref,
		title,
		serviceName,
		caption,
		errorSummary,
		sections,
		isWide = false,
		actions
	}: Properties = $props();

	const pageId = $props.id();
</script>

<PageTitle page={title} {serviceName} isError={Boolean(errorSummary)} />

<container-l>
	<!-- No cap on the frame: it holds the rail beside the column, and the
	     column carries its own cap -- or drops it, when `isWide` (#541). -->
	<center-l max="none" gutters="var(--page-gutter)">
		<!--
			sidebar-l, side="start" (default) (#564): the rail is FIRST in
			the DOM. `content-min` tracks the column's own cap -- --form-max
			normally, --measure when `isWide` drops that cap, matching
			OverviewHub's own uncapped-column pattern -- so the wrap trigger
			is always the same quantity the column itself is judged against,
			never an authored pixel.
		-->
		<sidebar-l
			basis="var(--page-rail)"
			space="var(--space-12)"
			content-min={isWide ? 'min(var(--measure), 100%)' : 'min(var(--form-max), 100%)'}
		>
			<!--
				Every completed step expands, so the rail stops being a
				position marker and becomes the whole journey at a glance --
				which is what this page is for (#432).
			-->
			<StepRail {journey} {steps} expand="completed" />

			<div class="column" class:wide={isWide}>
				<stack-l space="var(--space-6)">
					<BackLink href={backHref} />

					{#if errorSummary}
						{@render errorSummary()}
					{/if}

					<div>
						{#if caption}
							<span class="caption">{caption}</span>
						{/if}
						<Heading level={1} variant="page" text={title} />
					</div>

					<stack-l space="var(--space-8)">
						<!--
							Keyed on index, not on the heading: a Practice names its
							own sections and two may share a name, which is the same
							defect this ticket fixes in `DescriptionList`.
						-->
						{#each sections as section, sectionIndex (sectionIndex)}
							<section>
								<stack-l space="var(--space-4)">
									<Heading level={2} variant="section" text={section.heading} />
									<dl>
										{#each section.answers as answer, answerIndex (answerIndex)}
											<div class="row">
												<dt>{answer.label}</dt>
												<dd class="value">{answer.value}</dd>
												<!--
													Every row's link reads "Change", so on its own the
													list of links is nineteen identical names. GOV.UK
													answers that with visually-hidden text naming what
													changes; here it is a sibling joined by
													aria-describedby rather than a child, because a
													`Link` takes a label and not children -- the
													announced name becomes "Change, given name" either
													way, and the atom stays closed.

													`secondary`, not the plum prose link GOV.UK uses: a
													summary page carries one Change link per row, and the
													brief's Von Restorff rule spends the accent once per
													screen -- which here is `Save this client`. Nineteen
													plum links is the same overspend #432 rejected in the
													step rail's first pass. Neutral, underlined, and plum
													on hover keeps the affordance without the noise.
												-->
												<dd class="action">
													<Link
														href={answer.changeHref}
														label="Change"
														variant="secondary"
														describedBy="{pageId}-{sectionIndex}-{answerIndex}-changes"
													/>
													<span
														class="visually-hidden"
														id="{pageId}-{sectionIndex}-{answerIndex}-changes">{answer.changes}</span
													>
												</dd>
											</div>
										{/each}
									</dl>
								</stack-l>
							</section>
						{/each}
					</stack-l>

					<cluster-l space="var(--space-4)" align="center">{@render actions()}</cluster-l>
				</stack-l>
			</div>
		</sidebar-l>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		/*
		 * sidebar-l wraps at content-min="min(var(--form-max), 100%)" (or
		 * the --measure equivalent when `isWide` -- markup above), so it
		 * never fits beside the rail without the column reaching the
		 * width it is actually judged against (#564). Centred,
		 * top-aligned, for the same reason `QuestionPage`'s own
		 * equivalent Sidebar is: a summary of answers has nothing more to
		 * put in a wider window (#543), and the flex default (stretch)
		 * would otherwise match the rail and the column to whichever is
		 * taller.
		 *
		 * `min(…, 100%)`, not a bare cap, for the same reason
		 * `QuestionPage` needs it: `.column` also carries its own
		 * `max-inline-size` below (--form-max, or none when `isWide`),
		 * and pinning `min-inline-size` to the SAME length as that cap
		 * makes the column unable to shrink once wrapped onto its own
		 * row, overflowing a narrow frame instead of adapting to it.
		 */
		sidebar-l {
			justify-content: center;
			align-items: start;
		}

		.column {
			max-inline-size: var(--form-max);
		}

		.column.wide {
			max-inline-size: none;
		}

		dl {
			margin: 0;
		}

		dt {
			margin: 0;
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
		}

		dd {
			margin: 0;
			color: var(--color-on-surface);
		}


		.caption {
			display: block;
			margin-block-end: var(--space-1);
			font-size: var(--text-body-size);
			color: var(--color-on-surface-muted);
		}

		/*
		 * A hairline under each row and none around the list: the brief's
		 * rule that containers are declared by an edge rather than a box,
		 * and what makes nineteen rows scannable without nineteen borders.
		 *
		 * No @container query (#564): flex-wrap reads the row's own
		 * content instead of an authored width. `grid-template-columns:
		 * repeat(auto-fit, minmax(min-content, 1fr))` was tried first and
		 * read as the same idea, but `CSS.supports()` against a real
		 * Chromium returns false for it -- auto-fit's repetition count has
		 * to be computable from a fixed track size, and min-content is not
		 * one, so the whole declaration is dropped and every row silently
		 * stacked, always, which a class-level test never caught. Flexbox
		 * does not have that restriction: each cell's automatic minimum is
		 * its own min-content -- `white-space` is ordinary, so a cell CAN
		 * still wrap internally, but never below the width its own text
		 * needs to start a line -- and flex-wrap packs as many cells onto
		 * a line as fit before starting the next. That is what makes a
		 * label and a "Change" action -- both short, author-controlled
		 * strings -- the ones that force a stack when they do not fit:
		 * their min-content cannot shrink, where the previous no-wrap
		 * criterion had to measure that same fact against a pixel (416px)
		 * because nothing could yet be asked directly. A value is a
		 * Practice's own data of arbitrary length and wraps freely either
		 * way, which was always true and is unchanged.
		 *
		 * `flex-grow: 1` on every cell gives each fitting column an EQUAL
		 * share of the row once three fit, which does cost the previous
		 * design's one asymmetry -- `.action` no longer stays
		 * content-sized against two equal columns, all three now match --
		 * a visible proportion change, and the trade for removing the
		 * threshold entirely.
		 */
		.row {
			display: flex;
			flex-wrap: wrap;
			column-gap: var(--space-4);
			row-gap: var(--space-1);
			padding-block: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		.row > * {
			flex: 1 1 auto;
		}

		.action {
			text-align: end;
		}
	}
</style>
