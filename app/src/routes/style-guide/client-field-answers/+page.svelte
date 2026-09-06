<script lang="ts">
	import ClientFieldAnswers from '#lib/components/organisms/ClientFieldAnswers.svelte';
	import type { Field } from '#lib/clientFieldTemplate.js';
	import type { FieldValue } from '#lib/intakeDraft.svelte.js';

	const fields: Field[] = [
		{
			id: 'referral',
			type: 'short_text',
			label: 'Who told this Client about the Practice?',
			order: 0,
			archived: false
		},
		{
			id: 'hopes',
			type: 'long_text',
			label: 'In this Client’s own words, what would make this birth feel like a good one?',
			order: 1,
			archived: false
		},
		{
			id: 'birthplace',
			type: 'single_select',
			label: 'Planned place of birth',
			options: ['Home', 'Birth center', 'Strong Memorial Hospital'],
			order: 2,
			archived: false
		},
		{
			id: 'attendees',
			type: 'multi_select',
			label: 'Who else is expected to be in the room',
			options: ['Partner', 'Mother', 'Mother-in-law', 'Photographer'],
			order: 3,
			archived: false
		},
		{
			id: 'photos',
			type: 'checkbox',
			label: 'Consents to photographs being taken during labor and after the birth',
			order: 4,
			archived: false
		}
	];

	let values = $state<Record<string, FieldValue>>({
		birthplace: 'Home',
		attendees: ['Partner']
	});
</script>

<stack-l space="var(--space-6)">
	<h1>Client field answers</h1>

	<section>
		<h2>Every field type a Practice can define</h2>
		<p>
			The values a Practice’s own questions collect. Defining those questions is
			<code>DynamicFieldEditor</code>’s job, and this is the other half: it never renders a type
			picker and never reorders anything.
		</p>
		<ClientFieldAnswers
			{fields}
			{values}
			onChange={(fieldId, value) => (values = { ...values, [fieldId]: value })}
			idPrefix="style-guide-field"
		/>
	</section>

	<section>
		<h2>Nothing to ask</h2>
		<p>A section whose fields have all been archived renders nothing at all, not an empty box.</p>
		<ClientFieldAnswers fields={[]} values={{}} onChange={() => {}} idPrefix="style-guide-empty" />
	</section>
</stack-l>
