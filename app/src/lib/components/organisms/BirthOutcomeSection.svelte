<script lang="ts">
	/**
	 * The Engagement hub's birth-outcome section (#943): what happened to
	 * the pregnancy, the date it ended, and the one path that corrects
	 * either. ADR-0015's second fact, and #293's endpoint's only screen.
	 *
	 * ## It never asks unprompted
	 *
	 * ADR-0015: "Nadia does not need the software to tell her what
	 * happened; she needs it to stop asking her about a pregnancy." So an
	 * Engagement with no outcome recorded says so in one line and offers a
	 * control -- the question itself is behind that control, the same way
	 * this page's own "Mark care complete" reason question is, rather than
	 * a radio group asking about a possible loss on every page view.
	 *
	 * ## Recorded is read, not edited
	 *
	 * A recorded pair is frozen (00093's trigger), so it renders as facts
	 * with no field to type in. Correcting it is a named, separate act an
	 * Owner takes, and clearing it is a second one -- a `loss` typed onto
	 * the wrong Engagement has to be removable, not merely changeable,
	 * because `unknown` would claim the Practice looked and never learned.
	 *
	 * ## The frozen press-through
	 *
	 * `BIRTH_OUTCOME_FROZEN` reaches this component when the row was
	 * recorded by somebody else while this page was open. It is a
	 * confirmation, not an error: an Owner presses through it with
	 * `ConfirmDialog`, which is the house block-over-warn shape (#473) --
	 * a hard block with an override that names the act. A reader who may
	 * not correct one gets the BFF's own sentence instead, because for her
	 * it really is the end of the road.
	 */
	import {
		BIRTH_OUTCOMES,
		birthOutcomeLabel,
		type BirthOutcomeRequest,
		type BirthOutcomeResult
	} from '#lib/engagementDetail.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import {
		EMPTY_DATE_PARTS,
		joinDate,
		splitDate,
		type DateField,
		type DateParts
	} from '#lib/intakeDate.js';
	import { FormSubmission, orServiceProblem, type FormError } from '#lib/formSubmission.svelte.js';
	import { SectionState } from '#lib/sectionState.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import DateFields from '#lib/components/molecules/DateFields.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';

	interface Properties {
		/** The recorded outcome, or undefined for an Engagement that has
		 * none. `undefined` is the whole difference between the two states
		 * this component renders. */
		outcome?: string;
		/** The date the pregnancy ended, `YYYY-MM-DD`. Absent for an
		 * `unknown` outcome, which is never asked for a date. */
		endedOn?: string;
		/** Whether this reader may record an outcome at all -- the app-side
		 * mirror of the BFF's own `refuseFactWrite` (a contractor Doula may
		 * not). Drawing only: the endpoint refuses her whether or not the
		 * control was drawn. */
		canRecord: boolean;
		/** Whether this reader is a Practice Owner, the one role ADR-0015
		 * lets correct or clear a frozen outcome. */
		canCorrect: boolean;
		/** Owns the API call and whatever the page does with the result;
		 * this component owns what was typed and what the answer looked
		 * like on screen. */
		/**
		 * `fieldIds` maps the BFF's own field names onto this component's
		 * controls (#488) -- passed out rather than known by the caller,
		 * because the ids belong to the markup here.
		 */
		onRecord: (
			request: BirthOutcomeRequest,
			fieldIds: Record<string, string>
		) => Promise<BirthOutcomeResult>;
	}

	let { outcome, endedOn, canRecord, canCorrect, onRecord }: Properties = $props();

	const isRecorded = $derived(outcome !== undefined);

	/* The two field-id prefixes, so an error summary entry links to the
	   one control that has to change. RadioGroup names its inputs
	   `${name}-${value}` and DateFields names its boxes `${name}-${field}`. */
	const OUTCOME_NAME = 'birth-outcome';
	const DATE_NAME = 'pregnancy-ended-on';
	/* An error-summary entry links to a control, and a radio group's is
	   its first option -- GOV.UK's own rule, and what the hub's ending
	   reason group already does. */
	const OUTCOME_FIELD_ID = `${OUTCOME_NAME}-${BIRTH_OUTCOMES[0]!.value}`;
	/* Keyed by `engagement.BirthOutcomeRequest`'s json tags (#488). The
	   date's first box is the target, GOV.UK's rule for a group: the
	   group itself is a <fieldset> and is not focusable. */
	const FIELD_IDS = {
		birthOutcome: OUTCOME_FIELD_ID,
		pregnancyEndedOn: `${DATE_NAME}-day`
	};

	let isFormShown = $state(false);
	let choice = $state('');
	let parts = $state<DateParts>({ ...EMPTY_DATE_PARTS });
	const submission = new FormSubmission();

	/* Progressive disclosure, ADR-0015's own rule read as a screen: an
	   `unknown` outcome is the Practice saying it never learned, so a date
	   is not asked for -- absent from the page, never a disabled box. */
	const isDateAsked = $derived(choice !== '' && choice !== 'unknown');

	/* The frozen press-through's own state. `pendingRequest` always holds
	   a request rather than `undefined`, so the confirm handler needs no
	   assertion for a branch that cannot run; the dialog's own `open` is
	   what decides whether it is live. */
	let isFrozenDialogShown = $state(false);
	let frozenMessage = $state('');
	/* `null` here and in handleClear below is the wire value the endpoint
	   takes for "no outcome recorded": `JSON.stringify` drops an
	   `undefined` field, which would send a body asking for nothing rather
	   than a body asking for a clear. */
	// eslint-disable-next-line unicorn/no-null
	let pendingRequest = $state<BirthOutcomeRequest>({ birthOutcome: null });
	let isClearDialogShown = $state(false);

	/* Which of the three boxes a date refusal belongs to, kept beside the
	   summary's own copy rather than parsed back out of a `targetId`.
	   `joinDate` already reports the field; `DateFields` already takes
	   one, and the round trip through a string id in between was the only
	   thing that needed a cast. Cleared at the top of every submit, which
	   is inside `run`'s own reset. */
	let dateRefusal = $state<{ message: string; field: DateField } | undefined>();

	const recordLabel = $derived(isRecorded ? 'Correct what was recorded' : 'Record what happened');

	function openForm() {
		choice = outcome ?? '';
		parts = endedOn === undefined ? { ...EMPTY_DATE_PARTS } : splitDate(endedOn);
		// Both refusals, not just the summary's: `dateRefusal` outlives
		// `submission.errors` otherwise, and a form reopened after a
		// refused date renders the old message against boxes nobody has
		// typed in yet.
		submission.errors = [];
		dateRefusal = undefined;
		clearState.error = '';
		isFormShown = true;
	}

	/**
	 * Sends one request and reads its answer, reporting what is left to
	 * show the reader. The empty array is "nothing to report", which
	 * covers two unrelated answers on purpose: a recorded pair, and the
	 * frozen press-through, which opens a confirmation rather than
	 * refusing anything.
	 */
	async function send(request: BirthOutcomeRequest): Promise<FormError[]> {
		const result = await onRecord(request, FIELD_IDS);
		if (result.kind === 'recorded') {
			isFormShown = false;
			return [];
		}
		if (result.kind === 'confirmable') {
			if (!canCorrect) return [{ message: result.message }];
			frozenMessage = result.message;
			pendingRequest = { ...request, correction: true };
			isFrozenDialogShown = true;
			return [];
		}
		return result.errors;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			dateRefusal = undefined;
			if (choice === '') {
				return [{ message: 'Select what happened to the pregnancy', targetId: OUTCOME_FIELD_ID }];
			}
			let pregnancyEndedOn: string | undefined;
			if (isDateAsked) {
				const date = joinDate(parts, 'The date the pregnancy ended');
				if (!date.ok) {
					dateRefusal = { message: date.message, field: date.field };
					return [{ message: date.message, targetId: `${DATE_NAME}-${date.field}` }];
				}
				if (date.value === '') {
					const message = 'Enter the date the pregnancy ended';
					dateRefusal = { message, field: 'month' };
					return [{ message, targetId: `${DATE_NAME}-month` }];
				}
				pregnancyEndedOn = date.value;
			}
			return send({
				birthOutcome: choice,
				pregnancyEndedOn,
				// A first recording carries no correction flag at all: the
				// BFF refuses one offered where nothing is recorded, and
				// sending it always would make the deliberate act the default.
				...(isRecorded && { correction: true })
			});
		}, orServiceProblem);
	}

	/* Both dialogs go back through the submission they continue, rather
	   than writing `submission.errors` by hand: `run` is also what catches
	   a thrown fetch, so a dropped connection on the pressed-through
	   correction says so instead of rejecting into nothing. */
	async function handleFrozenConfirm() {
		isFrozenDialogShown = false;
		await submission.run(() => send(pendingRequest), orServiceProblem);
	}

	/* The clear is not a refused form -- no field was filled in and there
	   is none to send her back to -- so it reports as a section-local
	   operation outcome, which is what #467 assigns this page's `Notice`
	   and what `SectionState` already holds for every other control on
	   the hub. */
	const clearState = new SectionState<void>(undefined);

	async function handleClear() {
		isClearDialogShown = false;
		await clearState.mutate(async () => {
			// eslint-disable-next-line unicorn/no-null
			const errors = await send({ birthOutcome: null, correction: true });
			if (errors.length > 0) throw new Error(errors.map((entry) => entry.message).join(' '));
		}, 'Failed to remove the recorded outcome');
	}

	function recordedItems(recorded: string, date: string | undefined): { label: string; value: string }[] {
		const items = [{ label: 'What happened', value: birthOutcomeLabel(recorded) }];
		if (date !== undefined) {
			// "Date the pregnancy ended", not "The pregnancy ended": the row
			// above it can read "The pregnancy ended without a living baby",
			// and two rows opening on the same four words read as one
			// sentence broken in half. Seen on the rendered page, not in a test.
			items.push({ label: 'Date the pregnancy ended', value: formatCalendarDay(date) });
		}
		return items;
	}
</script>

<stack-l space="var(--space-4)">
	<!-- A clear is a control's own outcome, not a refused form: it says so
	     where it happened, which is what #467 settled for this page. -->
	{#if clearState.error}
		<Notice variant="error" message={clearState.error} />
	{/if}

	{#if outcome === undefined}
		<Text
			text="Nothing recorded yet. Record this once the Practice knows what happened — it does not wait for the care to end."
		/>
	{:else}
		<DescriptionList items={recordedItems(outcome, endedOn)} />
	{/if}

	<!--
		A contractor Doula is offered nothing here, matching
		api/internal/engagement/transition.go's own refuseFactWrite, and a
		reader who is not an Owner is offered nothing on a recorded pair --
		ADR-0015's role table gives the correction to an Owner alone. Both
		read the recorded value above either way: ADR-0006 puts this fact
		in front of every Staff member, and only the writing is narrowed.
	-->
	{#if !isFormShown && canRecord && (!isRecorded || canCorrect)}
		<cluster-l space="var(--space-3)">
			<Button label={recordLabel} size="sm" variant="secondary" onClick={openForm} />
			{#if isRecorded}
				<Button
					label="Remove this record"
					size="sm"
					variant="secondary"
					onClick={() => (isClearDialogShown = true)}
				/>
			{/if}
		</cluster-l>
	{/if}

	{#if isFormShown}
		<form onsubmit={handleSubmit} novalidate>
			<stack-l space="var(--space-5)">
				<!--
					A refused *form*, so the summary rather than a Notice: the
					same shape the hub's own "Mark care complete" question uses
					a few hundred lines up, and the same reason #467 gives for
					it -- there are fields to send her back to. #467's Notice
					exception on this page is for a control's own outcome,
					which is what the clear above renders as.
				-->
				{#if submission.errors.length > 0}
					<ErrorSummary errors={submission.errors} />
				{/if}
				<RadioGroup
					legend="What happened to the pregnancy?"
					name={OUTCOME_NAME}
					options={BIRTH_OUTCOMES}
					value={choice}
					onChange={(value) => (choice = value)}
					error={submission.errorFor(OUTCOME_FIELD_ID)}
				/>

				{#if isDateAsked}
					<DateFields
						legend="When did the pregnancy end?"
						hint="The day the pregnancy ended, which is often not the day you are recording it. For example, 11 3 2026 for November 3, 2026."
						name={DATE_NAME}
						{parts}
						onChange={(next) => (parts = next)}
						error={dateRefusal?.message}
						invalidField={dateRefusal?.field}
					/>
				{/if}

				<cluster-l space="var(--space-3)">
					<Button
						label={isRecorded ? 'Save the correction' : 'Record this outcome'}
						type="submit"
						size="sm"
						loading={submission.isSubmitting}
					/>
					<Button
						label="Cancel"
						type="button"
						size="sm"
						variant="secondary"
						onClick={() => (isFormShown = false)}
					/>
				</cluster-l>
			</stack-l>
		</form>
	{/if}
</stack-l>

<!-- The BFF's own sentence, not one written here: it is the only thing
     that knows this Engagement was recorded while the page was open. -->
<ConfirmDialog
	bind:open={isFrozenDialogShown}
	title="This Engagement already has a recorded outcome"
	consequence={frozenMessage}
	confirmLabel="Overwrite it with what I entered"
	onConfirm={handleFrozenConfirm}
/>

<ConfirmDialog
	bind:open={isClearDialogShown}
	title="Remove the recorded outcome?"
	consequence="This Engagement goes back to having no outcome recorded at all, and the date the pregnancy ended goes with it. That is not the same as recording that the Practice never learned."
	confirmLabel="Remove this record"
	onConfirm={handleClear}
/>
