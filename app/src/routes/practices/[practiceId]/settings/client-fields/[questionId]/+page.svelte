<script lang="ts">
	/*
	 * One Client field on its own page (#2193), on the Plan Templates'
	 * question form (`PracticeQuestionForm`). `new` is a field not yet saved,
	 * with the kind the kind page chose (`?kind=`).
	 *
	 * What a Client field does that a Plan Template field does not stays
	 * here. Removing one archives it, behind a confirmation, because a
	 * Client's values are read live against this list (GLOSSARY.md) and an
	 * archived field keeps what was recorded. A saved field keeps its kind
	 * of answer, since the Go BFF refuses a type change once values are read
	 * against it. And a question restating a structural fact is refused
	 * here before the boundary refuses it (`structuralFieldRefusal`).
	 *
	 * The template's write replaces the whole list
	 * (`PUT .../client-field-template`), so this page reads the list as it
	 * stands, puts this one field into it, and sends it back; the BFF records
	 * each real change, before and after, in the Practice's activity. Its
	 * save lands on the list, which says what changed (#2194's words, #1710).
	 * Write is Owner or Admin; anyone else reads who may change it.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import {
		FIELD_TYPES,
		archiveField,
		isSelectType,
		loadTemplate,
		saveTemplate,
		structuralFieldRefusal,
		type Field,
		type FieldType
	} from '#lib/clientFieldTemplate.js';
	import {
		activeQuestionCount,
		optionsFromText,
		placeQuestion,
		questionErrors,
		questionPosition,
		withQuestion
	} from '#lib/practiceQuestions.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import PracticeQuestionForm, {
		practiceQuestionIds
	} from '#lib/components/organisms/PracticeQuestionForm.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { clientFieldsArchivedMessage, clientFieldsHref, clientFieldsSavedMessage } from '../clientFieldPages.js';
	import type { PracticeSession } from '../../../+layout.js';

	const NAME = 'client-question';
	const ids = practiceQuestionIds(NAME);

	const session = $derived((page.data as { session: PracticeSession }).session);
	const canChange = $derived(isOwnerOrAdmin(session));
	const practiceId = $derived(page.params.practiceId!);
	const questionId = $derived(page.params.questionId!);
	const isNew = $derived(questionId === 'new');
	const listHref = $derived(clientFieldsHref(practiceId));
	const title = $derived(isNew ? 'Add a question' : 'Change a question');

	let fields = $state<Field[]>([]);
	let id = $state('');
	let type = $state<FieldType>('short_text');
	let label = $state('');
	let optionsText = $state('');
	let position = $state<number | undefined>();
	let isLoaded = $state(false);
	let loadError = $state('');
	let isConfirmingArchive = $state(false);
	const submission = new FormSubmission();

	onMount(async () => {
		try {
			const template = await loadTemplate(apiFetchWithSession, practiceId);
			fields = template.fields;
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load Client Field Template';
			return;
		}
		if (isNew) {
			const named = page.url.searchParams.get('kind');
			const kind = FIELD_TYPES.find((candidate) => candidate === named);
			if (!kind) {
				loadError = 'Choose the kind of answer first, from Add a question.';
				return;
			}
			id = crypto.randomUUID();
			type = kind;
		} else {
			// An archived field is collected again from the list, not changed.
			const question = fields.find((field) => field.id === questionId && !field.archived);
			if (!question) {
				loadError = 'This question is not on the Client Fields any more.';
				return;
			}
			({ id, type, label } = question);
			optionsText = (question.options ?? []).join('\n');
			position = questionPosition(fields, id);
		}
		isLoaded = true;
	});

	async function sendAndReturn(next: Field[], message = clientFieldsSavedMessage) {
		await saveTemplate(apiFetchWithSession, practiceId, next);
		await gotoWithOutcome(listHref, message);
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			const options = optionsFromText(optionsText);
			const errors = questionErrors({ type, label, options }, ids, structuralFieldRefusal);
			if (errors.length > 0) {
				return errors;
			}
			const question: Field = { id, type, label: label.trim(), order: 0, archived: false };
			if (isSelectType(type)) {
				question.options = options;
			}
			const placed = withQuestion(fields, question);
			await sendAndReturn(position === undefined ? placed : placeQuestion(placed, id, position));
		}, orThrownMessage);
	}

	async function archive() {
		isConfirmingArchive = false;
		const archived = clientFieldsArchivedMessage(label || 'Untitled question');
		await submission.run(() => sendAndReturn(archiveField(fields, id), archived), orThrownMessage);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet readOnly()}
	<Notice variant="status" message="Only a Practice Owner or Admin can change the Client Fields." />
{/snippet}

{#snippet noActions()}{/snippet}

{#if !canChange}
	<FormPage {title} fieldsets={[{ content: readOnly }]} actions={noActions} />
{:else if loadError || !isLoaded}
	<FormPage
		{title}
		fieldsets={[]}
		actions={noActions}
		loading={loadError ? undefined : 'Loading the question'}
		loadError={loadError || undefined}
	/>
{:else}
	<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
	<form onsubmit={save} novalidate>
		<QuestionPage
			journey={isNew ? 'Adding a question' : 'Changing a question'}
			steps={[]}
			backHref={listHref}
			caption="Client Fields"
			question={{ as: 'legend', text: title }}
			errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		>
			{#snippet content()}
				<PracticeQuestionForm
					name={NAME}
					{type}
					{label}
					{optionsText}
					{position}
					positionCount={activeQuestionCount(fields)}
					isKindLocked={!isNew}
					errorFor={(target) => submission.errorFor(target)}
					onLabelInput={(value) => (label = value)}
					onTypeChange={(value) => (type = value)}
					onOptionsInput={(value) => (optionsText = value)}
					onPositionChange={(value) => (position = value)}
				/>
			{/snippet}
			{#snippet actions()}
				<Button type="submit" label="Save question" loading={submission.isSubmitting} />
				{#if !isNew}
					<Button label="Archive question" variant="link" onClick={() => (isConfirmingArchive = true)} />
				{/if}
			{/snippet}
		</QuestionPage>
	</form>

	<ConfirmDialog
		bind:open={isConfirmingArchive}
		title={`Archive "${label || 'Untitled question'}"?`}
		consequence="A Client who already answered this keeps that answer, read-only on the Client's record. New Clients will not be asked this question, and you can collect it again later from the archived questions."
		confirmLabel="Archive question"
		onConfirm={archive}
	/>
{/if}
