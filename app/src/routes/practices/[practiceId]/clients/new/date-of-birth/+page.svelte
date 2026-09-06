<script lang="ts">
	/*
	 * The date of birth, in three boxes (#466, GOV.UK's Dates pattern).
	 * Never one field and never a picker: this is a date the reader
	 * already knows, so it is typed rather than navigated to.
	 *
	 * A `legend` question, so the group's hint is announced once from the
	 * <fieldset> `QuestionPage` owns -- repeating it on each of three
	 * boxes would say it three times, which is why `describedBy` arrives
	 * undefined here.
	 */
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import DateFields from '#lib/components/molecules/DateFields.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { joinDate, splitDate, type DateField, type DateParts } from '#lib/intakeDate.js';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import { journeySteps, nextStepHref, previousStepHref } from '#lib/intakeJourney.js';
	import type { FormError } from '#lib/formErrors.js';
	import IntakeActions from '../IntakeActions.svelte';
	import { JOURNEY, backHref, basePath, continueHref, knownAs, saveIntake, searchHref } from '../intake.js';

	const STEP = 'date-of-birth';
	const GROUP = 'intake-date-of-birth';

	const practiceId = $derived(page.params.practiceId ?? '');
	const base = $derived(basePath(practiceId));
	const steps = $derived(journeySteps(intakeFlow.steps, base, STEP, intakeDraft.visitedSteps));

	/*
	 * The boxes hold what was typed; the draft holds the composed
	 * "YYYY-MM-DD". They are seeded from the draft rather than bound to
	 * it, because `07` and `7` are the same stored date and a control
	 * that rewrote one into the other under the cursor is the formatting
	 * defect this pattern exists to avoid.
	 */
	let parts = $state<DateParts>(splitDate(intakeDraft.answers.dateOfBirth));
	let errors = $state<FormError[]>([]);
	let invalidField = $state<DateField | undefined>();
	let isSaving = $state(false);

	// Composes into the draft, or reports why it could not. Runs before
	// Continue and before a save, so a refused date never reaches the wire.
	function didCompose(): boolean {
		const result = joinDate(parts);
		if (!result.ok) {
			errors = [{ message: result.message, targetId: `${GROUP}-${result.field}` }];
			invalidField = result.field;
			return false;
		}
		errors = [];
		invalidField = undefined;
		intakeDraft.update({ dateOfBirth: result.value });
		return true;
	}

	async function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		if (!didCompose()) return;
		intakeDraft.visit(STEP);
		await goto(
			continueHref(page.url.searchParams, practiceId, nextStepHref(intakeFlow.steps, base, STEP))
		);
	}

	async function handleSaveForLater() {
		if (!didCompose()) return;
		isSaving = true;
		errors = (await saveIntake(practiceId, false)) ?? [];
		isSaving = false;
	}
</script>

<form onsubmit={handleContinue} novalidate>
	<QuestionPage
		journey={JOURNEY}
		{steps}
		backHref={backHref(
			page.url.searchParams,
			practiceId,
			previousStepHref(intakeFlow.steps, base, STEP, searchHref(practiceId))
		)}
		question={{ as: 'legend', text: `What is ${knownAs()}'s date of birth?` }}
		hint="This is what separates two Clients with the same name, next year and the year after. For example, 3 12 1988."
	>
		{#snippet errorSummary()}
			{#if errors.length > 0}
				<ErrorSummary {errors} />
			{/if}
		{/snippet}

		{#snippet content()}
			<DateFields
				name={GROUP}
				{parts}
				onChange={(next) => (parts = next)}
				error={errors[0]?.message}
				{invalidField}
			/>
		{/snippet}

		{#snippet actions()}
			<IntakeActions {isSaving} onSaveForLater={handleSaveForLater} />
		{/snippet}
	</QuestionPage>
</form>
