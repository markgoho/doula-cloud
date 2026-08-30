<script module lang="ts">
	export interface JourneyStep {
		label: string;
		href: string;
		status: 'completed' | 'current' | 'todo';
		/*
		 * The questions inside this step. Shown when the step is expanded,
		 * which is the whole reason the rail is a rail and not a progress
		 * bar: #432 wanted the journey legible at a glance on the summary
		 * page, and a step title alone does not say what was answered
		 * under it.
		 */
		questions?: { label: string; href: string }[];
	}
</script>

<script lang="ts">
	/*
	 * The journey rail that `QuestionPage` and `CheckAnswers` both carry
	 * (#464). Two identical consumers, which is exactly ADR-0018's bar for
	 * earning its own component; an organism rather than a molecule on
	 * #424's rule -- a molecule is a part of a section, an organism is a
	 * whole one, and this is a whole page region.
	 *
	 * ## It is a <nav>, and that is an amendment to ADR-0018
	 *
	 * ADR-0018 says a Template renders no navigation and no landmark. This
	 * renders both, on the account owner's call (#464). The rule's reason
	 * was that chrome is site-wide and session-derived, so a Template that
	 * rendered it could not be dropped into any route or rendered in a test
	 * with no session. A journey rail is neither: it is page-scoped, handed
	 * in as data by the route, and means nothing outside the one sequence
	 * it belongs to. The shell cannot render it, because the shell does not
	 * know the journey.
	 *
	 * #432's drawing asked for the <nav> *before* <main>. That is not
	 * available: #452 put <main> in the shell, so everything a Template
	 * renders is already inside it. A <nav> inside <main> is valid, is a
	 * landmark either way, and is what the account owner chose over a
	 * plain unnamed list.
	 *
	 * ## Steps are data in
	 *
	 * The rail never computes its own steps. #432 requires that a Practice
	 * which has added no Client fields gets five steps rather than six with
	 * an empty one, and that derivation reads the Practice's own Client
	 * Field Template -- an endpoint that does not exist yet (#460). So the
	 * shape here is a typed array, and building it is the route's job
	 * (#466).
	 */
	import Link from '#lib/components/atoms/Link.svelte';

	interface Properties {
		/**
		 * Names the landmark, and captions the narrow strip.
		 */
		journey: string;
		steps: JourneyStep[];
		/**
		 * Which steps show their questions. `current` is a question page,
		 * where the reader is inside one step; `completed` is the summary
		 * page, where there is no current step and the whole answered
		 * journey is the point. A closed pair rather than a boolean,
		 * because these are two behaviours and not one thing switched off.
		 */
		expand?: 'current' | 'completed';
		/**
		 * Where the narrow strip's "Show all steps" goes. Narrow has no room
		 * for the rail, so the full list becomes a page of its own -- which
		 * is a route, and therefore #466's to build. Omit it and the strip
		 * shows no link.
		 */
		allStepsHref?: string;
	}

	let { journey, steps, expand = 'current', allStepsHref }: Properties = $props();

	const railId = $props.id();

	const statusLabels = {
		completed: 'Completed',
		current: 'In progress',
		todo: 'Not started'
	} as const;

	const currentStep = $derived(steps.find((step) => step.status === 'current'));
	const completedCount = $derived(steps.filter((step) => step.status === 'completed').length);

	/*
	 * Goal-Gradient and Parkinson, from the brief: a multi-step form has to
	 * show where the end is. On a question page that is the step number; on
	 * the summary page there is no current step, so it is the count.
	 */
	const summary = $derived(
		currentStep === undefined
			? `${completedCount} of ${steps.length} steps completed`
			: `Step ${steps.indexOf(currentStep) + 1} of ${steps.length} · ${currentStep.label}`
	);
	const completedPercent = $derived(
		steps.length === 0 ? 0 : Math.round((completedCount / steps.length) * 100)
	);
	const trackWidth = $derived(`${completedPercent}%`);

	function statusId(index: number) {
		return `${railId}-status-${index}`;
	}

	function isExpanded(step: JourneyStep) {
		return expand === 'completed' ? step.status === 'completed' : step.status === 'current';
	}
</script>

<nav aria-label={journey}>
	<div class="rail">
		<ol>
			<!--
				Keyed on index rather than on the href, the same call
				`CheckAnswers` and `DescriptionList` make on this ticket: a
				journey is an ordered sequence, a step has no identity beyond
				where it sits, and two entries that happen to share a
				destination are a duplicate key that throws rather than a
				collision that hides a row.
			-->
			{#each steps as step, index (index)}
				<li>
					<Link
						variant="step"
						href={step.href}
						label={step.label}
						current={step.status === 'current'}
						describedBy={statusId(index)}
					/>
					<p class="status" id={statusId(index)}>{statusLabels[step.status]}</p>
					{#if step.questions && isExpanded(step)}
						<ol class="questions">
							{#each step.questions as question, questionIndex (questionIndex)}
								<li>
									<Link variant="rail" href={question.href} label={question.label} />
								</li>
							{/each}
						</ol>
					{/if}
				</li>
			{/each}
		</ol>
	</div>

	<!--
		The same journey where there is no room beside the column. Inside the
		one <nav> rather than in a second one of its own: two landmarks
		sharing a label is an axe `landmark-unique` failure, and `display:
		none` only takes the hidden half out of the accessibility tree -- it
		does not make two names one.
	-->
	<div class="strip">
		<p class="summary">{summary}</p>
		<div class="track" aria-hidden="true">
			<div class="track-fill" style:inline-size={trackWidth}></div>
		</div>
		{#if allStepsHref}
			<Link href={allStepsHref} label="Show all steps" variant="secondary" />
		{/if}
	</div>
</nav>

<style>
	@layer components {
		ol {
			margin: 0;
			padding: 0;
			list-style: none;
		}

		/* No gap: each step's 2px leading bar meets its neighbour's, so the
		   column of transparent bars reads as one track down the list and
		   the active one lights a segment of it. That is the same reason
		   RecordDetail's contents list carries no gap. */
		.rail > ol {
			display: grid;
		}

		/* Aligned under the step's own label rather than under its bar:
		   the status belongs to the words, and a status flush with the
		   track would read as another step. */
		.status {
			margin: 0 0 var(--space-3);
			padding-inline-start: calc(var(--space-4) + var(--border-active));
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		.questions {
			display: grid;
			margin-block-end: var(--space-3);
			padding-inline-start: var(--space-4);
		}

		.summary {
			margin: 0;
			color: var(--color-on-surface);
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-medium);
		}

		.track {
			block-size: var(--space-1);
			margin-block: var(--space-2);
			border-radius: var(--radius);
			background-color: var(--color-surface-container);
		}

		.track-fill {
			block-size: 100%;
			border-radius: var(--radius);
			background-color: var(--color-primary);
		}

		/* Narrow first: the rail needs a column beside the content, and
		   below the Templates' 60rem threshold there is not one. The same
		   threshold OverviewHub and RecordDetail use, so every archetype
		   changes shape at one width rather than at three a person would
		   notice. */
		.rail {
			display: none;
		}

		@container (min-width: 60rem) {
			.rail {
				display: block;
			}

			.strip {
				display: none;
			}
		}
	}
</style>
