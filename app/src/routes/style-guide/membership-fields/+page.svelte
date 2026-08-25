<script lang="ts">
	import MembershipFields from '#lib/components/molecules/MembershipFields.svelte';

	let roles = $state<string[]>(['doula']);
	let employmentType = $state<'employee' | 'contractor'>('employee');
</script>

<stack-l space="var(--space-6)">
	<h1>Membership fields</h1>

	<section>
		<h2>Default</h2>
		<p>
			The two halves of a Membership, edited together: what a person does at a Practice, and what
			she is to the business.
		</p>
		<MembershipFields
			{roles}
			{employmentType}
			onRolesChange={(next) => (roles = next)}
			onEmploymentTypeChange={(next) => (employmentType = next)}
		/>
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any control on this page to see its focus outline.</p>
	</section>
</stack-l>
