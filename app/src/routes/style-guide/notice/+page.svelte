<script lang="ts">
	import Notice from '#lib/components/atoms/Notice.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Notice</h1>

	<section>
		<h2>Variants</h2>
		<stack-l space="var(--space-4)">
			<!--
				The longest realistic value, not a representative one (ADR-0025):
				these are the longest messages the app actually passes -- the
				Stripe screens explain a whole recovery in one Notice.
			-->
			<Notice
				variant="error"
				message="The page published for this Practice is not loading, so Stripe would find nothing at its web address. Publish the website again."
			/>
			<Notice variant="status" message="Stripe onboarding finished. Status updates once Stripe confirms the account is active." />
			<Notice
				variant="info"
				message="Stripe will not process Client payments until it can see this Practice online. The website question needs an answer before Stripe can be connected."
			/>
		</stack-l>
	</section>
</stack-l>
