<script lang="ts">
	import StaffNameFields from '#lib/components/molecules/StaffNameFields.svelte';

	/*
	 * The longest realistic values, not representative ones (ADR-0025): a
	 * first name of two words and a hyphenated double-barreled last name,
	 * because the fields stack at the width they are given and it is the
	 * value, not the label, that decides whether anything overflows at 320px.
	 */
	let firstName = $state('Anne-Marie Chiamaka');
	let lastName = $state('Ochieng-Whitfield-Adeyemi');
</script>

<stack-l space="var(--space-6)">
	<h1>Staff name fields</h1>

	<section>
		<h2>Default</h2>
		<stack-l space="var(--space-4)">
			<StaffNameFields bind:firstName bind:lastName />
		</stack-l>
	</section>

	<section>
		<h2>Each field refused on its own</h2>
		<stack-l space="var(--space-4)">
			<StaffNameFields
				firstName=""
				lastName=""
				firstError="Enter your first name"
				lastError="Enter your last name"
			/>
		</stack-l>
	</section>
</stack-l>
