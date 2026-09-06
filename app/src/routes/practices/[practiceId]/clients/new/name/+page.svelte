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
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import type { FormError } from '#lib/formErrors.js';
	import IntakeQuestion from '../IntakeQuestion.svelte';
	import { GIVEN_NAME_ID, givenNameRefusal } from '../intake.js';

	const FAMILY_NAME_ID = 'intake-family-name';
	const PREFERRED_NAME_ID = 'intake-preferred-name';

	function errorFor(errors: FormError[], targetId: string): string | undefined {
		return errors.find((entry) => entry.targetId === targetId)?.message;
	}
</script>

<IntakeQuestion
	stepId="name"
	question={{ as: 'legend', text: "What is the Client's name?" }}
	hint="Only the given name is needed to save the record. The rest can be added at any time."
	validate={givenNameRefusal}
>
	{#snippet controls({ errors })}
		<stack-l space="var(--space-5)">
			<LabeledField
				id={GIVEN_NAME_ID}
				label="Given name"
				error={errorFor(errors, GIVEN_NAME_ID)}
			>
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
</IntakeQuestion>
