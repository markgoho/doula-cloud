<script lang="ts">
	/*
	 * The end of her details journey: what will be saved, with a way back
	 * to each question, and the one save (#1610, GOV.UK's Check answers
	 * pattern).
	 *
	 * ## What the rows show
	 *
	 * What the save will send, not what was typed: a blank never replaces
	 * a value on file (`detailsToSave`), so a question cleared on its page
	 * still shows the value her record keeps. A row that showed "Not
	 * answered" there would say the save takes away what it keeps.
	 *
	 * ## How it saves
	 *
	 * One edit, through the edit path's full-record `PUT` (`edit.go`), so
	 * ADR-0017's amendment's two gates run on it as on every edit, and the
	 * one `updated` entry on her history says who saved it and when.
	 *
	 * - Gate one, a name substitution (`substitution: true`). The journey
	 *   never asks the name, so this is reached only when her name changed
	 *   on file while the journey was open. It is answered here, by the
	 *   app's one confirmation mechanism (#473), the same as the Edit
	 *   form: "Yes, a different person" retries with `override: true`.
	 *   There is no control on this page for a refusal to point at, so
	 *   every refusal of the retry is the dialog's own.
	 * - Gate two, a possible duplicate (`substitution: false`). The edit
	 *   path's own duplicate page asks it, with Back to this page.
	 */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { editClient, type ClientEditFields, type CollisionMatch } from '#lib/client.js';
	import { displayName } from '#lib/clientDetail.js';
	import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
	import { detailsToSave } from '#lib/clientDetailsJourney.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import CheckAnswers from '#lib/components/templates/CheckAnswers.svelte';
	import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';
	import { errorsFromCause } from '#lib/formErrors.js';
	import { FormSubmission, orThrownErrors } from '#lib/formSubmission.svelte.js';
	import { answerSections } from '#lib/intakeAnswers.js';
	import { DETAILS_STEPS, detailsSavedMessage, journeySteps } from '#lib/intakeJourney.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import { detailsQuestions, knownAs } from '../details.js';

	/*
	 * GOV.UK's wider column for a long answer list, from the same count
	 * intake's own check page uses.
	 */
	const WIDE_FROM_ROWS = 14;

	const practiceId = $derived(page.params.practiceId ?? '');
	const clientId = $derived(page.params.clientId ?? '');
	const base = $derived(detailsQuestions.basePath);
	const steps = $derived(
		journeySteps(detailsQuestions.steps, base, undefined, clientDetails.draft.visitedSteps)
	);
	const askedFieldIds = $derived(
		detailsQuestions.sections.flatMap((section) => section.fields.map((field) => field.id))
	);
	// The layout renders no step until the record is read, so it is here.
	const toSave = $derived(detailsToSave(clientDetails.record!, clientDetails.draft.answers, askedFieldIds));
	const sections = $derived(answerSections(toSave, detailsQuestions.sections, base, DETAILS_STEPS));
	const rowCount = $derived(sections.reduce((total, section) => total + section.answers.length, 0));
	const lastStep = $derived(detailsQuestions.steps.at(-1)!);

	const submission = new FormSubmission();
	let matches = $state<CollisionMatch[]>([]);
	let isConflictOpen = $state(false);
	let overrideError = $state('');

	// The screen the journey was opened from says the save happened
	// (#1710): this page is gone once the save leaves it.
	async function finish() {
		const href = detailsQuestions.exitHref;
		const message = detailsSavedMessage(clientDetails.record!);
		clientDetails.draft.clear();
		await gotoWithOutcome(href, message);
	}

	async function handleSave() {
		await submission.run(async () => {
			const fields: ClientEditFields = toSave;
			const result = await editClient(apiFetchWithSession, practiceId, clientId, fields, false);
			if (!result.conflict) {
				await finish();
				return;
			}
			if (result.substitution) {
				matches = result.matches;
				isConflictOpen = true;
				return;
			}
			editMergeDraft.open(clientId, fields, result.matches, `${base}/check`);
			await goto(
				`${resolve('/practices/[practiceId]/clients/[clientId]/edit', { practiceId, clientId })}/duplicate`
			);
		}, orThrownErrors({}));
	}

	// The single deliberate override, reached only by the dialog's named
	// button. A rejection keeps the dialog open over its own message
	// (ConfirmDialog's contract, #804).
	async function handleOverrideConfirm() {
		overrideError = '';
		try {
			const result = await editClient(apiFetchWithSession, practiceId, clientId, toSave, true);
			// override: true skips the match query entirely (edit.go), so a
			// conflict here means something else refused the write.
			if (result.conflict) throw new Error('The Client record could not be saved.');
		} catch (error) {
			overrideError = errorsFromCause(error, {})
				.map((entry) => entry.message)
				.join(' ');
			throw new Error(overrideError, { cause: error });
		}
		await finish();
	}

	function handleConflictCancel() {
		matches = [];
		overrideError = '';
	}
</script>

<!-- Passed only while there is a refusal, never declared as a child of
     the Template: a declared snippet is always truthy, so the Template
     would title the page "Error: " before any save (#1705). -->
{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

<CheckAnswers
	journey={detailsQuestions.label}
	{steps}
	backHref={`${base}/${lastStep.slug}`}
	title="Check {knownAs()}'s details before saving"
	{sections}
	isWide={rowCount >= WIDE_FROM_ROWS}
	errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
>
	{#snippet actions()}
		<Button label="Save these details" loading={submission.isSubmitting} onClick={handleSave} />
	{/snippet}
</CheckAnswers>

<ConfirmDialog
	bind:open={isConflictOpen}
	title="Possible duplicate Client"
	consequence={`This name exactly matches an existing Client at this Practice: ${matches.map((match) => displayName(match)).join(', ')}. Saving keeps this as its own separate record -- nothing here is merged.`}
	confirmLabel="Yes, a different person"
	error={overrideError}
	onConfirm={handleOverrideConfirm}
	onCancel={handleConflictCancel}
/>
