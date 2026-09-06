<script lang="ts">
	/*
	 * The end of the sequence: everything intake was told, with a way
	 * back to each question, and the save (#466, GOV.UK's Check answers
	 * pattern).
	 *
	 * No step is `current` here, which is what makes `StepRail` read out
	 * the count rather than a position and what opens every completed
	 * step -- on this page the whole answered journey is the content
	 * rather than a marker beside it.
	 */
	import { page } from '#lib/appState.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import CheckAnswers from '#lib/components/templates/CheckAnswers.svelte';
	import { answerSections } from '#lib/intakeAnswers.js';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import { journeySteps } from '#lib/intakeJourney.js';
	import type { FormError } from '#lib/formErrors.js';
	import { givenNameRefusal, JOURNEY, basePath, knownAs, saveIntake } from '../intake.js';

	/*
	 * GOV.UK's wider column for a long answer list. Eleven structural
	 * rows is already a long page; a Practice with sections of its own
	 * takes it past twenty, which is the case the Template's `isWide`
	 * was built for.
	 */
	const WIDE_FROM_ROWS = 14;

	const practiceId = $derived(page.params.practiceId ?? '');
	const base = $derived(basePath(practiceId));
	const steps = $derived(journeySteps(intakeFlow.steps, base, undefined, intakeDraft.visitedSteps));
	const sections = $derived(answerSections(intakeDraft.answers, intakeFlow.sections, base));
	const rowCount = $derived(sections.reduce((total, section) => total + section.answers.length, 0));

	const lastStep = $derived(intakeFlow.steps.at(-1));

	let errors = $state<FormError[]>([]);
	let isSaving = $state(false);

	async function handleSave() {
		errors = givenNameRefusal('the-summary');
		if (errors.length > 0) return;
		isSaving = true;
		errors = (await saveIntake(practiceId, false)) ?? [];
		isSaving = false;
	}
</script>

<CheckAnswers
	journey={JOURNEY}
	{steps}
	backHref={lastStep ? `${base}/${lastStep.slug}` : base}
	title="Check {knownAs()}'s details before saving"
	{sections}
	isWide={rowCount >= WIDE_FROM_ROWS}
>
	{#snippet errorSummary()}
		{#if errors.length > 0}
			<ErrorSummary {errors} />
		{/if}
	{/snippet}

	{#snippet actions()}
		<Button label="Save this Client" loading={isSaving} onClick={handleSave} />
	{/snippet}
</CheckAnswers>
