<script lang="ts">
	/*
	 * #2193: "Add a question" on the Client Fields asks the kind of answer
	 * first, the Plan Templates' own kind page with this list's addresses:
	 * one radio question in #1252's human names, nothing chosen when it
	 * opens (ADR-0055), and then the new question's own page.
	 */
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { FIELD_TYPE_OPTIONS, FIELD_TYPES, type FieldType } from '#lib/clientFieldTemplate.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import ErrorSummary, { type FormError } from '#lib/components/molecules/ErrorSummary.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { clientFieldsHref } from '../clientFieldPages.js';

	const NAME = 'question-kind';

	const practiceId = $derived(page.params.practiceId!);

	let kind = $state<FieldType | undefined>();
	let errors = $state<FormError[]>([]);

	async function next(event: SubmitEvent) {
		event.preventDefault();
		if (!kind) {
			errors = [{ message: 'Select the kind of answer you need', targetId: radioFieldId(NAME, FIELD_TYPES[0]) }];
			return;
		}
		const href = resolve('/practices/[practiceId]/settings/client-fields/[questionId]', {
			practiceId,
			questionId: 'new'
		});
		await goto(`${href}?kind=${kind}`);
	}
</script>

{#snippet errorSummary()}
	<ErrorSummary {errors} />
{/snippet}

<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the controls and the actions as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the inputs, not to arrange anything. -->
<form onsubmit={next} novalidate>
	<QuestionPage
		journey="Adding a question"
		steps={[]}
		backHref={clientFieldsHref(practiceId)}
		caption="Client Fields"
		question={{ as: 'legend', text: 'What kind of answer do you need?' }}
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
