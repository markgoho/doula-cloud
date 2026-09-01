<script module lang="ts">
	/*
	 * The question, and the two shapes it takes. A discriminated union
	 * rather than a flag plus a nullable id, so the `for` that a label
	 * needs cannot be forgotten and cannot be supplied where it means
	 * nothing.
	 */
	export type Question =
		| { as: 'legend'; text: string }
		| { as: 'label'; text: string; for: string };
</script>

<script lang="ts">
	/*
	 * Archetype E, first half -- one question per page. ADR-0018, amended
	 * on #464.
	 *
	 * ## Why this is not a prop on `FormPage`
	 *
	 * `FormPage` renders `<Heading level={1}>` and, separately,
	 * `<fieldset><legend>`. A GOV.UK question page needs the legend -- or
	 * the label, where the page holds one input -- to *be* the <h1>, so a
	 * screen reader announces the question once instead of twice. That is a
	 * different tree, not a different attribute: the Dates pattern's own
	 * markup is `<legend><h1></h1></legend>`. #432 found it while drawing
	 * the intake sequence, and a mode flag on `FormPage` would have hidden
	 * two genuinely different page shapes behind a boolean -- which is the
	 * thing ADR-0018's "two named exits" rule exists to prevent.
	 *
	 * ## The error summary is a region and never markup
	 *
	 * This Template owns *where* the summary goes -- below the back link,
	 * above the <h1>, which is GOV.UK's position and is page-level
	 * arrangement. It owns none of what goes in it. #467 builds the
	 * component; rendering a `Notice` or an inline error box here would
	 * make this the second place that pattern lives, which is the
	 * duplication #467 exists to remove.
	 */
	import type { Snippet } from 'svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import StepRail, { type JourneyStep } from '#lib/components/organisms/StepRail.svelte';

	interface Properties {
		/**
		 * Names the rail's landmark. "Adding a client".
		 */
		journey: string;
		/**
		 * The Staff side leaves this unset and gets the product name; the
		 * Client portal passes its Practice's name (#431, #487).
		 */
		serviceName?: string;
		steps: JourneyStep[];
		allStepsHref?: string;
		backHref: string;
		/**
		 * GOV.UK's error summary, positioned by this Template and built by
		 * the route (#467). Nothing renders here when it is absent -- not an
		 * empty box, not a hidden live region.
		 */
		errorSummary?: Snippet;
		/**
		 * GOV.UK's section caption: where in the journey this question sits.
		 */
		caption?: string;
		question: Question;
		/**
		 * GOV.UK hint text: what the question is for, in the reader's words.
		 */
		hint?: string;
		/**
		 * The one thing the page asks -- a fieldset's controls, a single
		 * input, a radio group.
		 *
		 * `describedBy` is handed in the way `LabeledField` hands one to its
		 * children, and it is deliberately undefined in `legend` mode: a
		 * group's hint is announced from the <fieldset>, so repeating it on
		 * each of three date inputs would say it three times.
		 */
		content: Snippet<[{ describedBy: string | undefined }]>;
		/**
		 * `Continue`, and `Save and come back later`.
		 */
		actions: Snippet;
	}

	let {
		journey,
		serviceName,
		steps,
		allStepsHref,
		backHref,
		errorSummary,
		caption,
		question,
		hint,
		content,
		actions
	}: Properties = $props();

	const pageId = $props.id();
	const hintId = $derived(hint ? `${pageId}-hint` : undefined);
</script>

<PageTitle page={question.text} {serviceName} isError={Boolean(errorSummary)} />

{#snippet questionCaption()}
	{#if caption}
		<span class="caption">{caption}</span>
	{/if}
{/snippet}

{#snippet questionHint()}
	{#if hint}
		<p class="hint" id={hintId}>{hint}</p>
	{/if}
{/snippet}

<container-l>
	<!-- No cap on the frame: it holds the rail beside the column, and the
	     column carries the readability cap itself (#541). -->
	<center-l max="none" gutters="var(--page-gutter)">
		<div class="body">
			<StepRail {journey} {steps} {allStepsHref} expand="current" />

			<div class="column">
				<stack-l space="var(--space-6)">
					<BackLink href={backHref} />

					{#if errorSummary}
						{@render errorSummary()}
					{/if}

					{#if question.as === 'legend'}
						<!--
							The <fieldset> wraps the question and the controls it
							groups, and the hint is announced from the group rather
							than from any one control inside it -- GOV.UK's own
							markup for a multi-input question.
						-->
						<fieldset aria-describedby={hintId}>
							<legend>
								{@render questionCaption()}
								<h1>{question.text}</h1>
							</legend>
							{@render questionHint()}
							<div class="thing">{@render content({ describedBy: undefined })}</div>
						</fieldset>
					{:else}
						<!--
							Label-as-h1: the <h1> wraps the <label>, so the heading
							reads as the question and the control's accessible name
							is the question too, without either being said twice.
							The caption sits inside the heading and outside the
							label, so it is part of the page's title and not part of
							the control's name.
						-->
						<div>
							<h1>
								{@render questionCaption()}
								<label for={question.for}>{question.text}</label>
							</h1>
							{@render questionHint()}
						</div>
						<div class="thing">{@render content({ describedBy: hintId })}</div>
					{/if}

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

		/*
		 * The question column is capped at --form-max while the frame
		 * around it is not: the column is all controls, and a 1200px input
		 * is unreadable however wide the window is (#422). The rail takes
		 * its own width outside that cap.
		 */
		.column {
			max-inline-size: var(--form-max);
		}

		fieldset {
			margin: 0;
			padding: 0;
			border: 0;
			min-inline-size: 0;
		}

		legend {
			padding: 0;
		}

		/*
		 * The question is the page title, so it takes the page step of the
		 * type scale. Not reached through `Heading`: that atom renders the
		 * element itself, and here the <h1> has to sit inside a <legend> or
		 * wrap a <label>, which is markup no prop on it can express.
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

		/* Spelled out rather than `font: inherit`, per #417: the shorthand
		   silently resets font-variant-numeric, which is what carries the
		   tabular-figures rule. */
		h1 label {
			display: block;
			font-family: inherit;
			font-size: inherit;
			font-weight: inherit;
			line-height: inherit;
			letter-spacing: inherit;
			color: inherit;
		}

		/*
		 * Quieter and above the question, so it locates the reader without
		 * competing with what is being asked.
		 */
		.caption {
			display: block;
			margin-block-end: var(--space-1);
			font-size: var(--text-body-size);
			font-weight: var(--font-weight-normal);
			color: var(--color-on-surface-muted);
		}

		.hint {
			margin: var(--space-2) 0 0;
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		/* 28px from the question to the thing it asks for -- the brief's gap
		   between a labelled group and the next, which is what this is. */
		.thing {
			margin-block-start: var(--space-7);
		}

		/*
		 * The content floor, measured 2026-08-31 (#564): a measure
		 * criterion, not overflow -- `.column` is `minmax(0, --form-max)`,
		 * so it never overflows, but below this width the rail leaves it
		 * narrower than --form-max, which is not the designed width GOV.UK's
		 * form guidance sizes the controls for. Swept on
		 * /style-guide/question-page's own demo with the query forced live
		 * at every width: `.column` first reaches --form-max at 1080px.
		 * 67.5rem is that fixed point exactly, no margin added -- it is a
		 * pure function of --page-rail, --space-12 and --form-max, so
		 * `CheckAnswers`'s own 60rem query below measures to the same
		 * number independently rather than by being copied here. The
		 * previous 60rem (960px) was part of the shared 60rem set (#523)
		 * rather than measured against this column; at 960px it left
		 * `.column` 110px short of --form-max.
		 */
		@container (min-width: 67.5rem) {
			.body {
				/*
				 * A page that asks one question has nothing more to put in a
				 * wider window, so it does not take the room (#543). The
				 * leftover goes on both sides rather than all of it at the
				 * inline end. This is not --page-max coming back: nothing
				 * here names a page width, and both tracks are the sizes the
				 * rail and the column already had.
				 *
				 * The second track had to stop being `1fr` for this to work
				 * at all -- a flexible track absorbs every spare pixel, so
				 * `justify-content` would have had nothing left to
				 * distribute. `--form-max` is the same cap `.column` carries
				 * for the stacked state below this query, moved onto the
				 * track so that free space exists.
				 *
				 * The column keeps the width it had: measured on the drag
				 * surface, `fit-content` on the grid centres the pair too but
				 * sizes the column to its own longest line, which would make
				 * one journey's steps each a different width.
				 */
				grid-template-columns: var(--page-rail) minmax(0, var(--form-max));
				column-gap: var(--space-12);
				align-items: start;
				justify-content: center;
			}
		}
	}
</style>
