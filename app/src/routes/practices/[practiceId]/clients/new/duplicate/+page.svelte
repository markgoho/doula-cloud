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
	 *   match query entirely.
	 *
	 * Nothing has been saved when this page appears, which is why the
	 * first line says so.
	 */
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { editClient, type ClientMatch } from '#lib/client.js';
	import { displayName } from '#lib/clientDetail.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import { SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';
	import { intakeDraft } from '#lib/intakeDraft.svelte.js';
	import { intakeFlow } from '#lib/intakeFlow.svelte.js';
	import { journeySteps } from '#lib/intakeJourney.js';
	import { mergedEditFields, proposedChanges } from '#lib/intakeMerge.js';
	import { JOURNEY, basePath, detailHref, knownAs, saveIntake } from '../intake.js';

	const DIFFERENT_PERSON = 'different';
	// Named rather than generated, so the error summary can point an
	// entry at the first radio by id.
	const ANSWER_NAME = 'intake-same-person';

	const practiceId = $derived(page.params.practiceId ?? '');
	const base = $derived(basePath(practiceId));
	const steps = $derived(journeySteps(intakeFlow.steps, base, undefined, intakeDraft.visitedSteps));

	const chosenId = $derived(page.url.searchParams.get('match') ?? '');
	const reviewing = $derived<ClientMatch | undefined>(
		intakeDraft.matches.find((match) => match.id === chosenId)
	);
	const changes = $derived(reviewing ? proposedChanges(intakeDraft.answers, reviewing) : []);

	let answer = $state('');
	let errors = $state<FormError[]>([]);
	/*
	 * The refusal that belongs to the radio group, separate from
	 * `errors`. GOV.UK asks for a field's message twice -- in the summary
	 * and again against the control -- but a whole-submission refusal (a
	 * 5xx, a dropped connection) belongs only in the summary, because
	 * there is no control it is about. One list could not tell them
	 * apart.
	 */
	let answerError = $state<string | undefined>();
	let isSaving = $state(false);

	/*
	 * A reader who reloads this page, or reaches it from a bookmark, has
	 * no matches in the draft: they live in memory and the 409 that
	 * produced them is not repeated by a page load. The summary is where
	 * the sequence was, so that is where they go.
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
			void goto(`${base}/check`);
		}
	});

	function matchDescription(match: ClientMatch): string {
		const facts = [
			// In words, through `dates.ts`, like every other date the app
			// shows: this line's whole job is telling two people with the
			// same name apart, and "1988-02-09" is storage rather than a
			// fact a reader recognises.
			match.dateOfBirth ? `Born ${formatCalendarDay(match.dateOfBirth)}` : 'No date of birth on file',
			match.email || match.phone || 'No contact details on file',
			match.engagements.length === 1
				? '1 Engagement'
				: `${match.engagements.length} Engagements`
		];
		return facts.join(' · ');
	}

	const options = $derived([
		...intakeDraft.matches.map((match) => ({
			value: match.id,
			label: displayName(match),
			description: matchDescription(match)
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
		if (answer === '') {
			// Linked to the first option, which is where GOV.UK's error
			// summary sends a reader whose refusal belongs to a group: the
			// first control in it, not the group itself. The same words
			// appear against the group through `answerError`.
			answerError = 'Choose whether this is the same person';
			errors = [{ message: answerError, targetId: `${ANSWER_NAME}-${options[0]!.value}` }];
			return;
		}
		answerError = undefined;
		errors = [];
		if (answer === DIFFERENT_PERSON) {
			isSaving = true;
			errors = (await saveIntake(practiceId, true)) ?? [];
			isSaving = false;
			return;
		}
		const match = intakeDraft.matches.find((entry) => entry.id === answer);
		// Nothing typed differs from what is on file, so there is nothing
		// to confirm and nothing to write. Straight to the record.
		if (match && proposedChanges(intakeDraft.answers, match).length === 0) {
			intakeDraft.clear();
			await goto(detailHref(practiceId, answer));
			return;
		}
		await goto(`${base}/duplicate?match=${encodeURIComponent(answer)}`);
	}

	async function handleSaveChanges() {
		if (!reviewing) return;
		errors = [];
		isSaving = true;
		try {
			const result = await editClient(
				apiFetchWithSession,
				practiceId,
				reviewing.id,
				mergedEditFields(intakeDraft.answers, reviewing),
				true
			);
			// `override` is set, so `edit.go` runs no match query and a
			// conflict here would mean something else refused the write.
			if (result.conflict) {
				errors = [{ message: 'The Client record could not be saved.' }];
				return;
			}
			const clientId = reviewing.id;
			intakeDraft.clear();
			await goto(detailHref(practiceId, clientId));
		} catch (error) {
			errors = [
				{ message: error instanceof Error && error.message ? error.message : SERVICE_PROBLEM }
			];
		} finally {
			isSaving = false;
		}
	}
</script>

{#if reviewing}
	<QuestionPage
		journey={JOURNEY}
		{steps}
		backHref="{base}/duplicate"
		question={{ as: 'legend', text: `Save these changes to ${displayName(reviewing)}?` }}
		hint="No new record is created. The record already on file is kept and updated."
	>
		{#snippet errorSummary()}
			{#if errors.length > 0}
				<ErrorSummary {errors} />
			{/if}
		{/snippet}

		{#snippet content()}
			<DescriptionList
				items={changes.map((change) => ({
					label: change.label,
					value: `${change.onFile} → ${change.typed}`
				}))}
			/>
		{/snippet}

		{#snippet actions()}
			<Button
				label="Save changes to this record"
				loading={isSaving}
				onClick={handleSaveChanges}
			/>
		{/snippet}
	</QuestionPage>
{:else if intakeDraft.matches.length > 0}
	<form onsubmit={handleContinue} novalidate>
		<QuestionPage
			journey={JOURNEY}
			{steps}
			backHref="{base}/check"
			question={{ as: 'legend', text: 'Is this the same person?' }}
			hint="Nothing has been saved yet. What was typed matches a Client this Practice already has."
		>
			{#snippet errorSummary()}
				{#if errors.length > 0}
					<ErrorSummary {errors} />
				{/if}
			{/snippet}

			{#snippet content()}
				<stack-l space="var(--space-5)">
					<RadioGroup
						name={ANSWER_NAME}
						{options}
						value={answer}
						onChange={(chosen) => (answer = chosen)}
						error={answerError}
					/>
					<Text
						step="body-sm"
						tone="muted"
						text="Records are never merged here. Choosing an existing Client updates that record with what was typed for {knownAs()}."
					/>
				</stack-l>
			{/snippet}

			{#snippet actions()}
				<Button type="submit" label="Continue" loading={isSaving} />
			{/snippet}
		</QuestionPage>
	</form>
{/if}
