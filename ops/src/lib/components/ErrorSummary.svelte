<script lang="ts">
	import type { FormError } from '../authRefusal.ts';

	// GOV.UK's error summary: above the form, focused as it appears so a
	// screen reader reads it first, each error a link to its field.
	let { errors }: { errors: FormError[] } = $props();

	function focusOnMount(node: HTMLElement) {
		node.focus();
	}
</script>

{#if errors.length > 0}
	<div class="summary" role="alert" tabindex="-1" {@attach focusOnMount}>
		<h2>There is a problem</h2>
		<ul>
			{#each errors as error (error.message)}
				<li>
					{#if error.field}
						<a href={`#${error.field}`}>{error.message}</a>
					{:else}
						{error.message}
					{/if}
				</li>
			{/each}
		</ul>
	</div>
{/if}

<style>
	.summary {
		padding: var(--space-4);
		border: var(--border-active) solid var(--color-error);
		border-radius: var(--radius);
	}

	.summary:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}

	h2 {
		margin-block-end: var(--space-2);
	}

	ul {
		padding-inline-start: var(--space-4);
	}

	a {
		color: var(--color-error);
		font-weight: bold;
	}
</style>
