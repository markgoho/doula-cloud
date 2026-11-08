<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isNew?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Question actions, a question not yet saved', props: { isNew: true } }
	];
</script>

<script lang="ts">
	import QuestionActions from '#lib/components/organisms/QuestionActions.svelte';
	import { PracticeQuestion } from '#lib/practiceQuestion.svelte.js';
	import type { Field } from '#lib/planTemplate.js';

	let { isNew = false }: Properties = $props();

	const question = new PracticeQuestion<Field>({
		name: 'demo',
		get questionId() {
			return isNew ? 'new' : 'pain';
		},
		kind: undefined,
		load: async () => [],
		loadFailure: 'Failed to load plan template',
		notOnList: 'This question is not on the Birth Plan any more.',
		toField: (draft) => ({ ...draft, order: 0 }),
		onSave: async () => {},
		retire: {
			verb: 'Remove',
			consequence: 'Every Birth Plan filled in from now on will not ask it.',
			apply: (list) => list,
			onDone: async () => {}
		}
	});
</script>

<QuestionActions {question} />
