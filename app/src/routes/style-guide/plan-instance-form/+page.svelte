<script lang="ts">
	import PlanInstanceForm from '#lib/components/organisms/PlanInstanceForm.svelte';
	import { setAnswer, toggleMultiSelectOption, type Answers } from '#lib/planInstance.js';
	import type { Field } from '#lib/planTemplate.js';

	const fields: Field[] = [
		{ id: 'labour', type: 'section_header', label: 'During labour', order: 1 },
		{ id: 'preference', type: 'short_text', label: 'Preferred position', order: 2 },
		{ id: 'notes', type: 'long_text', label: 'Anything else', order: 3 },
		{ id: 'pain', type: 'single_select', label: 'Pain relief', options: ['Epidural', 'None'], order: 4 },
		{ id: 'people', type: 'multi_select', label: 'Who is in the room', options: ['Partner', 'Doula'], order: 5 },
		{ id: 'music', type: 'checkbox', label: 'Music playing', order: 6 }
	];

	let answers = $state<Answers>({ preference: 'Standing' });
</script>

<stack-l space="var(--space-6)">
	<h1>Plan instance form</h1>

	<section>
		<h2>Every field type</h2>
		<PlanInstanceForm
			{fields}
			{answers}
			onAnswerChange={(fieldId, value) => (answers = setAnswer(answers, fieldId, value))}
			onToggleOption={(fieldId, option) => (answers = toggleMultiSelectOption(answers, fieldId, option))}
		/>
	</section>
</stack-l>
