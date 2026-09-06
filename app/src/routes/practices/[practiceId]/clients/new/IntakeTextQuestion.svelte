<script lang="ts">
	/*
	 * A page that asks for one thing and holds one box.
	 *
	 * Two identical consumers -- the email address and the phone number,
	 * which GOV.UK keeps as separate patterns and separate pages rather
	 * than one "contact details" screen. That is ADR-0018's bar for an
	 * extraction, met exactly; anything more specific (the name's three
	 * boxes, the address's five, the date's three) stays in its own
	 * route, because a component that took a list of fields would be
	 * `FormPage` again.
	 *
	 * It is a `{ as: 'label' }` question, which is where the Template's
	 * `describedBy` has to be passed through to the input and the input
	 * has to carry the same id that went in as `question.for` -- the two
	 * halves #464 left to the route. Doing it once here is why it cannot
	 * be forgotten on one page and not the other.
	 */
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import { journeySteps, nextStepHref, previousStepHref } from '#lib/intakeJourney.js';
	import type { FormError } from '#lib/formErrors.js';
	import IntakeActions from './IntakeActions.svelte';
	import { JOURNEY, backHref, basePath, continueHref, saveIntake, searchHref } from './intake.js';

	interface Properties {
		stepId: string;
		fieldId: string;
		question: string;
		hint?: string;
		type?: 'text' | 'email' | 'tel';
		value: string;
		onInput: (value: string) => void;
	}

	let { stepId, fieldId, question, hint, type = 'text', value, onInput }: Properties = $props();

	const practiceId = $derived(page.params.practiceId ?? '');
	const base = $derived(basePath(practiceId));
	const steps = $derived(journeySteps(intakeFlow.steps, base, stepId, intakeDraft.visitedSteps));

	let errors = $state<FormError[]>([]);
	let isSaving = $state(false);

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

<form onsubmit={handleContinue} novalidate>
	<QuestionPage
		journey={JOURNEY}
		{steps}
		backHref={backHref(
			page.url.searchParams,
			practiceId,
			previousStepHref(intakeFlow.steps, base, stepId, searchHref(practiceId))
		)}
		question={{ as: 'label', text: question, for: fieldId }}
		{hint}
	>
		{#snippet errorSummary()}
			{#if errors.length > 0}
				<ErrorSummary {errors} />
			{/if}
		{/snippet}

		{#snippet content({ describedBy })}
			<TextInput id={fieldId} {describedBy} {type} {value} {onInput} autocomplete="off" />
		{/snippet}

		{#snippet actions()}
			<IntakeActions {isSaving} onSaveForLater={handleSaveForLater} />
		{/snippet}
	</QuestionPage>
</form>
