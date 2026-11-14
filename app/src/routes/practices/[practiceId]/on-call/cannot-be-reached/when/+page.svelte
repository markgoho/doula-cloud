<script lang="ts">
	/*
	 * Step 2 of recording a time a Doula cannot be reached (#2934): from when
	 * until when, and an optional reason. The two times are read in the
	 * Practice's zone, as the Engagement's gap form reads them (ADR-0036).
	 */
	import { goto } from '$app/navigation';
	import { cannotBeReachedDraft as draft } from '#lib/cannotBeReachedDraft.svelte.js';
	import { whenQuestion } from '#lib/cannotBeReached.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import ErrorSummary, { type FormError } from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { practiceZoneNote } from '#lib/practiceZone.js';
	import { instantFromWall } from '#lib/zonedTime.js';
	import { JOURNEY_NAME, Journey } from '../journey.svelte.js';

	const IDS = { startsAt: 'cannot-be-reached-from', endsAt: 'cannot-be-reached-until', reason: 'cannot-be-reached-reason' };
	const MOST_REASON = 500;
	const journey = new Journey();

	let errors = $state<FormError[]>([]);
	const errorFor = (id: string) => errors.find((entry) => entry.targetId === id)?.message;
	const zoneNote = practiceZoneNote(true);

	$effect(() => {
		if (journey.isOwnerView && !draft.answers.staffId) void goto(journey.href('doula'), { replaceState: true });
	});

	const instantOf = (local: string) => {
		const instant = instantFromWall(local, journey.session.timezone);
		return local === '' || Number.isNaN(instant.getTime()) ? undefined : instant;
	};

	function next(event: SubmitEvent) {
		event.preventDefault();
		const startsAt = instantOf(draft.answers.startsAt);
		const endsAt = instantOf(draft.answers.endsAt);
		const found: FormError[] = [];
		if (!startsAt) found.push({ message: 'Enter when the gap starts', targetId: IDS.startsAt });
		if (!endsAt) found.push({ message: 'Enter when the gap ends', targetId: IDS.endsAt });
		if (startsAt && endsAt && endsAt <= startsAt) found.push({ message: 'The gap must end after it starts', targetId: IDS.endsAt });
		if (draft.answers.reason.trim().length > MOST_REASON) {
			found.push({ message: `Keep the reason to ${MOST_REASON} characters or fewer.`, targetId: IDS.reason });
		}
		errors = found;
		if (found.length === 0) void goto(journey.nextAfter('when'));
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
<form onsubmit={next} novalidate>
	<QuestionPage
		journey={JOURNEY_NAME}
		steps={journey.steps('when')}
		backHref={journey.backFrom('when')}
		question={{ as: 'legend', text: whenQuestion(journey.isOwnerView, draft.answers.staffName) }}
		errorSummary={errors.length > 0 ? errorSummary : undefined}
	>
		{#snippet content()}
			{#if zoneNote}
				<Text text={zoneNote} tone="variant" />
			{/if}
			<LabeledField id={IDS.startsAt} label="From" error={errorFor(IDS.startsAt)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="datetime-local"
						value={draft.answers.startsAt}
						onInput={(value) => draft.update({ startsAt: value })}
					/>
				{/snippet}
			</LabeledField>
			<LabeledField id={IDS.endsAt} label="Until" error={errorFor(IDS.endsAt)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="datetime-local"
						value={draft.answers.endsAt}
						onInput={(value) => draft.update({ endsAt: value })}
					/>
				{/snippet}
			</LabeledField>
			<LabeledField
				id={IDS.reason}
				label="Reason"
				hint="Optional. Owners, Admins and the Doulas on each birth can read it; the Client never does."
				error={errorFor(IDS.reason)}
			>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput {id} {describedBy} {invalid} value={draft.answers.reason} onInput={(value) => draft.update({ reason: value })} />
				{/snippet}
			</LabeledField>
		{/snippet}
		{#snippet actions()}
			<Button type="submit" label="Continue" />
		{/snippet}
	</QuestionPage>
</form>
