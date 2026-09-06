<script lang="ts">
	/*
	 * The phone number, on its own page -- see the email step's comment
	 * for why the two are not one screen (#466).
	 */
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import IntakeQuestion from '../IntakeQuestion.svelte';
	import { knownAs } from '../intake.js';

	const FIELD_ID = 'intake-phone';
</script>

<IntakeQuestion
	stepId="phone"
	question={{ as: 'label', text: `What is ${knownAs()}'s phone number?`, for: FIELD_ID }}
	hint="For a number outside the United States, include the country code."
>
	{#snippet controls({ describedBy })}
		<TextInput
			id={FIELD_ID}
			{describedBy}
			type="tel"
			value={intakeDraft.answers.phone}
			onInput={(value) => intakeDraft.update({ phone: value })}
			autocomplete="off"
		/>
	{/snippet}
</IntakeQuestion>
