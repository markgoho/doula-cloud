<script lang="ts">
	/*
	 * The email address, on its own page. GOV.UK has a pattern for an
	 * email address and a pattern for a phone number and none for
	 * "contact details", which is why this is not one screen with the
	 * next one (#466).
	 */
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import IntakeTextQuestion from '../IntakeTextQuestion.svelte';
	import { knownAs } from '../intake.js';
</script>

<IntakeTextQuestion
	stepId="email"
	fieldId="intake-email"
	question={`What is ${knownAs()}'s email address?`}
	hint="An email address is what a Client uses to reach the portal. It can be added later."
	type="email"
	value={intakeDraft.answers.email}
	onInput={(value) => intakeDraft.update({ email: value })}
/>
