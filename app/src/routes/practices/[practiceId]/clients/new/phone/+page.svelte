<script lang="ts">
	/*
	 * The phone number, on its own page -- see the email step's comment
	 * for why the two are not one screen (#466).
	 */
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import IntakeTextQuestion from '../IntakeTextQuestion.svelte';
	import { knownAs } from '../intake.js';
</script>

<IntakeTextQuestion
	stepId="phone"
	fieldId="intake-phone"
	question={`What is ${knownAs()}'s phone number?`}
	hint="For international numbers include the country code."
	type="tel"
	value={intakeDraft.answers.phone}
	onInput={(value) => intakeDraft.update({ phone: value })}
/>
