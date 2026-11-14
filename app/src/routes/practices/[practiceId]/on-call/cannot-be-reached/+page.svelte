<script lang="ts">
	/*
	 * Step 1 of recording a time a Doula cannot be reached (#2934), for an
	 * Owner or Admin: which Doula. It opens with nothing chosen (ADR-0055)
	 * unless the roster she came from was narrowed to one Doula, whose id the
	 * address carries; a Doula is never asked, and goes on to the time.
	 */
	import { goto } from '$app/navigation';
	import { untrack } from 'svelte';
	import { apiFetchWithSession } from '#lib/api.js';
	import { cannotBeReachedDraft as draft } from '#lib/cannotBeReachedDraft.svelte.js';
	import { loadDoulaChoices, type CoverOption } from '#lib/cannotBeReached.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import ErrorSummary, { type FormError } from '#lib/components/molecules/ErrorSummary.svelte';
	import ReadFailure from '#lib/components/molecules/ReadFailure.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { errorsFromCause } from '#lib/formErrors.js';
	import { dayIn } from '#lib/zonedTime.js';
	import { JOURNEY_NAME, Journey } from './journey.svelte.js';

	const SELECT_ID = 'cannot-be-reached-doula';
	const journey = new Journey();

	let choices = $state.raw<CoverOption[]>();
	let failure = $state('');
	let chosen = $state(untrack(() => journey.address.doula || draft.answers.staffId));
	let errors = $state<FormError[]>([]);

	$effect(() => {
		if (!journey.isOwnerView) void goto(journey.href('when'), { replaceState: true });
	});

	async function read() {
		failure = '';
		choices = undefined;
		try {
			choices = await loadDoulaChoices(apiFetchWithSession, journey.practiceId, dayIn(new Date(), journey.session.timezone));
		} catch (error) {
			failure = errorsFromCause(error)[0]!.message;
		}
	}
	void read();

	function next(event: SubmitEvent) {
		event.preventDefault();
		if (chosen === '') {
			errors = [{ message: 'Select the Doula who cannot be reached', targetId: SELECT_ID }];
			return;
		}
		if (chosen !== draft.answers.staffId) {
			draft.update({ staffId: chosen, staffName: choices?.find((choice) => choice.staffId === chosen)?.name ?? '', cover: {} });
		}
		void goto(journey.href('when'));
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
<form onsubmit={next} novalidate>
	<QuestionPage
		journey={JOURNEY_NAME}
		steps={journey.steps('doula')}
		backHref={journey.rosterHref}
		question={{ as: 'label', text: 'Which Doula cannot be reached?', for: SELECT_ID }}
		error={errors[0]?.message}
		errorSummary={errors.length > 0 ? errorSummary : undefined}
	>
		{#snippet content({ describedBy })}
			{#if failure}
				<ReadFailure message={failure} onRetry={read} />
			{:else if !choices}
				<Skeleton variant="row" lines={1} label="Loading the Doulas" />
			{:else if choices.length === 0}
				<Text text="No Doula is on a birth with days on call in the next 92 days." tone="variant" />
			{:else}
				<Select
					id={SELECT_ID}
					{describedBy}
					invalid={errors.length > 0}
					options={choices.map((choice) => ({ value: choice.staffId, label: choice.name }))}
					placeholder="Choose a Doula"
					bind:value={chosen}
				/>
			{/if}
		{/snippet}
		{#snippet actions()}
			<Button type="submit" label="Continue" />
		{/snippet}
	</QuestionPage>
</form>
