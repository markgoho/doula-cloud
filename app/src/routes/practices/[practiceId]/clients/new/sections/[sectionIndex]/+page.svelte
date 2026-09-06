<script lang="ts">
	/*
	 * One page per Practice-named section (#466, ADR-0017's
	 * Practice-defined layer).
	 *
	 * The sections are computed, never written down: `intakeJourney.ts`
	 * splits the Practice's Client Field Template on its `section_header`
	 * entries, and fields that sit before the first header become a
	 * section headed with the Practice's own name. A Practice that has
	 * added nothing has no sections, so this route is never reached and
	 * the journey is five steps rather than six with an empty one.
	 *
	 * A `legend` question: the section's name is the page's <h1> and the
	 * <fieldset> groups every field asked under it.
	 */
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import ClientFieldAnswers from '#lib/components/organisms/ClientFieldAnswers.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import { journeySteps, nextStepHref, previousStepHref } from '#lib/intakeJourney.js';
	import type { FormError } from '#lib/formErrors.js';
	import IntakeActions from '../../IntakeActions.svelte';
	import { JOURNEY, backHref, basePath, continueHref, saveIntake, searchHref } from '../../intake.js';

	const practiceId = $derived(page.params.practiceId ?? '');
	const base = $derived(basePath(practiceId));
	const index = $derived(Number(page.params.sectionIndex));
	const section = $derived(intakeFlow.sections[index]);
	const stepId = $derived(`section-${index}`);
	const steps = $derived(journeySteps(intakeFlow.steps, base, stepId, intakeDraft.visitedSteps));

	let errors = $state<FormError[]>([]);
	let isSaving = $state(false);

	// A Practice can archive its last field, or rename a section, between
	// one visit and the next -- and a bookmark outlives either. An index
	// with no section behind it is the end of the sequence rather than an
	// error page: the summary is where it would have led anyway.
	$effect(() => {
		if (section === undefined && intakeFlow.status === 'ready') {
			void goto(`${base}/check`);
		}
	});

	async function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		intakeDraft.visit(stepId);
		await goto(
			continueHref(page.url.searchParams, practiceId, nextStepHref(intakeFlow.steps, base, stepId))
		);
	}

	async function handleSaveForLater() {
		isSaving = true;
		errors = (await saveIntake(practiceId, false)) ?? [];
		isSaving = false;
	}
</script>

{#if section}
	<form onsubmit={handleContinue} novalidate>
		<QuestionPage
			journey={JOURNEY}
			{steps}
			backHref={backHref(
				page.url.searchParams,
				practiceId,
				previousStepHref(intakeFlow.steps, base, stepId, searchHref(practiceId))
			)}
			question={{ as: 'legend', text: section.heading }}
			hint="These are the questions this Practice asks. Every one of them can be left for later."
		>
			{#snippet errorSummary()}
				{#if errors.length > 0}
					<ErrorSummary {errors} />
				{/if}
			{/snippet}

			{#snippet content()}
				<ClientFieldAnswers
					fields={section.fields}
					values={intakeDraft.answers.fieldValues}
					onChange={(fieldId, value) => intakeDraft.setFieldValue(fieldId, value)}
					idPrefix="intake-field-{index}"
				/>
			{/snippet}

			{#snippet actions()}
				<IntakeActions {isSaving} onSaveForLater={handleSaveForLater} />
			{/snippet}
		</QuestionPage>
	</form>
{/if}
