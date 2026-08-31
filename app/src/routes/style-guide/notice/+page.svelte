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
				message="The page we publish for you is not loading, so Stripe would find nothing at your web address. Open your website settings and publish it again."
			/>
			<Notice variant="status" message="Stripe onboarding finished. Status updates once Stripe confirms your account is active." />
			<Notice
				variant="info"
				message="Stripe will not let you take Client payments until it can see where you are online. Tell us your website or let us publish a page for you, then come back here."
			/>
		</stack-l>
	</section>
</stack-l>
