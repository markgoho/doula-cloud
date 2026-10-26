<script lang="ts">
	/*
	 * Her name, the journey's first question (#2048). The same three
	 * fields intake's name question asks (`clients/new/name`), each with
	 * its own label through `LabeledField`, under one `legend` question --
	 * `QuestionPage` owns the <fieldset>, and its <legend> is the <h1>.
	 *
	 * The given name is the one value a Client record cannot be without
	 * (ADR-0017), so it is the one refusal here; the other two may be
	 * cleared. A save that changes the name runs the duplicate check
	 * (`detailsSave`), which is how two records are combined.
	 */
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import type { FormError } from '#lib/formErrors.js';
	import JourneyQuestion from './JourneyQuestion.svelte';
	import type { QuestionJourney } from './questionJourney.js';

	let { journey }: { journey: QuestionJourney } = $props();

	const GIVEN_NAME_ID = 'details-given-name';
	const FAMILY_NAME_ID = 'details-family-name';
	const PREFERRED_NAME_ID = 'details-preferred-name';

	function validate(): FormError[] {
		if (journey.draft.answers.givenName.trim() !== '') return [];
		return [{ message: 'Enter a given name', targetId: GIVEN_NAME_ID }];
	}
</script>

<JourneyQuestion
	{journey}
	stepId="name"
	question={{ as: 'legend', text: `What is ${journey.knownAs}'s name?` }}
	hint="Only the given name is needed."
	{validate}
>
	{#snippet controls({ errors })}
		<stack-l space="var(--space-5)">
			<LabeledField
				id={GIVEN_NAME_ID}
				label="Given name"
				error={errors.find((entry) => entry.targetId === GIVEN_NAME_ID)?.message}
			>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						value={journey.draft.answers.givenName}
						onInput={(value) => journey.draft.update({ givenName: value })}
						autocomplete="off"
					/>
				{/snippet}
			</LabeledField>
			<LabeledField id={FAMILY_NAME_ID} label="Family name (optional)">
				{#snippet children({ id, describedBy })}
					<TextInput
						{id}
						{describedBy}
						value={journey.draft.answers.familyName}
						onInput={(value) => journey.draft.update({ familyName: value })}
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
						value={journey.draft.answers.preferredName}
						onInput={(value) => journey.draft.update({ preferredName: value })}
						autocomplete="off"
					/>
				{/snippet}
			</LabeledField>
		</stack-l>
	{/snippet}
</JourneyQuestion>
