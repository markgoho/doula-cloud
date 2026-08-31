<script lang="ts">
	import PlanInstanceForm from '#lib/components/organisms/PlanInstanceForm.svelte';
	import { setAnswer, toggleMultiSelectOption, type Answers } from '#lib/planInstance.js';
	import type { Field } from '#lib/planTemplate.js';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): the
	 * labels are a Practice's own questions, and the free-text answer is
	 * where a Client pastes a link -- the one value a browser will not
	 * break (#521).
	 */
	const fields: Field[] = [
		{
			id: 'labour',
			type: 'section_header',
			label: 'During labour and immediately after the birth',
			order: 1
		},
		{
			id: 'preference',
			type: 'short_text',
			label: 'What position do you want to be in for as long as you can?',
			order: 2
		},
		{
			id: 'notes',
			type: 'long_text',
			label: 'Anything else your Doula and the birth team should know',
			order: 3
		},
		{
			id: 'pain',
			type: 'single_select',
			label: 'What pain relief do you want offered, and when?',
			options: ['Epidural, as early as it can be given', 'Nothing unless I ask for it'],
			order: 4
		},
		{
			id: 'people',
			type: 'multi_select',
			label: 'Who do you want in the room with you?',
			options: ['My partner, for the whole labour', 'My Doula from Highland Midwifery'],
			order: 5
		},
		{ id: 'music', type: 'checkbox', label: 'Music playing throughout the labour', order: 6 }
	];

	let answers = $state<Answers>({
		preference: 'Standing, or leaning on the bed if I get tired',
		notes:
			'The playlist is at https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
	});
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
