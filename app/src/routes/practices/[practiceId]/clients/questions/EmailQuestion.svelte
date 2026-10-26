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
	 * to the route. Optional, and the label says so (#1610).
	 */
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import JourneyQuestion from './JourneyQuestion.svelte';
	import type { QuestionJourney } from './questionJourney.js';

	/*
	 * `emailOnFile` is the address her record holds. A save that changes
	 * it revokes any portal invite still waiting at the old address
	 * (`portalinvite/outbox.go`'s live-read-at-send rule), so the page
	 * says so before the save rather than after it -- what the Edit form
	 * said until #2048 retired it.
	 */
	let { journey, emailOnFile = '' }: { journey: QuestionJourney; emailOnFile?: string } = $props();

	const FIELD_ID = 'intake-email';

	const hasChangedEmail = $derived(
		emailOnFile !== '' && journey.draft.answers.email.trim() !== emailOnFile
	);
</script>

<JourneyQuestion
	{journey}
	stepId="email"
	question={{ as: 'label', text: `What is ${journey.knownAs}'s email address? (optional)`, for: FIELD_ID }}
	hint="An email address is what a Client uses to reach the portal. It can be added later."
>
	{#snippet controls({ describedBy })}
		<stack-l space="var(--space-4)">
			<TextInput
				id={FIELD_ID}
				{describedBy}
				type="email"
				value={journey.draft.answers.email}
				onInput={(value) => journey.draft.update({ email: value })}
				autocomplete="off"
			/>
			{#if hasChangedEmail}
				<Notice
					variant="info"
					message="Saving a new email address cancels any portal invite sent to the old one."
				/>
			{/if}
		</stack-l>
	{/snippet}
</JourneyQuestion>
