<script lang="ts">
	/*
	 * The phone number, on its own page -- see `EmailQuestion`'s comment
	 * for why the two are not one screen (#466). Optional, and the label
	 * says so (#1610).
	 */
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import JourneyQuestion from './JourneyQuestion.svelte';
	import type { QuestionJourney } from './questionJourney.js';

	let { journey }: { journey: QuestionJourney } = $props();

	const FIELD_ID = 'intake-phone';
</script>

<JourneyQuestion
	{journey}
	stepId="phone"
	question={{ as: 'label', text: `What is ${journey.knownAs}'s phone number? (optional)`, for: FIELD_ID }}
	hint="For a number outside the United States, include the country code."
>
	{#snippet controls({ describedBy })}
		<TextInput
			id={FIELD_ID}
			{describedBy}
			type="tel"
			value={journey.draft.answers.phone}
			onInput={(value) => journey.draft.update({ phone: value })}
			autocomplete="off"
		/>
	{/snippet}
</JourneyQuestion>
