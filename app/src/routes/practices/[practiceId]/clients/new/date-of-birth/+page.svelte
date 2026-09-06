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
	 */
	import DateFields from '#lib/components/molecules/DateFields.svelte';
	import { joinDate, splitDate, type DateField, type DateParts } from '#lib/intakeDate.js';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import type { FormError } from '#lib/formErrors.js';
	import IntakeQuestion from '../IntakeQuestion.svelte';
	import { knownAs } from '../intake.js';

	const GROUP = 'intake-date-of-birth';

	/*
	 * The boxes hold what was typed; the draft holds the composed
	 * "YYYY-MM-DD". They are seeded from the draft rather than bound to
	 * it, because `07` and `7` are the same stored date and a control
	 * that rewrote one into the other under the cursor is the formatting
	 * defect this pattern exists to avoid.
	 */
	let parts = $state<DateParts>(splitDate(intakeDraft.answers.dateOfBirth));
	let invalidField = $state<DateField | undefined>();

	// Composes into the draft, or reports why it could not -- run before
	// Continue and before a save, so a refused date never reaches the wire.
	function compose(): FormError[] {
		const result = joinDate(parts);
		if (!result.ok) {
			invalidField = result.field;
			return [{ message: result.message, targetId: `${GROUP}-${result.field}` }];
		}
		invalidField = undefined;
		intakeDraft.update({ dateOfBirth: result.value });
		return [];
	}
</script>

<IntakeQuestion
	stepId="date-of-birth"
	question={{ as: 'legend', text: `What is ${knownAs()}'s date of birth?` }}
	hint="This is what separates two Clients with the same name, next year and the year after. For example, 3 12 1988."
	validate={compose}
>
	{#snippet controls({ errors })}
		<DateFields
			name={GROUP}
			{parts}
			onChange={(next) => (parts = next)}
			error={errors[0]?.message}
			{invalidField}
		/>
	{/snippet}
</IntakeQuestion>
