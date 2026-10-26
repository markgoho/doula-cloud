<script lang="ts">
	/*
	 * One page of the signup journey (#2049), the same shape as a Client's
	 * details journey's `JourneyQuestion`: the rail, Back and Continue, the
	 * Change round trip from the check page, and the error summary's place.
	 * A page owns its question, its controls and what stops its Continue.
	 *
	 * Continue sends nothing. The journey creates the Practice once, from
	 * its check page, so this page only checks what it asks and moves on.
	 *
	 * The <form> is outside the Template for `JourneyQuestion`'s reason:
	 * `QuestionPage` renders the controls and the actions as two regions,
	 * and the submit button has to be inside the form that owns the
	 * inputs for Enter to continue from any field.
	 */
	import { untrack, type Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import QuestionPage, { type Question } from '#lib/components/templates/QuestionPage.svelte';
	import type { FormError } from '#lib/formSubmission.svelte.js';
	import { CHANGE_PARAMETER, CHANGE_VALUE } from '#lib/intakeJourney.js';
	import {
		JOURNEY_CAPTION,
		JOURNEY_LABEL,
		backHref,
		nextHref,
		railSteps,
		signupDraft,
		type SignupStepId
	} from './signupJourney.svelte.js';

	interface Properties {
		stepId: SignupStepId;
		question: Question;
		hint?: string;
		/**
		What stops Continue. A page that asks nothing required omits it.
		*/
		validate?: () => FormError[] | Promise<FormError[]>;
		/**
		 * The controls. `errorFor` reads the refusals this page shows, its
		 * own and any the check page's button carried here.
		 */
		controls: Snippet<
			[{ describedBy: string | undefined; errorFor: (targetId: string) => string | undefined }]
		>;
	}

	let { stepId, question, hint, validate, controls }: Properties = $props();

	const isFromCheck = $derived(page.url.searchParams.get(CHANGE_PARAMETER) === CHANGE_VALUE);
	const steps = $derived(railSteps(stepId, signupDraft.visited));

	// Opened on what the check page's button was refused for, when that
	// refusal belongs to a control on this page (#2049).
	let errors = $state<FormError[]>(untrack(() => signupDraft.refusalsFor(stepId)));
	let isChecking = $state(false);

	function errorFor(targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}

	async function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		isChecking = true;
		try {
			errors = (await validate?.()) ?? [];
		} finally {
			isChecking = false;
		}
		if (errors.length > 0) return;
		signupDraft.settle(stepId);
		signupDraft.visit(stepId);
		await goto(nextHref(stepId, isFromCheck));
	}
</script>

<!-- Passed only while there is a refusal: a declared snippet is always
     truthy, so the Template would title a first visit "Error: " (#1705). -->
{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
<form onsubmit={handleContinue} novalidate>
	<QuestionPage
		journey={JOURNEY_LABEL}
		{steps}
		backHref={backHref(stepId, isFromCheck)}
		caption={JOURNEY_CAPTION}
		{question}
		{hint}
		error={question.as === 'label' ? errorFor(question.for) : undefined}
		errorSummary={errors.length > 0 ? errorSummary : undefined}
	>
		{#snippet content({ describedBy })}
			{@render controls({ describedBy, errorFor })}
		{/snippet}

		{#snippet actions()}
			<Button type="submit" label="Continue" loading={isChecking} />
		{/snippet}
	</QuestionPage>
</form>
