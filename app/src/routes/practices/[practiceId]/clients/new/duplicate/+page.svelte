<script lang="ts">
	/*
	 * The save-time duplicate check, as a page (#466, ADR-0017).
	 *
	 * `create.go` answers 409 with the matches when `FindMatches` hits
	 * and `override` is false. #432 drew that as a page rather than a
	 * dialog, and it is a question page like any other: one question, a
	 * radio group, a Continue. `RadioGroup` gets no legend -- the
	 * Template already owns the <fieldset> and its <legend> is the <h1>
	 * -- and each option's `description` is what tells two Sarahs apart.
	 *
	 * Two answers, and both are real:
	 *
	 * - **A match.** Nothing is created. What intake typed is proposed as
	 *   changes to the Client already on file, shown before anything is
	 *   written (`?match=` is that second state, so it is a real URL and
	 *   the browser's Back works on it), then saved as the full-object
	 *   PUT `edit.go` expects.
	 * - **A different person.** The one deliberate override on this path.
	 *   It re-sends the create with `override: true`, which skips the
	 *   match query entirely, and opens the Start work form (#1611).
	 *
	 * A match ends on her record, because she is a returning Client and
	 * new work with her starts there (#1611).
	 *
	 * Nothing has been saved when this page appears, which is why the
	 * first line says so.
	 */
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { editClient, type ClientMatch } from '#lib/client.js';
	import { fullName } from '#lib/personName.js';
	import { matchHistory } from '#lib/clientMatch.js';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { detailsSavedMessage } from '#lib/intakeJourney.js';
	import { mergedEditFields } from '#lib/intakeMerge.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import { proposedChanges } from '#lib/proposedChanges.js';
	import { JOURNEY, basePath, detailHref, intakeSteps, saveIntake } from '../intake.js';

	const DIFFERENT_PERSON = 'different';
	// Named rather than generated, so the error summary can point an
	// entry at the first radio by id.
	const ANSWER_NAME = 'intake-same-person';

	const practiceId = $derived(page.params.practiceId ?? '');
	const base = $derived(basePath(practiceId));
	const steps = $derived(intakeSteps(practiceId));

	const chosenId = $derived(page.url.searchParams.get('match') ?? '');
	const reviewing = $derived<ClientMatch | undefined>(
		intakeDraft.matches.find((match) => match.id === chosenId)
	);
	// The match always survives here; what was typed is absorbed into it.
	const changes = $derived(reviewing ? proposedChanges(reviewing, intakeDraft.answers) : []);

	let answer = $state('');
	const submission = new FormSubmission();
	/*
	 * The refusal that belongs to the radio group, separate from
	 * `submission.errors`. GOV.UK asks for a field's message twice -- in
	 * the summary and again against the control -- but a whole-submission
	 * refusal (a 5xx, a dropped connection) belongs only in the summary,
	 * because there is no control it is about. One list could not tell
	 * them apart.
	 */
	let answerError = $state<string | undefined>();

	/*
	 * A reader who reloads this page, or reaches it from a bookmark, has
	 * no matches in the draft: they live in memory and the 409 that
	 * produced them is not repeated by a page load. The name question is
	 * where the save was asked for, so that is where they go.
	 *
	 * `onMount`, deliberately, not an `$effect`. Every way OFF this page
	 * clears the draft, which empties `matches` -- so a reactive check
	 * would fire on the way out and race the navigation it was reacting
	 * to, sending a reader who had just saved to the summary instead of
	 * to the Client. The question this asks is only ever about how the
	 * page was ARRIVED at.
	 */
	onMount(() => {
		if (intakeDraft.matches.length === 0) {
			void goto(`${base}/name`);
		}
	});

	/*
	 * #2143: each existing record says, under its name, the line Find a
	 * Client prints for it (`matchHistory`: when it was added and when the
	 * Client was last cared for, which is what tells two people with one
	 * name apart), and then what choosing it does. That second line used
	 * to be one paragraph under all the options; said under each, it is
	 * read at the moment of choosing. It holds on both branches below:
	 * with something to change, the changes are shown before anything is
	 * written; with nothing, the record opens as it is.
	 */
	const options = $derived([
		...intakeDraft.matches.map((match) => ({
			value: match.id,
			label: fullName(match),
			description: [
				matchHistory(match),
				'Keeps this record. You check any changes before they are saved.'
			]
		})),
		{
			value: DIFFERENT_PERSON,
			label: 'No, this is a different person',
			description:
				'A second record is created under the same name. This is the only way to make one.'
		}
	]);

	async function handleContinue(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			if (answer === '') {
				// Linked to the first option, which is where GOV.UK's error
				// summary sends a reader whose refusal belongs to a group: the
				// first control in it, not the group itself. The same words
				// appear against the group through `answerError`.
				answerError = 'Choose whether this is the same person';
				return [{ message: answerError, targetId: `${ANSWER_NAME}-${options[0]!.value}` }];
			}
			answerError = undefined;
			if (answer === DIFFERENT_PERSON) {
				return await saveIntake(practiceId, true);
			}
			const match = intakeDraft.matches.find((entry) => entry.id === answer);
			// Nothing typed differs from what is on file, so there is nothing
			// to confirm and nothing to write. Straight to the record.
			if (match && proposedChanges(match, intakeDraft.answers).length === 0) {
				intakeDraft.clear();
				await goto(detailHref(practiceId, answer));
				return;
			}
			await goto(`${base}/duplicate?match=${encodeURIComponent(answer)}`);
		}, orThrownMessage);
	}

	async function handleSaveChanges() {
		if (!reviewing) return;
		await submission.run(async () => {
			const fields = mergedEditFields(intakeDraft.answers, reviewing);
			const result = await editClient(apiFetchWithSession, practiceId, reviewing.id, fields, true);
			// `override` is set, so `edit.go` runs no match query and a
			// conflict here would mean something else refused the write.
			if (result.conflict) {
				return [{ message: 'The Client record could not be saved.' }];
			}
			const clientId = reviewing.id;
			intakeDraft.clear();
			// #1710: the save leaves this page, so her record says it
			// happened.
			await gotoWithOutcome(detailHref(practiceId, clientId), detailsSavedMessage(fields));
		}, orThrownMessage);
	}
</script>

<!-- Passed only while there is a refusal, never declared as a child of
     the Template: a declared snippet is always truthy, so the Template
     would title a first visit "Error: " (#1705). Both questions below
     read the one submission, so they share it. -->
{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#if reviewing}
	<QuestionPage
		journey={JOURNEY}
		{steps}
		backHref="{base}/duplicate"
		question={{ as: 'legend', text: `Save these changes to ${fullName(reviewing)}?` }}
		hint="No new record is created. The record already on file is kept and updated."
		errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
	>
		{#snippet content()}
			<SummaryList
				rows={changes.map((change) => ({
					label: change.label,
					value: `${change.before} → ${change.after}`
				}))}
			/>
		{/snippet}

		{#snippet actions()}
			<Button
				label="Save changes to this record"
				loading={submission.isSubmitting}
				onClick={handleSaveChanges}
			/>
		{/snippet}
	</QuestionPage>
{:else if intakeDraft.matches.length > 0}
	<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
	<form onsubmit={handleContinue} novalidate>
		<QuestionPage
			journey={JOURNEY}
			{steps}
			backHref="{base}/name"
			question={{ as: 'legend', text: 'Is this the same person?' }}
			hint="Nothing has been saved yet. What was typed matches a Client this Practice already has."
			errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		>
			{#snippet content()}
				<RadioGroup
					name={ANSWER_NAME}
					{options}
					value={answer}
					onChange={(chosen) => (answer = chosen)}
					error={answerError}
				/>
			{/snippet}

			{#snippet actions()}
				<Button type="submit" label="Continue" loading={submission.isSubmitting} />
			{/snippet}
		</QuestionPage>
	</form>
{/if}
