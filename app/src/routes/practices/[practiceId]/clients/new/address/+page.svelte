<script lang="ts">
	/*
	 * The address -- the five structural columns intake never reached
	 * before #466. One <fieldset>, five inputs, and the field widths this
	 * ticket makes the route's own business: the Templates guarantee a
	 * --form-max column and set no widths, and a ZIP code narrower than
	 * an address line is content sizing rather than page arrangement.
	 */
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import type { IntakeAnswers } from '#lib/intakeDraft.svelte.js';
	import IntakeQuestion from '../IntakeQuestion.svelte';
	import { knownAs } from '../intake.js';

	/*
	 * Five near-identical blocks became one snippet after a review of
	 * this ticket: what actually differs between them is the column, its
	 * label and how wide the box is. `width` is a class rather than a
	 * length so the sizes stay together in one place in the stylesheet
	 * below, where the reason for each is written once.
	 */
	const lines: { key: keyof IntakeAnswers; label: string; width: 'line' | 'town' | 'short' }[] = [
		{ key: 'addressLine1', label: 'Address line 1', width: 'line' },
		{ key: 'addressLine2', label: 'Address line 2 (optional)', width: 'line' },
		{ key: 'addressLocality', label: 'City', width: 'town' },
		{ key: 'addressRegion', label: 'State', width: 'short' },
		{ key: 'addressPostalCode', label: 'ZIP code', width: 'short' }
	];

	function idFor(key: string): string {
		return `intake-${key.replaceAll(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase()}`;
	}
</script>

<IntakeQuestion
	stepId="address"
	question={{ as: 'legend', text: `What is ${knownAs()}'s address?` }}
	hint="Where a Practice sends anything on paper, and where a Doula drives to."
>
	{#snippet controls()}
		<stack-l space="var(--space-5)">
			{#each lines as line (line.key)}
				<div class={line.width}>
					<LabeledField id={idFor(line.key)} label={line.label}>
						{#snippet children({ id, describedBy })}
							<TextInput
								{id}
								{describedBy}
								value={String(intakeDraft.answers[line.key] ?? '')}
								onInput={(value) => intakeDraft.update({ [line.key]: value })}
								autocomplete="off"
							/>
						{/snippet}
					</LabeledField>
				</div>
			{/each}
		</stack-l>
	{/snippet}
</IntakeQuestion>

<style>
	@layer components {
		/*
		 * Content sizing, which #466 makes this route's own business. A
		 * ZIP code is five characters and a two-letter state is two, so a
		 * box that could hold a street name tells the reader the wrong
		 * thing about what goes in it -- GOV.UK's Text input sizing rule.
		 * `ch` tracks the font rather than a canvas measurement, and
		 * `max-inline-size` rather than a width, so at 320px each box
		 * shrinks with the column instead of overflowing it (ADR-0024).
		 *
		 * The `:global(input)` is what makes any of it visible.
		 * `TextInput` sets no width of its own, so an <input> takes the
		 * browser's default `size` -- about 208px -- whatever column it is
		 * put in, and a 12ch wrapper around one would have been a box the
		 * control painted straight out of. `Select` and `Textarea` both
		 * stretch, so the three controls in one form do not agree; that is
		 * app-wide, predates this ticket, and is #805, which takes these
		 * three rules out when it lands.
		 */
		.line :global(input),
		.town :global(input),
		.short :global(input) {
			inline-size: 100%;
		}

		.town {
			max-inline-size: 24ch;
		}

		.short {
			max-inline-size: 12ch;
		}
	}
</style>
