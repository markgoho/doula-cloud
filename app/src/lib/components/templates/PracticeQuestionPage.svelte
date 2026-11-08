<script module lang="ts">
	import type { FieldType } from '#lib/planTemplate.js';

	export interface Listed {
		id: string;
		type: FieldType;
		label: string;
		options?: string[];
		order: number;
		archived?: boolean;
	}

	export interface Draft {
		id: string;
		type: FieldType;
		label: string;
		options?: string[];
	}
</script>

<script lang="ts" generics="F extends Listed">
	import { onMount } from 'svelte';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import { FIELD_TYPES, isSelectType } from '#lib/planTemplate.js';
	import {
		activeQuestionCount,
		optionsFromText,
		placeQuestion,
		questionErrors,
		questionPosition,
		withQuestion
	} from '#lib/practiceQuestions.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import PracticeQuestionForm, {
		practiceQuestionIds
	} from '#lib/components/organisms/PracticeQuestionForm.svelte';
	import FormPage from './FormPage.svelte';
	import QuestionPage from './QuestionPage.svelte';

	interface Properties {
		/**
		The stem of every control's id (`practiceQuestionIds`).
		*/
		name: string;
		/**
		Names the list, above the question: "Client Fields", "Doula Plan".
		*/
		caption: string;
		listHref: string;
		/**
		The question's id, or `new` for one not yet saved.
		*/
		questionId: string;
		/**
		The kind of answer a new question was started with (`?kind=`).
		*/
		kind: string | undefined;
		/**
		The list as it stands. A rejection carries the reason the page shows.
		*/
		load: () => Promise<F[]>;
		loadFailure: string;
		notOnList: string;
		/**
		A question the list holds but this page does not change.
		*/
		isEditable?: (question: F) => boolean;
		/**
		A saved question keeps its kind of answer.
		*/
		locksKind?: boolean;
		refuseLabel?: (label: string) => string | undefined;
		toField: (draft: Draft) => F;
		/**
		Sends the whole list back and leaves the page.
		*/
		onSave: (fields: F[]) => Promise<void>;
		retire: {
			verb: string;
			consequence: string;
			apply: (fields: F[], id: string) => F[];
			onDone: (fields: F[], label: string) => Promise<void>;
		};
	}

	let {
		name,
		caption,
		listHref,
		questionId,
		kind,
		load,
		loadFailure,
		notOnList,
		isEditable = () => true,
		locksKind = false,
		refuseLabel,
		toField,
		onSave,
		retire
	}: Properties = $props();

	const ids = $derived(practiceQuestionIds(name));
	const isNew = $derived(questionId === 'new');
	const title = $derived(isNew ? 'Add a question' : 'Change a question');
	let fields = $state.raw<F[]>([]);
	let id = $state('');
	let type = $state<FieldType>('short_text');
	let label = $state('');
	let optionsText = $state('');
	let position = $state<number | undefined>();
	let isLoaded = $state(false);
	let loadError = $state('');
	let isConfirmingRetire = $state(false);
	const submission = new FormSubmission();
	const shownLabel = $derived(label || 'Untitled question');

	onMount(async () => {
		try {
			fields = await load();
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : loadFailure;
			return;
		}
		if (isNew) {
			const started = FIELD_TYPES.find((candidate) => candidate === kind);
			if (!started) {
				loadError = 'Choose the kind of answer first, from Add a question.';
				return;
			}
			id = crypto.randomUUID();
			type = started;
		} else {
			const question = fields.find((field) => field.id === questionId && isEditable(field));
			if (!question) {
				loadError = notOnList;
				return;
			}
			({ id, type, label } = question);
			optionsText = (question.options ?? []).join('\n');
			position = questionPosition(fields, id);
		}
		isLoaded = true;
	});

	async function save(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			const options = optionsFromText(optionsText);
			const errors = questionErrors({ type, label, options }, ids, refuseLabel);
			if (errors.length > 0) {
				return errors;
			}
			const draft: Draft = { id, type, label: label.trim() };
			if (isSelectType(type)) {
				draft.options = options;
			}
			const placed = withQuestion(fields, toField(draft));
			await onSave(position === undefined ? placed : placeQuestion(placed, id, position));
		}, orThrownMessage);
	}

	async function retireQuestion() {
		isConfirmingRetire = false;
		await submission.run(() => retire.onDone(retire.apply(fields, id), shownLabel), orThrownMessage);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#if loadError || !isLoaded}
	<FormPage
		{title}
		fieldsets={[]}
		loading={loadError ? undefined : 'Loading the question'}
		loadError={loadError || undefined}
	/>
{:else}
	<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
	<form onsubmit={save} novalidate>
		<QuestionPage
			journey="Adding or changing a question"
			steps={[]}
			backHref={listHref}
			{caption}
			question={{ as: 'legend', text: title }}
			errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		>
			{#snippet content()}
				<PracticeQuestionForm
					{name}
					{type}
					{label}
					{optionsText}
					{position}
					positionCount={activeQuestionCount(fields)}
					isKindLocked={locksKind && !isNew}
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
					<Button label={`${retire.verb} question`} variant="link" onClick={() => (isConfirmingRetire = true)} />
				{/if}
			{/snippet}
		</QuestionPage>
	</form>

	<ConfirmDialog
		bind:open={isConfirmingRetire}
		title={`${retire.verb} "${shownLabel}"?`}
		consequence={retire.consequence}
		confirmLabel={`${retire.verb} question`}
		onConfirm={retireQuestion}
	/>
{/if}
