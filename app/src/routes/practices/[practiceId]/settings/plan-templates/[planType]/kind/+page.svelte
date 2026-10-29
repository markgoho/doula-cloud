<script lang="ts">
	/*
	 * #2193: "Add a question" asks the kind of answer first, as one radio
	 * question in #1252's human names, and then opens the new question's
	 * own page with that kind chosen. The six kinds wait here rather than
	 * as six buttons on the list (the brief's Hick tie-break: Tesler keeps
	 * all six, Hick keeps them off the screen until they are asked for).
	 * Nothing is chosen when it opens (ADR-0055).
	 */
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { FIELD_TYPE_OPTIONS, FIELD_TYPES, type FieldType } from '#lib/planTemplate.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import ErrorSummary, { type FormError } from '#lib/components/molecules/ErrorSummary.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { isPlanType, planLabel, planTemplatesHref } from '../../planTemplatePages.js';

	const NAME = 'question-kind';
	const QUESTION = 'What kind of answer do you need?';

	const practiceId = $derived(page.params.practiceId!);
	const planType = $derived(page.params.planType);

	let kind = $state<FieldType | undefined>();
	let errors = $state<FormError[]>([]);

	async function next(event: SubmitEvent) {
		event.preventDefault();
		if (!kind || !isPlanType(planType)) {
			errors = [{ message: 'Select the kind of answer you need', targetId: radioFieldId(NAME, FIELD_TYPES[0]) }];
			return;
		}
		const href = resolve('/practices/[practiceId]/settings/plan-templates/[planType]/[questionId]', {
			practiceId,
			planType,
			questionId: 'new'
		});
		await goto(`${href}?kind=${kind}`);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

{#snippet noActions()}{/snippet}

{#if isPlanType(planType)}
	<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
	<form onsubmit={next} novalidate>
		<QuestionPage
			journey="Adding a question"
			steps={[]}
			backHref={planTemplatesHref(practiceId, planType)}
			caption={planLabel(planType)}
			question={{ as: 'legend', text: QUESTION }}
			errorSummary={errors.length > 0 ? errorSummary : undefined}
		>
			{#snippet content()}
				<RadioGroup
					name={NAME}
					options={FIELD_TYPE_OPTIONS}
					value={kind}
					onChange={(value) => (kind = value)}
					error={errors[0]?.message}
				/>
			{/snippet}
			{#snippet actions()}
				<Button type="submit" label="Continue" />
			{/snippet}
		</QuestionPage>
	</form>
{:else}
	<FormPage title="Plan Templates" fieldsets={[]} actions={noActions} loadError="There is no plan of that kind." />
{/if}
