<script lang="ts">
	import BirthPlanView from '#lib/components/molecules/BirthPlanView.svelte';
	import type { Field } from '#lib/planTemplate.js';

	const fields: Field[] = [
		{ id: 'labour', type: 'section_header', label: 'During labour', order: 1 },
		{ id: 'pain', type: 'single_select', label: 'Pain relief', options: ['Epidural', 'None'], order: 2 },
		{ id: 'people', type: 'multi_select', label: 'Who is in the room', options: ['Partner', 'Doula'], order: 3 },
		{ id: 'music', type: 'checkbox', label: 'Music playing', order: 4 },
		{ id: 'notes', type: 'long_text', label: 'Anything else', order: 5 }
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Birth plan view</h1>

	<section>
		<h2>Answered</h2>
		<BirthPlanView
			{fields}
			answers={{ pain: 'Epidural', people: ['Partner', 'Doula'], music: true, notes: 'Lights low.' }}
		/>
	</section>

	<section>
		<h2>Unanswered &mdash; every value falls back to an em dash</h2>
		<BirthPlanView {fields} answers={{}} />
	</section>
</stack-l>
