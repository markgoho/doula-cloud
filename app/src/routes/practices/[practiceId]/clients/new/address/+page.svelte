<script lang="ts">
	/*
	 * The address -- the five structural columns intake never reached
	 * before #466. One <fieldset>, five inputs, and the field widths this
	 * ticket makes the route's own business: the Templates guarantee a
	 * --form-max column and set no widths, and a ZIP code narrower than
	 * an address line is content sizing rather than page arrangement.
	 */
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import { journeySteps, nextStepHref, previousStepHref } from '#lib/intakeJourney.js';
	import type { FormError } from '#lib/formErrors.js';
	import IntakeActions from '../IntakeActions.svelte';
	import { JOURNEY, backHref, basePath, continueHref, knownAs, saveIntake, searchHref } from '../intake.js';

	const STEP = 'address';

	const practiceId = $derived(page.params.practiceId ?? '');
	const base = $derived(basePath(practiceId));
	const steps = $derived(journeySteps(intakeFlow.steps, base, STEP, intakeDraft.visitedSteps));

	let errors = $state<FormError[]>([]);
	let isSaving = $state(false);

	async function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		intakeDraft.visit(STEP);
		await goto(
			continueHref(page.url.searchParams, practiceId, nextStepHref(intakeFlow.steps, base, STEP))
		);
	}

	async function handleSaveForLater() {
		isSaving = true;
		errors = (await saveIntake(practiceId, false)) ?? [];
		isSaving = false;
	}
</script>

<form onsubmit={handleContinue} novalidate>
	<QuestionPage
		journey={JOURNEY}
		{steps}
		backHref={backHref(
			page.url.searchParams,
			practiceId,
			previousStepHref(intakeFlow.steps, base, STEP, searchHref(practiceId))
		)}
		question={{ as: 'legend', text: `What is ${knownAs()}'s address?` }}
		hint="Where a Practice sends anything on paper, and where a Doula drives to."
	>
		{#snippet errorSummary()}
			{#if errors.length > 0}
				<ErrorSummary {errors} />
			{/if}
		{/snippet}

		{#snippet content()}
			<stack-l space="var(--space-5)">
				<LabeledField id="intake-address-line-1" label="Address line 1">
					{#snippet children({ id, describedBy })}
						<TextInput
							{id}
							{describedBy}
							value={intakeDraft.answers.addressLine1}
							onInput={(value) => intakeDraft.update({ addressLine1: value })}
							autocomplete="off"
						/>
					{/snippet}
				</LabeledField>
				<LabeledField id="intake-address-line-2" label="Address line 2 (optional)">
					{#snippet children({ id, describedBy })}
						<TextInput
							{id}
							{describedBy}
							value={intakeDraft.answers.addressLine2}
							onInput={(value) => intakeDraft.update({ addressLine2: value })}
							autocomplete="off"
						/>
					{/snippet}
				</LabeledField>
				<div class="town">
					<LabeledField id="intake-address-locality" label="City">
						{#snippet children({ id, describedBy })}
							<TextInput
								{id}
								{describedBy}
								value={intakeDraft.answers.addressLocality}
								onInput={(value) => intakeDraft.update({ addressLocality: value })}
								autocomplete="off"
							/>
						{/snippet}
					</LabeledField>
				</div>
				<div class="short">
					<LabeledField id="intake-address-region" label="State">
						{#snippet children({ id, describedBy })}
							<TextInput
								{id}
								{describedBy}
								value={intakeDraft.answers.addressRegion}
								onInput={(value) => intakeDraft.update({ addressRegion: value })}
								autocomplete="off"
							/>
						{/snippet}
					</LabeledField>
				</div>
				<div class="short">
					<LabeledField id="intake-address-postal-code" label="ZIP code">
						{#snippet children({ id, describedBy })}
							<TextInput
								{id}
								{describedBy}
								value={intakeDraft.answers.addressPostalCode}
								onInput={(value) => intakeDraft.update({ addressPostalCode: value })}
								autocomplete="off"
							/>
						{/snippet}
					</LabeledField>
				</div>
			</stack-l>
		{/snippet}

		{#snippet actions()}
			<IntakeActions {isSaving} onSaveForLater={handleSaveForLater} />
		{/snippet}
	</QuestionPage>
</form>

<style>
	@layer components {
		/*
		 * Content sizing, which #466 makes this route's own business. A
		 * ZIP code is five characters and a two-letter state is two, so a
		 * box that could hold a street name tells the reader the wrong
		 * thing about what goes in it -- GOV.UK's Text input sizing rule.
		 * `ch` tracks the font rather than a canvas measurement, and
		 * `max-inline-size` rather than a width, so at 320px the box
		 * shrinks with the column instead of overflowing it (ADR-0024).
		 */
		.town {
			max-inline-size: 24ch;
		}

		.short {
			max-inline-size: 12ch;
		}
	}
</style>
