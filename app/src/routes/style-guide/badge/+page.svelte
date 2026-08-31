<script lang="ts">
	import Badge from '#lib/components/atoms/Badge.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Badge</h1>

	<section>
		<h2>Variants</h2>
		<stack-l space="var(--space-4)">
			<!--
				The longest realistic value, not a representative one (ADR-0025):
				a Badge label is a status, so the longest realistic one is the
				longest status the app actually sets -- these are the strings the
				Practice and Staff screens pass.
			-->
			<Badge variant="info" label="Stripe onboarding incomplete" />
			<Badge variant="success" label="Active" />
			<Badge variant="warning" label="Expired -- invite again or revoke" />
			<Badge variant="error" label="Email could not be delivered" />
			<Badge variant="neutral" label="Draft" />
		</stack-l>
	</section>
</stack-l>
