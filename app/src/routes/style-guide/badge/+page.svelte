<script lang="ts">
	import Badge from '#lib/components/atoms/Badge.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Badge</h1>

	<section>
		<h2>Variants</h2>
		<stack-l space="var(--space-4)">
			<Badge variant="info" label="Onboarding incomplete" />
			<Badge variant="success" label="Active" />
			<Badge variant="warning" label="Payment failed" />
			<Badge variant="error" label="Not connected" />
			<Badge variant="neutral" label="Draft" />
		</stack-l>
	</section>
</stack-l>
