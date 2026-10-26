<script lang="ts">
	/*
	 * The end of her details journey: what will be saved, with a way back
	 * to each question, and the one save (#1610, GOV.UK's Check answers
	 * pattern).
	 *
	 * ## What the rows show
	 *
	 * What the save will send (`detailsToSave`): her record, with every
	 * answer laid over it. A question cleared on its page clears the value
	 * on file since #2048, so its row says "Not answered".
	 *
	 * ## How it saves
	 *
	 * Through `detailsSave`, the journey's one save, which a Change on her
	 * record also uses: the full-record `PUT` and ADR-0017's amendment's
	 * two gates. Gate one's dialog is the layout's; gate two's question is
	 * the journey's `duplicate` page, with Back to this page.
	 */
	import { page } from '#lib/appState.svelte.js';
	import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
	import { detailsToSave } from '#lib/clientDetailsJourney.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import CheckAnswers from '#lib/components/templates/CheckAnswers.svelte';
	import { FormSubmission, orThrownErrors } from '#lib/formSubmission.svelte.js';
	import { answerSections } from '#lib/intakeAnswers.js';
	import { DETAILS_STEPS, journeySteps } from '#lib/intakeJourney.js';
	import { detailsQuestions, knownAs } from '../details.js';

	/*
	 * GOV.UK's wider column for a long answer list, from the same count
	 * intake's own check page uses.
	 */
	const WIDE_FROM_ROWS = 14;

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

	async function handleSave() {
		await submission.run(() => detailsQuestions.save(page.url.pathname), orThrownErrors({}));
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
