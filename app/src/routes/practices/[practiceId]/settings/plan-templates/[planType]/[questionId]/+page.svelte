<script lang="ts">
	/*
	 * One Plan Template question on its own page (#2193): its words, its
	 * kind of answer, its answers to choose from, and its place in the list
	 * (`PracticeQuestion`). `new` is a question not yet saved, with the
	 * kind the kind page chose (`?kind=`).
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
	import { loadTemplate, removeField, saveTemplate, type Field } from '#lib/planTemplate.js';
	import { PracticeQuestion } from '#lib/practiceQuestion.svelte.js';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import QuestionActions from '#lib/components/organisms/QuestionActions.svelte';
	import QuestionFields from '#lib/components/organisms/QuestionFields.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import {
		isPlanType,
		planLabel,
		planTemplateRemovedMessage,
		planTemplateSavedMessage,
		planTemplatesHref,
		type PlanType
	} from '../../planTemplatePages.js';

	const practiceId = $derived(page.params.practiceId!);
	const planType = $derived(page.params.planType as PlanType);
	const label = $derived(isPlanType(planType) ? planLabel(planType) : '');
	const listHref = $derived(planTemplatesHref(practiceId, planType));

	async function load() {
		if (!isPlanType(planType)) {
			throw new Error('There is no plan of that kind.');
		}
		const template = await loadTemplate(apiFetchWithSession, practiceId, planType);
		return template.fields;
	}

	async function send(fields: Field[], message: string) {
		await saveTemplate(apiFetchWithSession, practiceId, planType, fields);
		await gotoWithOutcome(listHref, message);
	}

	const question = new PracticeQuestion<Field>({
		name: 'plan-question',
		get questionId() {
			return page.params.questionId!;
		},
		get kind() {
			return page.url.searchParams.get('kind') ?? undefined;
		},
		load,
		loadFailure: 'Failed to load plan template',
		get notOnList() {
			return `This question is not on the ${label} any more.`;
		},
		toField: (draft) => ({ ...draft, order: 0 }),
		onSave: (fields) => send(fields, planTemplateSavedMessage(planType)),
		retire: {
			verb: 'Remove',
			get consequence() {
				return `Every ${label} filled in from now on will not ask it. A ${label} already filled in keeps it, with its answer.`;
			},
			apply: removeField,
			onDone: (fields, removed) => send(fields, planTemplateRemovedMessage(planType, removed))
		}
	});

	onMount(() => question.load());
</script>

{#snippet errorSummary()}
	<ErrorSummary errors={question.submission.errors} />
{/snippet}

{#if !question.isLoaded}
	<FormPage {...question.waiting} />
{:else}
	<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
	<form onsubmit={(event) => question.save(event)} novalidate>
		<QuestionPage
			{...question.frame}
			backHref={listHref}
			caption={label}
			errorSummary={question.submission.errors.length > 0 ? errorSummary : undefined}
		>
			{#snippet content()}
				<QuestionFields {question} />
			{/snippet}
			{#snippet actions()}
				<QuestionActions {question} />
			{/snippet}
		</QuestionPage>
	</form>
{/if}
