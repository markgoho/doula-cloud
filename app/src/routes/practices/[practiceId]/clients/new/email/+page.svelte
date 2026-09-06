<script lang="ts">
	/*
	 * The email address, on its own page. GOV.UK has a pattern for an
	 * email address and a pattern for a phone number and none for
	 * "contact details", which is why this is not one screen with the
	 * next one (#466).
	 *
	 * A `{ as: 'label' }` question, which is where the Template's
	 * `describedBy` has to reach the input and the input has to carry the
	 * same id that went in as `question.for` -- the two halves #464 left
	 * to the route.
	 */
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import IntakeQuestion from '../IntakeQuestion.svelte';
	import { knownAs } from '../intake.js';

	const FIELD_ID = 'intake-email';
</script>

<IntakeQuestion
	stepId="email"
	question={{ as: 'label', text: `What is ${knownAs()}'s email address?`, for: FIELD_ID }}
	hint="An email address is what a Client uses to reach the portal. It can be added later."
>
	{#snippet controls({ describedBy })}
		<TextInput
			id={FIELD_ID}
			{describedBy}
			type="email"
			value={intakeDraft.answers.email}
			onInput={(value) => intakeDraft.update({ email: value })}
			autocomplete="off"
		/>
	{/snippet}
</IntakeQuestion>
