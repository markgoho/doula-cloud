<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isEmpty?: boolean;
		isReadOnly?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Practice question list, empty', props: { isEmpty: true } },
		{ name: 'Practice question list, for a reader who may not change it', props: { isReadOnly: true } }
	];
</script>

<script lang="ts">
	import PracticeQuestionList, {
		type ListedQuestion
	} from '#lib/components/organisms/PracticeQuestionList.svelte';

	let { isEmpty = false, isReadOnly = false }: Properties = $props();

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * Practice writes its own questions, so a row's label is the whole
	 * question. One row of each kind the list names differently: a plain
	 * answer, a choice, and a section heading.
	 */
	const questions: ListedQuestion[] = [
		{
			id: 'a',
			type: 'long_text',
			label: 'Who do you want in the room with you, and is there anyone you do not want there at all?'
		},
		{ id: 'b', type: 'single_select', label: 'What pain relief do you want offered, and when?' },
		{ id: 'c', type: 'section_header', label: 'After the birth' },
		{ id: 'd', type: 'short_text', label: '' }
	];
</script>

<PracticeQuestionList
	questions={isEmpty ? [] : questions}
	emptyMessage="No questions yet."
	changeHref={isReadOnly ? undefined : (question) => `/style-guide/practice-question-form#${question.id}`}
	addHref={isReadOnly ? undefined : '/style-guide/practice-question-form'}
/>
