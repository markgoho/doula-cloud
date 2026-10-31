<script lang="ts">
	// app/'s stylesheet, whole, by relative import as site/ takes it: one
	// set of tokens for the product (ADR-0062).
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

	:global(.ops-button) {
		min-block-size: var(--control-height);
		padding: var(--space-2) var(--space-4);
		color: var(--color-on-primary);
		font-weight: bold;
		background-color: var(--color-primary);
		border: none;
		border-radius: var(--radius);
		cursor: pointer;
		align-self: start;
	}

	:global(.ops-button:hover) {
		background-color: var(--color-primary-hover);
	}

	:global(.ops-button:focus-visible) {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}

	:global(.ops-button:disabled) {
		cursor: progress;
	}
</style>
