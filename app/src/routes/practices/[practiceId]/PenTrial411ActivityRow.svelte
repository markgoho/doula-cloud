<script lang="ts">
	interface Properties {
		/**
		 * Rendered exactly as the design shows it: `2026-08-28 09:41`.
		 */
		timestamp: string;
		description: string;
		actor: string;
	}

	let { timestamp, description, actor }: Properties = $props();

	// The design's display form is one space away from a valid datetime
	// attribute, so the machine-readable value costs nothing here.
	const machineTimestamp = $derived(timestamp.replace(' ', 'T'));
</script>

<li>
	<time datetime={machineTimestamp}>{timestamp}</time>
	<span class="column-rule" aria-hidden="true"></span>
	<span class="description">{description}</span>
	<span class="actor">{actor}</span>
</li>

<style>
	@layer components {
		li {
			display: flex;
			gap: var(--space-5, 1.25rem);
			align-items: center;
			block-size: 52px;
			border-block-end: var(--border-thin) solid var(--color-border);
			font-family: var(--font-family-base);
		}

		/* The design leaves the last row's rule off so the ledger does not
		   draw a line against the panel's own bottom edge. */
		li:last-child {
			border-block-end: 0;
		}

		time {
			flex: 0 0 136px;
			color: var(--color-muted);
			font-family: var(--font-family-mono, ui-monospace, monospace);
			font-size: 0.75rem;
			font-weight: var(--font-weight-normal);
		}

		.column-rule {
			flex: 0 0 var(--border-thin);
			align-self: stretch;
			background-color: var(--color-border);
		}

		.description {
			flex: 1 1 auto;
			color: var(--color-text);
			font-size: 0.9375rem;
			font-weight: var(--font-weight-normal);
		}

		.actor {
			flex: 0 0 auto;
			color: var(--color-muted);
			font-size: var(--text-sm);
			font-weight: var(--font-weight-normal);
		}
	}
</style>
