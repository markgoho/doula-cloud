<script lang="ts">
	import Notice from '#lib/components/atoms/Notice.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Notice</h1>

	<section>
		<h2>Variants</h2>
		<stack-l space="var(--space-4)">
			<Notice variant="error" message="Missing invite token" />
			<Notice variant="status" message="Stripe onboarding finished. Status updates once Stripe confirms your account is active." />
			<Notice variant="info" message="Your Stripe onboarding link expired. Start again below." />
		</stack-l>
	</section>
</stack-l>
