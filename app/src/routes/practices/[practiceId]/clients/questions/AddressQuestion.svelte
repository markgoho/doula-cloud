<script lang="ts">
	/*
	 * The address -- the five structural columns intake never reached
	 * before #466. One <fieldset>, five inputs, and a width for each line
	 * that holds a short answer: a ZIP code narrower than an address line
	 * is content sizing rather than page arrangement.
	 *
	 * The legend says "(optional)" (#1610, GOV.UK's Question pages
	 * pattern), and it says it for every line under it: no line of an
	 * address is required, so the second line no longer says it again on
	 * its own, which read as if the other four were.
	 */
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput, { type InputWidth } from '#lib/components/atoms/TextInput.svelte';
	import type { IntakeAnswers } from '#lib/intakeDraft.svelte.js';
	import JourneyQuestion from './JourneyQuestion.svelte';
	import type { QuestionJourney } from './questionJourney.js';

	let { journey }: { journey: QuestionJourney } = $props();

	/*
	 * Five near-identical blocks became one snippet after a review of
	 * #466: what actually differs between them is the column, its label
	 * and how wide the box is. The widths are GOV.UK's Addresses pattern
	 * on the shared scale `TextInput` argues (#1216): a town at 20
	 * characters, a state and a ZIP code at 10 (ZIP+4 is ten). Until then
	 * they were two classes on a wrapper here, 24ch and 12ch. The two
	 * address lines set no width and keep the column.
	 */
	const lines: { key: keyof IntakeAnswers; label: string; width?: InputWidth }[] = [
		{ key: 'addressLine1', label: 'Address line 1' },
		{ key: 'addressLine2', label: 'Address line 2' },
		{ key: 'addressLocality', label: 'City', width: 20 },
		{ key: 'addressRegion', label: 'State', width: 10 },
		{ key: 'addressPostalCode', label: 'ZIP code', width: 10 }
	];

	function idFor(key: string): string {
		return `intake-${key.replaceAll(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase()}`;
	}
</script>

<JourneyQuestion
	{journey}
	stepId="address"
	question={{ as: 'legend', text: `What is ${journey.knownAs}'s address? (optional)` }}
	hint="Where a Practice sends anything on paper, and where a Doula drives to."
>
	{#snippet controls()}
		<stack-l space="var(--space-5)">
			{#each lines as line (line.key)}
				<LabeledField id={idFor(line.key)} label={line.label} width={line.width}>
					{#snippet children({ id, describedBy, width })}
						<TextInput
							{id}
							{describedBy}
							{width}
							value={String(journey.draft.answers[line.key] ?? '')}
							onInput={(value) => journey.draft.update({ [line.key]: value })}
							autocomplete="off"
						/>
					{/snippet}
				</LabeledField>
			{/each}
		</stack-l>
	{/snippet}
</JourneyQuestion>
