<script lang="ts">
	import '../../../app/src/lib/styles/app.css';

	let { children } = $props();
</script>

<header>
	<p class="service">DoulaCloud operations</p>
</header>
<main id="main">
	<center-l>
		<stack-l>
			{@render children()}
		</stack-l>
	</center-l>
</main>

<style>
	header {
		padding: var(--space-3) var(--space-4);
		color: var(--color-on-primary);
		background-color: var(--color-primary);
	}

	.service {
		font-weight: bold;
	}

	main {
		padding: var(--space-6) var(--space-4);
	}
</style>
