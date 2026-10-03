<script lang="ts">
	/*
	 * Intake for a new Client: one question, her name (#1611, ADR-0017's
	 * amendment of 2026-10-02). Its one button saves the Client and opens
	 * the Start work form. The other details are added later from her
	 * record (#1610), so there are no later steps and no "Save and come
	 * back later".
	 *
	 * A `legend` question -- three inputs under one <fieldset>, whose
	 * <legend> is the page's <h1>. `QuestionPage` owns that markup, so
	 * each field carries its own label and hint through `LabeledField`.
	 *
	 * ## What the search carried
	 *
	 * A date of birth, an email address or a phone number typed into the
	 * search is in the draft, and the save sends it with the name, so the
	 * collision check has those keys. The page lists each one before the
	 * save, so nothing is saved that she cannot see.
	 *
	 * ## Why the <form> is outside the Template
	 *
	 * `QuestionPage` renders the controls and the actions as two separate
	 * regions of one column, so no <form> inside it could hold both. Round
	 * the outside, the submit button is inside the form that owns the
	 * inputs, which is what gives GOV.UK's implicit submission -- Enter,
	 * from any field, saves.
	 */
	import { page } from '#lib/appState.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { formatCalendarDay } from '#lib/dates.js';
	import { FormSubmission, orServiceProblem } from '#lib/formSubmission.svelte.js';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import {
		GIVEN_NAME_ID,
		INTAKE_FIELD_IDS,
		JOURNEY,
		exitHref,
		givenNameRefusal,
		intakeSteps,
		saveIntake
	} from '../intake.js';

	const FAMILY_NAME_ID = 'intake-family-name';
	const PREFERRED_NAME_ID = 'intake-preferred-name';

	const practiceId = $derived(page.params.practiceId ?? '');

	const submission = new FormSubmission();

	// The search's keys that are saved with the name, in the order the
	// search asks them. A key the search did not carry is not listed.
	const carriedAnswers = $derived(
		[
			{
				label: 'Date of birth',
				value: intakeDraft.answers.dateOfBirth && formatCalendarDay(intakeDraft.answers.dateOfBirth)
			},
			{ label: 'Email address', value: intakeDraft.answers.email.trim() },
			{ label: 'Phone number', value: intakeDraft.answers.phone.trim() }
		].filter((item) => item.value !== '')
	);

	async function handleSave(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			const refusal = givenNameRefusal();
			if (refusal.length > 0) return refusal;
			return await saveIntake(practiceId, false, INTAKE_FIELD_IDS);
		}, orServiceProblem);
	}
</script>

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs (see the note above), not to arrange anything. -->
<form onsubmit={handleSave} novalidate>
	<QuestionPage
		journey={JOURNEY}
		steps={intakeSteps(practiceId)}
		backHref={exitHref(practiceId, intakeDraft.origin)}
		question={{ as: 'legend', text: "What is the Client's name?" }}
		hint="Only the given name is needed to save the record. The rest can be added at any time."
	>
		{#snippet errorSummary()}
			{#if submission.errors.length > 0}
				<ErrorSummary errors={submission.errors} />
			{/if}
		{/snippet}

		{#snippet content()}
			<stack-l space="var(--space-5)">
				<LabeledField id={GIVEN_NAME_ID} label="Given name" error={submission.errorFor(GIVEN_NAME_ID)}>
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
				<LabeledField id={FAMILY_NAME_ID} label="Family name (optional)">
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
					label="Preferred name (optional)"
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
				{#if carriedAnswers.length > 0}
					<stack-l space="var(--space-2)">
						<Text text="From your search, also saved with the name:" />
						<DescriptionList items={carriedAnswers} />
					</stack-l>
				{/if}
			</stack-l>
		{/snippet}

		{#snippet actions()}
			<Button type="submit" label="Save and continue" loading={submission.isSubmitting} />
		{/snippet}
	</QuestionPage>
</form>
