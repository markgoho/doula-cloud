<script lang="ts">
	import ReadFailure from '#lib/components/molecules/ReadFailure.svelte';

	const retry = () => new Promise<void>((resolve) => setTimeout(resolve, 600));
</script>

<stack-l space="var(--space-6)">
	<h1>Read failure</h1>

	<section>
		<h2>Default</h2>
		<ReadFailure message="Who is on call did not load." onRetry={retry} />
	</section>

	<section>
		<h2>Another list</h2>
		<ReadFailure message="Your Invoices did not load." onRetry={retry} />
	</section>
</stack-l>
