<script lang="ts">
	/*
	 * One question of a Client journey, as a page (#466, #1610).
	 *
	 * Intake's name page and every shared question in this folder compose
	 * this, in either journey: intake for a new Client, or her details
	 * added from her record. What they share is everything except the
	 * question and the controls under it: where the rail's data comes
	 * from, where Back and Continue go, the Change round trip, the error
	 * summary's position, and -- in intake only -- the free save. Which
	 * journey it is comes in as `journey`, so the page never reaches for
	 * the other journey's state.
	 *
	 * ## What a route still owns
	 *
	 * The question, the hint, the controls, and -- through `validate` --
	 * whether Continue is allowed to happen. The name page refuses a blank
	 * given name; the date page composes three boxes into one string and
	 * refuses a date that is not real.
	 *
	 * ## Why the <form> is outside the Template
	 *
	 * `QuestionPage` renders the controls and the actions as two separate
	 * regions of one column, so no <form> inside it could hold both. Round
	 * the outside, the submit button is inside the form that owns the
	 * inputs, which is what gives GOV.UK's implicit submission -- Enter,
	 * from any field, continues.
	 */
	import type { Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import QuestionPage, { type Question } from '#lib/components/templates/QuestionPage.svelte';
	import { journeySteps, nextStepHref, previousStepHref, type StepId } from '#lib/intakeJourney.js';
	import { FormSubmission, orServiceProblem, type FormError } from '#lib/formSubmission.svelte.js';
	import QuestionActions from './QuestionActions.svelte';
	import { checkOr, type QuestionJourney } from './questionJourney.js';

	interface Properties {
		journey: QuestionJourney;
		stepId: StepId;
		question: Question;
		hint?: string;
		/**
		 * What stops Continue, and what stops a save. Returns the refusals
		 * to show, or an empty array to go on. A route that only collects
		 * -- the address, a Practice's own section -- omits it: ADR-0017
		 * requires a given name and nothing else, so there is nothing else
		 * to refuse.
		 */
		validate?: () => FormError[];
		/**
		 * The controls. `describedBy` is the Template's own, handed through
		 * the way `LabeledField` hands one to its children, and `errors` is
		 * whatever the last `validate` returned so a control can mark
		 * itself.
		 */
		controls: Snippet<[{ describedBy: string | undefined; errors: FormError[] }]>;
	}

	let { journey, stepId, question, hint, validate, controls }: Properties = $props();

	const steps = $derived(
		journeySteps(journey.steps, journey.basePath, stepId, journey.draft.visitedSteps)
	);

	/*
	 * One `FormSubmission`, but only `handleSaveForLater` drives it through
	 * `run`: `QuestionActions`' own spinner belongs to that button alone,
	 * and sharing `isSubmitting` with Continue's own (busy-free) navigation
	 * would flip it on a click Continue never asked to be busy for.
	 * `handleContinue` writes `submission.errors` directly instead -- the
	 * same array, so the summary and the two actions never disagree about
	 * what is wrong.
	 */
	const submission = new FormSubmission();

	// Runs `validate` and keeps what it said, so the error summary and the
	// controls both see the same refusals.
	function isRefused(): boolean {
		submission.errors = validate?.() ?? [];
		return submission.errors.length > 0;
	}

	async function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		if (isRefused()) return;
		journey.draft.visit(stepId);
		await goto(
			checkOr(
				page.url.searchParams,
				journey.basePath,
				nextStepHref(journey.steps, journey.basePath, stepId)
			)
		);
	}

	// Only offered where the journey has one -- see `QuestionJourney`.
	const saveForLater = $derived(journey.saveForLater);

	async function handleSaveForLater(save: NonNullable<QuestionJourney['saveForLater']>) {
		if (isRefused()) return;
		await submission.run(() => save(stepId), orServiceProblem);
	}
</script>

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs (see the note above), not to arrange anything. -->
<form onsubmit={handleContinue} novalidate>
	<QuestionPage
		journey={journey.label}
		{steps}
		backHref={checkOr(
			page.url.searchParams,
			journey.basePath,
			previousStepHref(journey.steps, journey.basePath, stepId, journey.exitHref)
		)}
		{question}
		{hint}
	>
		{#snippet errorSummary()}
			{#if submission.errors.length > 0}
				<ErrorSummary errors={submission.errors} />
			{/if}
		{/snippet}

		{#snippet content({ describedBy })}
			{@render controls({ describedBy, errors: submission.errors })}
		{/snippet}

		{#snippet actions()}
			<QuestionActions
				isSaving={submission.isSubmitting}
				onSaveForLater={saveForLater && (() => handleSaveForLater(saveForLater))}
			/>
		{/snippet}
	</QuestionPage>
</form>
