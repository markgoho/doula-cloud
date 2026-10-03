<script lang="ts">
	/*
	 * The date of birth, in three boxes (#466, GOV.UK's Dates pattern).
	 * Never one field and never a picker: this is a date the reader
	 * already knows, so it is typed rather than navigated to.
	 *
	 * A `legend` question, so the group's hint is announced once from the
	 * <fieldset> `QuestionPage` owns -- repeating it on each of three
	 * boxes would say it three times, which is why `describedBy` arrives
	 * undefined here and is not passed on.
	 *
	 * Optional, and the legend says so (GOV.UK's Question pages pattern,
	 * #1610): ADR-0017 requires a given name and nothing else.
	 */
	import { untrack } from 'svelte';
	import DateFields from '#lib/components/molecules/DateFields.svelte';
	import { dateFieldId, dateGroupRefusal, joinDate, splitDate, type DateParts } from '#lib/intakeDate.js';
	import type { FormError } from '#lib/formErrors.js';
	import JourneyQuestion from './JourneyQuestion.svelte';
	import { DATE_OF_BIRTH_GROUP, type QuestionJourney } from './questionJourney.js';

	let { journey }: { journey: QuestionJourney } = $props();

	// One literal, shared with intake's `intakeFieldIds` so the summary
	// entry and the boxes it points at cannot drift apart.
	const GROUP = DATE_OF_BIRTH_GROUP;

	/*
	 * The boxes hold what was typed; the draft holds the composed
	 * "YYYY-MM-DD". They are seeded from the draft rather than bound to
	 * it, because `07` and `7` are the same stored date and a control
	 * that rewrote one into the other under the cursor is the formatting
	 * defect this pattern exists to avoid.
	 */
	let parts = $state<DateParts>(splitDate(untrack(() => journey.draft.answers.dateOfBirth)));

	// Composes into the draft, or reports why it could not -- run before
	// Continue and before a save, so a refused date never reaches the wire.
	// Which box is wrong is read back out of the returned refusal by
	// `dateGroupRefusal` below, not tracked here a second time.
	function compose(): FormError[] {
		const result = joinDate(parts);
		if (!result.ok) {
			return [{ message: result.message, targetId: dateFieldId(GROUP, result.field) }];
		}
		journey.draft.update({ dateOfBirth: result.value });
		return [];
	}
</script>

<JourneyQuestion
	{journey}
	stepId="date-of-birth"
	question={{ as: 'legend', text: `What is ${journey.knownAs}'s date of birth? (optional)` }}
	hint="This is what separates two Clients with the same name, next year and the year after. For example, 3 12 1988."
	validate={compose}
>
	{#snippet controls({ errors })}
		{@const refusal = dateGroupRefusal(errors, GROUP)}
		<DateFields
			name={GROUP}
			{parts}
			onChange={(next) => (parts = next)}
			error={refusal?.message}
			invalidField={refusal?.field}
		/>
	{/snippet}
</JourneyQuestion>
