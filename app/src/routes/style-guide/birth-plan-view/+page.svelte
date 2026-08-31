<script lang="ts">
	import BirthPlanView from '#lib/components/molecules/BirthPlanView.svelte';
	import type { Field } from '#lib/planTemplate.js';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * Practice writes its own plan template, so every label and option here
	 * is a whole question rather than a word, and the free-text answer is
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
			id: 'pain',
			type: 'single_select',
			label: 'What pain relief do you want offered, and when?',
			options: ['Epidural, as early as it can be given', 'Nothing unless I ask for it'],
			order: 2
		},
		{
			id: 'people',
			type: 'multi_select',
			label: 'Who do you want in the room with you?',
			options: ['My partner, for the whole labour', 'My Doula from Highland Midwifery'],
			order: 3
		},
		{ id: 'music', type: 'checkbox', label: 'Music playing throughout the labour', order: 4 },
		{
			id: 'notes',
			type: 'long_text',
			label: 'Anything else your Doula and the birth team should know',
			order: 5
		}
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Birth plan view</h1>

	<section>
		<h2>Answered</h2>
		<BirthPlanView
			{fields}
			answers={{
				pain: 'Epidural, as early as it can be given',
				people: ['My partner, for the whole labour', 'My Doula from Highland Midwifery'],
				music: true,
				notes:
					'Lights low, and the playlist is at https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
			}}
		/>
	</section>

	<section>
		<h2>Unanswered &mdash; every value falls back to an em dash</h2>
		<BirthPlanView {fields} answers={{}} />
	</section>
</stack-l>
