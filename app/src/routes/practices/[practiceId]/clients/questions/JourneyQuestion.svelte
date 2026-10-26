<script lang="ts">
	/*
	 * One question of a Client's details journey, as a page (#466, #1610).
	 *
	 * Every shared question in this folder composes this. What they share
	 * is everything except the question and the controls under it: where
	 * the rail's data comes from, where Back and Continue go, the Change
	 * round trip, and the error summary's position. The journey comes in
	 * as `journey`, so the page reads no module state of its own. Intake
	 * for a new Client is one question with its own save (#1611), so it
	 * does not compose this.
	 *
	 * ## What a route still owns
	 *
	 * The question, the hint, the controls, and -- through `validate` --
	 * whether Continue is allowed to happen. The date page composes three
	 * boxes into one string and refuses a date that is not real.
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
	import Button from '#lib/components/atoms/Button.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import QuestionPage, { type Question } from '#lib/components/templates/QuestionPage.svelte';
	import { journeySteps, nextStepHref, previousStepHref, type StepId } from '#lib/intakeJourney.js';
	import { FormSubmission, orThrownErrors, type FormError } from '#lib/formSubmission.svelte.js';
	import { checkOr, isFromRecord, type QuestionJourney } from './questionJourney.js';

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

	/*
	 * A Change on her record opened this one question (#2048): the reader
	 * came to change one thing, so the page is a journey of one -- no rail
	 * (`QuestionPage` draws none for one step) -- Back goes to the record,
	 * and the button saves, landing on the record with a status Notice.
	 */
	const isRecordChange = $derived(isFromRecord(page.url.searchParams));
	const steps = $derived(
		journeySteps(journey.steps, journey.basePath, stepId, journey.draft.visitedSteps).filter(
			(step) => !isRecordChange || step.status === 'current'
		)
	);

	/*
	 * What the last `validate` said, so the error summary and the controls
	 * both see the same refusals. Continue saves nothing -- the details
	 * journey saves at its check page -- except on a Change from her
	 * record, whose save is the `submission` below.
	 */
	let errors = $state<FormError[]>([]);

	// The save's own refusals: the server's, which `validate` cannot know.
	const submission = new FormSubmission();
	const shownErrors = $derived(errors.length > 0 ? errors : submission.errors);

	async function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		errors = validate?.() ?? [];
		if (errors.length > 0) return;
		journey.draft.visit(stepId);
		if (isRecordChange) {
			await submission.run(
				() => journey.save(`${page.url.pathname}${page.url.search}`),
				orThrownErrors({})
			);
			return;
		}
		await goto(
			checkOr(
				page.url.searchParams,
				journey.basePath,
				nextStepHref(journey.steps, journey.basePath, stepId)
			)
		);
	}
</script>

<!-- Passed only while there is a refusal, never declared as a child of
     the Template: a declared snippet is always truthy, so the Template
     would title a first visit "Error: " (#1705). -->
{#snippet errorSummary()}
	<ErrorSummary errors={shownErrors} />
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs (see the note above), not to arrange anything. -->
<form onsubmit={handleContinue} novalidate>
	<QuestionPage
		journey={journey.label}
		{steps}
		backHref={isRecordChange
			? journey.exitHref
			: checkOr(
					page.url.searchParams,
					journey.basePath,
					previousStepHref(journey.steps, journey.basePath, stepId, journey.exitHref)
				)}
		{question}
		{hint}
		errorSummary={shownErrors.length > 0 ? errorSummary : undefined}
	>
		{#snippet content({ describedBy })}
			{@render controls({ describedBy, errors })}
		{/snippet}

		{#snippet actions()}
			<!-- The submit, so the form submits on Enter from any field. -->
			<Button
				type="submit"
				label={isRecordChange ? 'Save' : 'Continue'}
				loading={submission.isSubmitting}
			/>
		{/snippet}
	</QuestionPage>
</form>
