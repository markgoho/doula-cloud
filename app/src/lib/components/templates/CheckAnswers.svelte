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
		allStepsHref?: string;
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
		allStepsHref,
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
		<div class="body">
			<!--
				Every completed step expands, so the rail stops being a
				position marker and becomes the whole journey at a glance --
				which is what this page is for (#432).
			-->
			<StepRail {journey} {steps} {allStepsHref} expand="completed" />

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
		</div>
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		.body {
			display: grid;
			gap: var(--space-6);
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

		/*
		 * A hairline under each row and none around the list: the brief's
		 * rule that containers are declared by an edge rather than a box,
		 * and what makes nineteen rows scannable without nineteen borders.
		 */
		.row {
			display: grid;
			gap: var(--space-1);
			padding-block: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
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
		 * A row stacks before the page frame does, at its own threshold.
		 *
		 * No-wrap criterion, not overflow (#564): the overflow criterion
		 * fails here for the same reason it fails on a rail -- the row's
		 * tracks are flexible (`1fr 1fr auto`), and a wrappable string in
		 * a flexible track never overflows, it just wraps to one word per
		 * line. Swept with the query forced live, the row never overflows
		 * down to 284px, well under the 320px this repo verifies -- a
		 * threshold that never stops firing is not a floor at all
		 * (CONTEXT.md's own failure sentence: one configuration at every
		 * available space).
		 *
		 * What actually distinguishes the two configurations is wrapping,
		 * so that is what is measured: a label and a "Change" action are
		 * short, author-controlled strings, and if THEY wrap, the row is
		 * too narrow for the 3-column shape and stacking is the better
		 * read. A value is a Practice's own data of arbitrary length, and
		 * wrapping a long value is correct behaviour rather than a
		 * failure, so the value column is deliberately excluded from the
		 * criterion -- this is derived from what the content IS, not
		 * imported from GOV.UK's own number. Swept on
		 * /style-guide/check-answers's own demo, checking every `<dt>`
		 * and every `.action` link's line count (Range#getClientRects,
		 * immune to a grid row stretching a cell's box to match a
		 * WRAPPING sibling's height): the label is the one that wraps
		 * here, and it stops at 416px. 26rem is that fixed point exactly,
		 * no margin added. The action link never wraps at any width
		 * tested -- "Change" is too short. This does not corroborate the
		 * 40rem (640px) the previous, GOV.UK-derived comment carried; the
		 * measured floor is well below it.
		 */
		@container (min-width: 26rem) {
			.row {
				grid-template-columns: 1fr 1fr auto;
				gap: var(--space-4);
			}

			.action {
				text-align: end;
			}
		}

		/*
		 * The content floor, measured 2026-08-31 (#564): a measure
		 * criterion, not overflow -- `.column` is `minmax(0, --form-max)`
		 * in the default (non-wide) state, so it never overflows, but
		 * below this width the rail leaves it narrower than --form-max.
		 * Swept on /style-guide/check-answers's own demo with the query
		 * forced live at every width: `.column` first reaches --form-max
		 * at 1080px, the same fixed point `QuestionPage` measures
		 * independently -- both are `var(--page-rail) minmax(0,
		 * var(--form-max))` with the same `--space-12` gap, so the number
		 * is a pure function of those three tokens rather than a copy.
		 * 67.5rem is that fixed point exactly, no margin added. The
		 * previous 60rem (960px) was part of the shared 60rem set (#523);
		 * at 960px it left `.column` 110px short of --form-max, the same
		 * shortfall `QuestionPage` had.
		 */
		@container (min-width: 67.5rem) {
			.body {
				/*
				 * A summary of answers has nothing more to put in a wider
				 * window, so it does not take the room (#543). See
				 * `QuestionPage` for why the second track had to stop being
				 * `1fr`: `justify-content` distributes free space, and a
				 * flexible track leaves none.
				 */
				grid-template-columns: var(--page-rail) minmax(0, var(--form-max));
				column-gap: var(--space-12);
				align-items: start;
				justify-content: center;
			}

			/*
			 * The wide exit takes the room, because it has something to put
			 * there. Restoring the flexible track is the whole override:
			 * `justify-content` above then has no free space to distribute
			 * and stops centring by itself, which is the same rule read from
			 * the other end rather than an exception to it.
			 */
			.body:has(.column.wide) {
				grid-template-columns: var(--page-rail) minmax(0, 1fr);
			}
		}
	}
</style>
