<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isNew?: boolean;
		isKindLocked?: boolean;
		isRefused?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Practice question form, a question not yet saved', props: { isNew: true } },
		{ name: 'Practice question form, a saved Client field', props: { isKindLocked: true } },
		{ name: 'Practice question form, refused', props: { isRefused: true } }
	];
</script>

<script lang="ts">
	import type { FieldType } from '#lib/planTemplate.js';
	import PracticeQuestionForm, {
		practiceQuestionIds
	} from '#lib/components/organisms/PracticeQuestionForm.svelte';

	let { isNew = false, isKindLocked = false, isRefused = false }: Properties = $props();

	const ids = practiceQuestionIds('demo');

	/*
	 * A choice question, so the answers box shows, with the longest
	 * realistic words (ADR-0025): the whole question and whole answers.
	 * The page's state is its props' first value; the picker on the drag
	 * surface mounts each variant afresh.
	 */
	let type = $state<FieldType>('single_select');
	// svelte-ignore state_referenced_locally
	let label = $state(isRefused ? '' : 'What pain relief do you want offered, and when should it be offered?');
	// svelte-ignore state_referenced_locally
	let optionsText = $state(isRefused ? '' : 'Nothing unless I ask for it\nEpidural, as early as it can be given');
	let position = $state(2);

	const refusals: Record<string, string> = {
		[ids.label]: 'Enter the question',
		[ids.options]: 'Enter the answers to choose from, one on each line'
	};
</script>

<PracticeQuestionForm
	name="demo"
	{type}
	{label}
	{optionsText}
	position={isNew ? undefined : position}
	positionCount={5}
	{isKindLocked}
	errorFor={isRefused ? (id) => refusals[id] : undefined}
	onLabelInput={(value) => (label = value)}
	onTypeChange={(value) => (type = value)}
	onOptionsInput={(value) => (optionsText = value)}
	onPositionChange={(value) => (position = value)}
/>
