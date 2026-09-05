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
	<!-- No cap on the frame: the column carries the readability cap
	     itself (#541). -->
	<center-l max="none" gutters="var(--page-gutter)">
		<!--
			The journey sits ABOVE the question, full width, and this
			Template no longer uses sidebar-l (#585).

			It used to put the journey in a --page-rail column beside the
			question, and StepRail then guessed with a container query
			whether that column existed. It could not: the wrap is a
			flexbox event no selector reports, and the guess was measured
			wrong across a 125px band. Now nothing guesses. A question
			page asks one question, so a permanent rail beside it is the
			rail-column-and-nothing #543 already called out; collapsed
			above the question, the journey costs one line at every
			width and the reader opens it when they want it.

			CheckAnswers keeps its sidebar-l: there the whole answered
			journey IS the page's content, so it earns the column.
		-->
		<div class="column">
			<stack-l space="var(--space-6)">
				<BackLink href={backHref} />

				<!-- After the back link and before the question: the back link
				     is where the reader came from, the journey is where they
				     are, and the question is what is being asked. Inside the
				     stack, so it takes the same rhythm as everything else in
				     the column rather than needing spacing of its own. -->
				<StepRail {journey} {steps} expand="current" />

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
	</center-l>
</container-l>

<style>
	@layer components {
		container-l {
			padding-block: var(--space-8);
		}

		/*
		 * The question column is capped at --form-max while the frame
		 * around it is not: the column is all controls, and a 1200px input
		 * is unreadable however wide the window is (#422). Centred rather
		 * than left at the inline start, because a page that asks one
		 * question has nothing else to put in a wider window (#543) --
		 * which is what the sidebar-l this replaced achieved with
		 * `justify-content: center` once its rail wrapped away (#585).
		 *
		 * The journey is inside this cap now rather than beside it. It is
		 * one collapsed line, so it costs the column nothing, and a
		 * summary line that ran to 1200px would be as unreadable as the
		 * inputs under it.
		 */
		.column {
			max-inline-size: var(--form-max);
			margin-inline: auto;
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

		/* Capped at --measure, not --form-max (#609): the column is sized
		   for a field, but a hint is read as a sentence, and --form-max
		   alone would run it to roughly 90ch. No margin-inline: auto --
		   the hint stays flush left with the question above it. */
		.hint {
			margin: var(--space-2) 0 0;
			max-inline-size: var(--measure);
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		/* 28px from the question to the thing it asks for -- the brief's gap
		   between a labelled group and the next, which is what this is. */
		.thing {
			margin-block-start: var(--space-7);
		}

	}
</style>
