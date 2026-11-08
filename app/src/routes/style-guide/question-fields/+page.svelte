<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isLocked?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Question fields, a saved kind of answer that cannot change', props: { isLocked: true } }
	];
</script>

<script lang="ts">
	import { onMount } from 'svelte';
	import QuestionFields from '#lib/components/organisms/QuestionFields.svelte';
	import { PracticeQuestion } from '#lib/practiceQuestion.svelte.js';
	import type { Field } from '#lib/planTemplate.js';

	let { isLocked = false }: Properties = $props();

	/*
	 * A choice question with the longest realistic words (ADR-0025). Neither
	 * save nor retire reaches an endpoint.
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

	const question = new PracticeQuestion<Field>({
		name: 'demo',
		questionId: 'pain',
		kind: undefined,
		load: async () => fields,
		loadFailure: 'Failed to load plan template',
		notOnList: 'This question is not on the Birth Plan any more.',
		get locksKind() {
			return isLocked;
		},
		toField: (draft) => ({ ...draft, order: 0 }),
		onSave: async () => {},
		retire: {
			verb: 'Remove',
			consequence: 'Every Birth Plan filled in from now on will not ask it.',
			apply: (list, id) => list.filter((field) => field.id !== id),
			onDone: async () => {}
		}
	});

	onMount(() => question.load());
</script>

{#if question.isLoaded}
	<QuestionFields {question} />
{/if}
