<script lang="ts">
	/*
	 * The first question, and the only one that cannot say the Client's
	 * name (#463): every page after this one can, because this is where
	 * it is given.
	 *
	 * A `legend` question -- three inputs under one <fieldset>, whose
	 * <legend> is the page's <h1>. `QuestionPage` owns that markup, so
	 * `describedBy` arrives undefined here and each field carries its own
	 * label and hint through `LabeledField`.
	 */
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import { journeySteps, nextStepHref } from '#lib/intakeJourney.js';
	import type { FormError } from '#lib/formErrors.js';
	import IntakeActions from '../IntakeActions.svelte';
	import { JOURNEY, backHref, basePath, continueHref, saveIntake, searchHref } from '../intake.js';

	const STEP = 'name';
	const GIVEN_NAME_ID = 'intake-given-name';
	const FAMILY_NAME_ID = 'intake-family-name';
	const PREFERRED_NAME_ID = 'intake-preferred-name';

	const practiceId = $derived(page.params.practiceId ?? '');
	const base = $derived(basePath(practiceId));
	const steps = $derived(journeySteps(intakeFlow.steps, base, STEP, intakeDraft.visitedSteps));

	let errors = $state<FormError[]>([]);
	let isSaving = $state(false);

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	// The one refusal on the whole sequence. ADR-0017 requires a given
	// name and nothing else, and CreateHandler refuses without one, so
	// asking here is the difference between a message beside the field
	// and a message from the server four pages later.
	function refusal(): FormError[] {
		return intakeDraft.answers.givenName.trim() === ''
			? [{ message: "Enter the Client's given name", targetId: GIVEN_NAME_ID }]
			: [];
	}

	async function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		errors = refusal();
		if (errors.length > 0) return;
		intakeDraft.visit(STEP);
		await goto(
			continueHref(page.url.searchParams, practiceId, nextStepHref(intakeFlow.steps, base, STEP))
		);
	}

	async function handleSaveForLater() {
		errors = refusal();
		if (errors.length > 0) return;
		isSaving = true;
		errors = (await saveIntake(practiceId, false)) ?? [];
		isSaving = false;
	}
</script>

<form onsubmit={handleContinue} novalidate>
	<QuestionPage
		journey={JOURNEY}
		{steps}
		backHref={backHref(page.url.searchParams, practiceId, searchHref(practiceId))}
		question={{ as: 'legend', text: "What is the Client's name?" }}
		hint="Only the given name is needed to save the record. The rest can be added at any time."
	>
		{#snippet errorSummary()}
			{#if errors.length > 0}
				<ErrorSummary {errors} />
			{/if}
		{/snippet}

		{#snippet content()}
			<stack-l space="var(--space-5)">
				<LabeledField id={GIVEN_NAME_ID} label="Given name" error={errorFor(GIVEN_NAME_ID)}>
					{#snippet children({ id, describedBy, invalid })}
						<TextInput
							{id}
							{describedBy}
							{invalid}
							value={intakeDraft.answers.givenName}
							onInput={(value) => intakeDraft.update({ givenName: value })}
							autocomplete="off"
						/>
					{/snippet}
				</LabeledField>
				<LabeledField id={FAMILY_NAME_ID} label="Family name">
					{#snippet children({ id, describedBy })}
						<TextInput
							{id}
							{describedBy}
							value={intakeDraft.answers.familyName}
							onInput={(value) => intakeDraft.update({ familyName: value })}
							autocomplete="off"
						/>
					{/snippet}
				</LabeledField>
				<LabeledField
					id={PREFERRED_NAME_ID}
					label="Preferred name"
					hint="What the Client is called day to day, if different"
				>
					{#snippet children({ id, describedBy })}
						<TextInput
							{id}
							{describedBy}
							value={intakeDraft.answers.preferredName}
							onInput={(value) => intakeDraft.update({ preferredName: value })}
							autocomplete="off"
						/>
					{/snippet}
				</LabeledField>
			</stack-l>
		{/snippet}

		{#snippet actions()}
			<IntakeActions {isSaving} onSaveForLater={handleSaveForLater} />
		{/snippet}
	</QuestionPage>
</form>
