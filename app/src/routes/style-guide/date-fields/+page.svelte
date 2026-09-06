<script lang="ts">
	import DateFields from '#lib/components/molecules/DateFields.svelte';
	import { joinDate, splitDate, type DateParts } from '#lib/intakeDate.js';

	let asked = $state<DateParts>(splitDate('1988-02-09'));
	let grouped = $state<DateParts>({ month: '', day: '', year: '' });

	const composed = $derived(joinDate(asked));
</script>

<stack-l space="var(--space-6)">
	<h1>Date fields</h1>

	<section>
		<h2>On a question page</h2>
		<p>
			No legend: the Template owns the fieldset and its legend is the page heading, so a second
			one here would announce the question twice. What the three boxes compose to:
			<code>{composed.ok ? composed.value || '(nothing)' : composed.message}</code>
		</p>
		<DateFields name="style-guide-dob" parts={asked} onChange={(next) => (asked = next)} />
	</section>

	<section>
		<h2>With a group of its own</h2>
		<p>What a form asking a date alongside other questions gets.</p>
		<DateFields
			name="style-guide-grouped"
			legend="Date of birth"
			parts={grouped}
			onChange={(next) => (grouped = next)}
		/>
	</section>

	<section>
		<h2>Refused</h2>
		<p>The refusal is announced once, from the group, and only the box it is about is marked.</p>
		<DateFields
			name="style-guide-refused"
			parts={{ month: '2', day: '30', year: '1988' }}
			onChange={() => {}}
			error="Date of birth must be a real date"
			invalidField="day"
		/>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any box on this page to see its focus outline.</p>
	</section>
</stack-l>
