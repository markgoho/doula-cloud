<script lang="ts">
	/*
	 * One Plan Template question on its own page (#2193): its words, its
	 * kind of answer, its answers to choose from, and its place in the list.
	 * `new` is a question not yet saved, with the kind the kind page chose
	 * (`?kind=`).
	 *
	 * Each page saves on its own, as GOV.UK's "Add another" does: the
	 * template's write replaces the whole list
	 * (`PUT .../plan-templates/{planType}`), so this page reads the list as
	 * it stands, puts this one question into it, and sends the list back.
	 * Its save, and its Remove, land on the list, which says what changed
	 * (#2194's words, before the <h1>, #1710). ADR-0001 holds as before: a
	 * plan already filled in keeps the questions it was filled in against.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import {
		FIELD_TYPES,
		isSelectType,
		loadTemplate,
		removeField,
		saveTemplate,
		type Field,
		type FieldType
	} from '#lib/planTemplate.js';
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
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import {
		isPlanType,
		planLabel,
		planTemplateSavedMessage,
		planTemplatesHref,
		type PlanType
	} from '../../planTemplatePages.js';

	const NAME = 'plan-question';
	const ids = practiceQuestionIds(NAME);

	const practiceId = $derived(page.params.practiceId!);
	const questionId = $derived(page.params.questionId!);
	const isNew = $derived(questionId === 'new');

	let fields = $state<Field[]>([]);
	let id = $state('');
	let type = $state<FieldType>('short_text');
	let label = $state('');
	let optionsText = $state('');
	let position = $state<number | undefined>();
	let isLoaded = $state(false);
	let loadError = $state('');
	let isConfirmingRemove = $state(false);
	const submission = new FormSubmission();

	function kindFromQuery(): FieldType | undefined {
		const named = page.url.searchParams.get('kind');
		return FIELD_TYPES.find((kind) => kind === named);
	}

	onMount(async () => {
		const planType = page.params.planType;
		if (!isPlanType(planType)) {
			loadError = 'There is no plan of that kind.';
			return;
		}
		try {
			const template = await loadTemplate(apiFetchWithSession, practiceId, planType);
			fields = template.fields;
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load plan template';
			return;
		}
		if (isNew) {
			const kind = kindFromQuery();
			if (!kind) {
				loadError = 'Choose the kind of answer first, from Add a question.';
				return;
			}
			id = crypto.randomUUID();
			type = kind;
		} else {
			const question = fields.find((field) => field.id === questionId);
			if (!question) {
				loadError = `This question is not on the ${planLabel(planType)} any more.`;
				return;
			}
			({ id, type, label } = question);
			optionsText = (question.options ?? []).join('\n');
			position = questionPosition(fields, id);
		}
		isLoaded = true;
	});

	const planType = $derived(page.params.planType as PlanType);
	const listHref = $derived(planTemplatesHref(practiceId, planType));

	async function sendAndReturn(next: Field[]) {
		await saveTemplate(apiFetchWithSession, practiceId, planType, next);
		await gotoWithOutcome(listHref, planTemplateSavedMessage(planType));
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			const options = optionsFromText(optionsText);
			const errors = questionErrors({ type, label, options }, ids);
			if (errors.length > 0) {
				return errors;
			}
			const question: Field = { id, type, label: label.trim(), order: 0 };
			if (isSelectType(type)) {
				question.options = options;
			}
			const placed = withQuestion(fields, question);
			await sendAndReturn(position === undefined ? placed : placeQuestion(placed, id, position));
		}, orThrownMessage);
	}

	async function remove() {
		isConfirmingRemove = false;
		await submission.run(() => sendAndReturn(removeField(fields, id)), orThrownMessage);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet noActions()}{/snippet}

{#if loadError || !isLoaded}
	<FormPage
		title={isNew ? 'Add a question' : 'Change a question'}
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
			caption={planLabel(planType)}
			question={{ as: 'legend', text: isNew ? 'Add a question' : 'Change a question' }}
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
					<Button label="Remove question" variant="link" onClick={() => (isConfirmingRemove = true)} />
				{/if}
			{/snippet}
		</QuestionPage>
	</form>

	<ConfirmDialog
		bind:open={isConfirmingRemove}
		title={`Remove "${label || 'Untitled question'}"?`}
		consequence={`Every ${planLabel(planType)} filled in from now on will not ask it. A ${planLabel(planType)} already filled in keeps it, with its answer.`}
		confirmLabel="Remove question"
		onConfirm={remove}
	/>
{/if}
