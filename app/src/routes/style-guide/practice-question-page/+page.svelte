<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isNew?: boolean;
		isMissing?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Practice question page, a question not yet saved', props: { isNew: true } },
		{ name: 'Practice question page, a question the list no longer holds', props: { isMissing: true } }
	];
</script>

<script lang="ts">
	import PracticeQuestionPage from '#lib/components/templates/PracticeQuestionPage.svelte';
	import type { Field } from '#lib/planTemplate.js';

	let { isNew = false, isMissing = false }: Properties = $props();

	/*
	 * A choice question with the longest realistic words (ADR-0025). Neither
	 * save nor retire reaches an endpoint; the page stays where it is.
	 */
	const fields: Field[] = [
		{ id: 'first', type: 'short_text', label: 'Who do you want with you?', order: 0 },
		{
			id: 'pain',
			type: 'single_select',
			label: 'What pain relief do you want offered, and when should it be offered?',
			options: ['Nothing unless I ask for it', 'Epidural, as early as it can be given'],
			order: 1
		}
	];
</script>

<PracticeQuestionPage
	name="demo"
	caption="Birth Plan"
	listHref="/style-guide/practice-question-page"
	questionId={isNew ? 'new' : (isMissing ? 'gone' : 'pain')}
	kind="single_select"
	load={async () => fields}
	loadFailure="Failed to load plan template"
	notOnList="This question is not on the Birth Plan any more."
	toField={(draft) => ({ ...draft, order: 0 })}
	onSave={async () => {}}
	retire={{
		verb: 'Remove',
		consequence: 'Every Birth Plan filled in from now on will not ask it.',
		apply: (list, id) => list.filter((field) => field.id !== id),
		onDone: async () => {}
	}}
/>
